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

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

var (
	// ErrReservationAlreadyExist is returned when reservation for id and interface already exist in the pool
	ErrReservationAlreadyExist = errors.New("reservation already exist, " +
		"duplicate allocations are not allowed by the spec")
	// ErrIPAlreadyReserved is returned when trying to reserve IP which is already reserved
	ErrIPAlreadyReserved = errors.New("ip address is already reserved")
)

// Store implements API to open file store in exclusive mode
type Store interface {
	// Open returns a session for the store with exclusive lock.
	// returns an error if failed to read persistedData from the disk.
	// Open blocks if another Session is already opened.
	// Session.Commit() should be called to close the session, release the lock and write changes to disk and
	// Session.Cancel() can be called to close the session, revert all changes and release the lock
	Open(ctx context.Context) (Session, error)
}

// Session is the interface implemented by the store package
type Session interface {
	// Reserve reserves IP for the id and interface name,
	// returns error if allocation failed
	Reserve(poolKey string, id string, ifName string, meta types.ReservationMetadata, address net.IP) error
	// ListReservations lists all reservations in the pool, including ones already
	// released by their owner and pending removal once their cooldown elapses (see
	// ReleaseReservationByID). Unlike GetReservationByID, it does not filter those out.
	ListReservations(poolKey string) []types.Reservation
	// ListPools return list with names of all known pools
	ListPools() []string
	// RemovePool removes information about the pool from the store
	RemovePool(poolKey string)
	// GetLastReservedIP returns last reserved IP for the pool or nil
	GetLastReservedIP(poolKey string) net.IP
	// SetLastReservedIP set last reserved IP fot the pool
	SetLastReservedIP(poolKey string, ip net.IP)
	// ReleaseReservationByID releases the reservation identified by id and interface name.
	// Returns true if the reservation was actually removed from the store.
	//
	// If cooldown <= 0, the reservation is removed immediately, regardless of any prior
	// release state.
	//
	// If cooldown > 0:
	//   - a reservation not yet marked released is marked released now (kept in the
	//     store, still blocking its IP from reuse) instead of being removed. The
	//     release timestamp is set once and is not reset by later calls, since CNI DEL
	//     is idempotent and may be retried for the same id/ifName.
	//   - a reservation already marked released is removed once its cooldown has
	//     elapsed; otherwise this call is a no-op.
	//
	// This is a no-op (returns false) if the reservation does not exist.
	ReleaseReservationByID(poolKey string, id string, ifName string, cooldown time.Duration) bool
	// GetReservationByID returns the existing, still-active reservation for id and
	// interface name. Returns nil if not found, including for a reservation that has
	// already been released by its owner and is only pending removal (see
	// ReleaseReservationByID).
	GetReservationByID(poolKey string, id string, ifName string) *types.Reservation
	// Commit writes persistedData to the disk and release the lock.
	// the store can't be used after this call
	Commit() error
	// Cancel cancels all modifications and release the lock
	// the store can't be used after this call
	Cancel()
}

// New create and initialize new store instance
func New(storeFile string) Store {
	return &store{
		storeFile: storeFile,
		lock:      &sync.Mutex{},
	}
}

type store struct {
	storeFile     string
	persistedData *types.Root
	lock          *sync.Mutex
}

// Open is the Store interface implementation for store
func (m *store) Open(ctx context.Context) (Session, error) {
	m.lock.Lock()
	logger := logr.FromContextOrDiscard(ctx)
	if m.persistedData == nil {
		diskData, err := m.loadFromDisk()
		if err != nil {
			m.lock.Unlock()
			logger.Error(err, "failed to load store data from the disk")
			return nil, err
		}
		m.persistedData = diskData
	}
	return newStore(m.storeFile, m.persistedData, m.lock, logger), nil
}

func (m *store) loadFromDisk() (*types.Root, error) {
	dst := &types.Root{}
	data, err := os.ReadFile(m.storeFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to read store persistedData: %v", err)
	}
	if len(data) != 0 {
		if err := json.Unmarshal(data, dst); err != nil {
			return nil, fmt.Errorf("failed to unmarshal store persistedData: %v", err)
		}
		if dst.Checksum == 0 {
			return nil, fmt.Errorf("checksum not set in store file")
		}
		if err := dst.Checksum.Verify(dst); err != nil {
			return nil, fmt.Errorf("store file corrupted, checksum mismatch")
		}
	} else {
		dst = types.NewRoot()
	}
	if dst.Pools == nil {
		dst.Pools = map[string]types.PoolReservations{}
	}
	return dst, nil
}

func newStore(storeFile string, persistentData *types.Root, lock *sync.Mutex, logger logr.Logger) *session {
	s := &session{
		log:           logger,
		lock:          lock,
		storeFile:     storeFile,
		persistedData: persistentData,
		tmpData:       persistentData.DeepCopy(),
	}
	return s
}

// default session implementation
type session struct {
	log       logr.Logger
	lock      *sync.Mutex
	storeFile string
	// store holds this data instance
	persistedData *types.Root
	// temp data, this data is local for session instance
	tmpData    *types.Root
	isModified bool
	isClosed   bool
}

// Commit is the Session interface implementation for session
func (s *session) Commit() error {
	s.checkClosed()
	defer s.markClosed()
	defer s.lock.Unlock()
	if !s.isModified {
		return nil
	}
	if err := s.writeToDisk(s.tmpData); err != nil {
		s.log.Error(err, "failed to write data to disk")
		return err
	}
	*s.persistedData = *s.tmpData
	return nil
}

