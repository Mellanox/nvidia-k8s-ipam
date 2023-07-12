/*
 Copyright 2023, NVIDIA CORPORATION & AFFILIATES
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
     http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/renameio/v2"
	"k8s.io/klog/v2"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/types"
)

var (
	// ErrReservationAlreadyExist is returned when reservation for id and interface already exist in the pool
	ErrReservationAlreadyExist = errors.New("reservation already exist, " +
		"duplicate allocations are not allowed by the spec")
	// ErrIPAlreadyReserved is returned when trying to reserve IP which is already reserved
	ErrIPAlreadyReserved = errors.New("ip address is already reserved")
)

// Manager implements API to open file store in exclusive mode
type Manager interface {
	// Open returns open instance of the store with exclusive lock.
	// returns an error if failed to read data from the disk.
	// Open blocks if Store is already opened.
	// Store.Close() should be called to write changes to disk and release the lock.
	Open(ctx context.Context) (Store, error)
}

// Store is the interface implemented by the store package
type Store interface {
	// Reserve reserves IP for the id and interface name,
	// returns error if allocation failed
	Reserve(pool string, id string, ifName string, meta types.ReservationMetadata, address net.IP) error
	// ListReservations list all reservations in the pool
	ListReservations(pool string) []types.Reservation
	// GetLastReservedIP returns last reserved IP for the pool or nil
	GetLastReservedIP(pool string) net.IP
	// SetLastReservedIP set last reserved IP fot the pool
	SetLastReservedIP(pool string, ip net.IP)
	// ReleaseReservationByID releases reservation by id and interface name
	ReleaseReservationByID(pool string, id string, ifName string)
	// GetReservationByID returns existing reservation for id and interface name,
	// return nil if allocation not found
	GetReservationByID(pool string, id string, ifName string) *types.Reservation
	// Commit writes data to the disk and release the lock
	Commit() error
	// Cancel simply release the lock
	Cancel()
}

// NewManager create and initialize new store manager
func NewManager(storeFile string) Manager {
	return &manager{
		storeFile: storeFile,
		lock:      &sync.Mutex{},
	}
}

type manager struct {
	storeFile string
	lock      *sync.Mutex
}

func (m *manager) Open(ctx context.Context) (Store, error) {
	m.lock.Lock()
	return newStore(m.storeFile, m.lock, klog.FromContext(ctx))
}

func newStore(storeFile string, lock *sync.Mutex, logger logr.Logger) (Store, error) {
	s := &store{
		storeFile: storeFile,
		lock:      lock,
		log:       logger,
	}
	if err := s.load(); err != nil {
		lock.Unlock()
		return nil, err
	}
	return s, nil
}

// default store implementation
type store struct {
	log        logr.Logger
	lock       *sync.Mutex
	storeFile  string
	data       *types.Root
	isModified bool
	isCanceled bool
}

// Commit is the Store interface implementation for store
func (s *store) Commit() error {
	if s.isCanceled {
		return fmt.Errorf("can't commit to canceled store")
	}
	defer s.lock.Unlock()
	return s.save()
}

func (s *store) Cancel() {
	s.isCanceled = true
	s.lock.Unlock()
}

// Reserve is the Store interface implementation for store
func (s *store) Reserve(pool string, id string, ifName string, meta types.ReservationMetadata, address net.IP) error {
	reservationKey := s.getKey(id, ifName)
	poolData := s.getPoolData(pool, s.data)
	_, exist := poolData.Entries[reservationKey]
	if exist {
		return ErrReservationAlreadyExist
	}
	duplicateIP := false
	for _, r := range poolData.Entries {
		if address.Equal(r.IPAddress) {
			duplicateIP = true
			break
		}
	}
	if duplicateIP {
		// duplicate allocator should retry
		return ErrIPAlreadyReserved
	}
	poolData.LastReservedIP = address
	poolData.LastPoolConfig = meta.PoolConfigSnapshot
	reservation := types.Reservation{
		ContainerID:   id,
		InterfaceName: ifName,
		IPAddress:     address,
		Metadata:      meta,
	}
	reservation.Metadata.CreateTime = time.Now()
	poolData.Entries[reservationKey] = reservation
	s.data.Pools[pool] = *poolData
	s.isModified = true
	return nil
}

func (s *store) getPoolData(pool string, layout *types.Root) *types.PoolReservations {
	res, exist := layout.Pools[pool]
	if exist {
		if res.Entries == nil {
			res.Entries = map[string]types.Reservation{}
		}
		return &res
	}
	return types.NewPoolReservations(pool)
}

// ListReservations is the Store interface implementation for store
func (s *store) ListReservations(pool string) []types.Reservation {
	poolData := s.getPoolData(pool, s.data)
	allocations := make([]types.Reservation, 0, len(poolData.Entries))
	for _, a := range poolData.Entries {
		allocations = append(allocations, a)
	}
	return allocations
}

// GetLastReservedIP is the Store interface implementation for store
func (s *store) GetLastReservedIP(pool string) net.IP {
	poolData := s.getPoolData(pool, s.data)
	return poolData.LastReservedIP
}

// SetLastReservedIP is the Store interface implementation for store
func (s *store) SetLastReservedIP(pool string, ip net.IP) {
	poolData := s.getPoolData(pool, s.data)
	poolData.LastReservedIP = ip
	s.data.Pools[pool] = *poolData
	s.isModified = true
}

// ReleaseReservationByID is the Store interface implementation for store
func (s *store) ReleaseReservationByID(pool string, id string, ifName string) {
	poolData := s.getPoolData(pool, s.data)
	delete(poolData.Entries, s.getKey(id, ifName))
	s.data.Pools[pool] = *poolData
	s.isModified = true
}

// GetReservationByID is the Store interface implementation for store
func (s *store) GetReservationByID(pool string, id string, ifName string) *types.Reservation {
	poolData := s.getPoolData(pool, s.data)
	reservation, exist := poolData.Entries[s.getKey(id, ifName)]
	if !exist {
		return nil
	}
	return &reservation
}

func (s *store) load() error {
	layout := &types.Root{}
	data, err := os.ReadFile(s.storeFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to read store data: %v", err)
	}
	if len(data) != 0 {
		if err := json.Unmarshal(data, layout); err != nil {
			return fmt.Errorf("failed to unmarshal store data: %v", err)
		}
	} else {
		layout = types.NewRoot()
	}
	if layout.Pools == nil {
		layout.Pools = map[string]types.PoolReservations{}
	}
	s.data = layout
	return nil
}

func (s *store) save() error {
	if !s.isModified {
		return nil
	}
	data, err := json.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("failed to marshal store data: %v", err)
	}
	if err := renameio.WriteFile(s.storeFile, data, 0664); err != nil {
		return fmt.Errorf("failed to write store data: %v", err)
	}
	return nil
}

func (s *store) getKey(id, ifName string) string {
	return id + "_" + ifName
}