// Cancel is the Session interface implementation for session
func (s *session) Cancel() {
	s.checkClosed()
	defer s.markClosed()
	defer s.lock.Unlock()
}

func (s *session) markClosed() {
	s.isClosed = true
}

func (s *session) checkClosed() {
	if s.isClosed {
		panic("operation was performed on the closed store")
	}
}

// Reserve is the Session interface implementation for session
func (s *session) Reserve(poolKey string, id string, ifName string,
	meta types.ReservationMetadata, address net.IP) error {
	s.checkClosed()
	reservationKey := s.getKey(id, ifName)
	poolData := s.getPoolData(poolKey, s.tmpData)
	// A reservation pending its release cooldown (IsReleased() == true) still occupies
	// this key and blocks it, exactly like an active one: the cooldown is a floor on
	// reuse of the address, and that applies to its own former owner too, not just other
	// callers.
	if _, exist := poolData.Entries[reservationKey]; exist {
		return ErrReservationAlreadyExist
	}
	for _, r := range poolData.Entries {
		if address.Equal(r.IPAddress) {
			// duplicate allocator should retry
			return ErrIPAlreadyReserved
		}
	}
	poolData.LastReservedIP = address
	poolData.LastPoolConfig = meta.PoolConfigSnapshot
	reservation := types.Reservation{
		ContainerID:   id,
		InterfaceName: ifName,
		IPAddress:     address,
		Metadata:      meta,
	}
	reservation.Metadata.CreateTime = time.Now().Format(time.RFC3339Nano)
	poolData.Entries[reservationKey] = reservation
	s.updatePoolData(poolKey, poolData)
	return nil
}

// updatePoolData stores poolData under poolKey in the session and marks it modified.
func (s *session) updatePoolData(poolKey string, poolData *types.PoolReservations) {
	s.tmpData.Pools[poolKey] = *poolData
	s.isModified = true
}

func (s *session) getPoolData(poolKey string, layout *types.Root) *types.PoolReservations {
	res, exist := layout.Pools[poolKey]
	if exist {
		if res.Entries == nil {
			res.Entries = map[string]types.Reservation{}
		}
		return &res
	}
	return types.NewPoolReservations(poolKey)
}

// ListReservations is the Session interface implementation for session
func (s *session) ListReservations(poolKey string) []types.Reservation {
	s.checkClosed()
	poolData := s.getPoolData(poolKey, s.tmpData)
	allocations := make([]types.Reservation, 0, len(poolData.Entries))
	for _, a := range poolData.Entries {
		allocations = append(allocations, a)
	}
	return allocations
}

// ListPools is the Session interface implementation for session
func (s *session) ListPools() []string {
	s.checkClosed()
	pools := make([]string, 0, len(s.tmpData.Pools))
	for poolName := range s.tmpData.Pools {
		pools = append(pools, poolName)
	}
	return pools
}

// RemovePool is the Session interface implementation for session
func (s *session) RemovePool(poolKey string) {
	s.checkClosed()
	delete(s.tmpData.Pools, poolKey)
	s.isModified = true
}

// GetLastReservedIP is the Session interface implementation for session
func (s *session) GetLastReservedIP(poolKey string) net.IP {
	s.checkClosed()
	poolData := s.getPoolData(poolKey, s.tmpData)
	return poolData.LastReservedIP
}

// SetLastReservedIP is the Session interface implementation for session
func (s *session) SetLastReservedIP(poolKey string, ip net.IP) {
	s.checkClosed()
	poolData := s.getPoolData(poolKey, s.tmpData)
	poolData.LastReservedIP = ip
	s.updatePoolData(poolKey, poolData)
}

// ReleaseReservationByID is the Session interface implementation for session
func (s *session) ReleaseReservationByID(poolKey string, id string, ifName string, cooldown time.Duration) bool {
	s.checkClosed()
	key := s.getKey(id, ifName)
	poolData := s.getPoolData(poolKey, s.tmpData)
	reservation, exist := poolData.Entries[key]
	if !exist {
		return false
	}
	expired := reservation.IsReleased() && time.Since(reservation.ReleasedAt) >= cooldown
	if cooldown > 0 && reservation.IsReleased() && !expired {
		return false
	}

	remove := cooldown <= 0 || expired
	if remove {
		delete(poolData.Entries, key)
	} else {
		reservation.ReleasedAt = time.Now()
		poolData.Entries[key] = reservation
	}
	s.updatePoolData(poolKey, poolData)
	return remove
}

// GetReservationByID is the Session interface implementation for session
func (s *session) GetReservationByID(poolKey string, id string, ifName string) *types.Reservation {
	s.checkClosed()
	poolData := s.getPoolData(poolKey, s.tmpData)
	reservation, exist := poolData.Entries[s.getKey(id, ifName)]
	if !exist || reservation.IsReleased() {
		return nil
	}
	return &reservation
}

func (s *session) writeToDisk(src *types.Root) error {
	src.Checksum = types.NewChecksum(src)
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal store persistedData: %v", err)
	}
	if err := renameio.WriteFile(s.storeFile, data, 0664); err != nil {
		return fmt.Errorf("failed to write store persistedData: %v", err)
	}
	return nil
}

func (s *session) getKey(id, ifName string) string {
	return id + "_" + ifName
}
