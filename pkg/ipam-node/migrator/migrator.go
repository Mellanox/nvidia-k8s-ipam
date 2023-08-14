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

package migrator

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

const (
	// EnvDisableMigration contains the name of the environment variable which can be used
	// to disable migration
	EnvDisableMigration = "MIGRATOR_DISABLE_MIGRATION"
	// EnvHostLocalStorePath contains the name of the environment variable which can be used to
	// change host local IPAM store path
	EnvHostLocalStorePath = "MIGRATOR_HOST_LOCAL_STORE"
	// DefaultHostLocalStorePath contains default path for host-local store which was used
	// in the older version
	DefaultHostLocalStorePath = "/var/lib/cni/nv-ipam/state/host-local"
	// PlaceholderForUnknownField contains placeholder string which will be used as a value
	// for fields for which we have no data
	PlaceholderForUnknownField = "MIGRATED_NO_DATA"
)

// New create and initialize new instance of the migrator
func New(store storePkg.Store) Migrator {
	return &migrator{
		store: store,
	}
}

// Migrator is the interface implemented by migrator package
type Migrator interface {
	// Migrate execute migration logic.
	// The implementation will check if host-local IPAM data that were created by the previous version
	// of nv-ipam are available on the filesystem. If the data is found,
	// the migrator will read and save them into the store and then remove data in the old format.
	// Some information required by the new store schema can't be restored from the host-local IPAM store
	// format used in the older version of the nv-ipam. Missing data will be replaced by
	// a special placeholder which indicates that data is missing.
	Migrate(ctx context.Context) error
}

// default migrator implementation
type migrator struct {
	store storePkg.Store
}

// Migrate is the Migrator interface implementation for the migrator
func (m *migrator) Migrate(ctx context.Context) error {
	logger := logr.FromContextOrDiscard(ctx).WithName("migrator")
	if os.Getenv(EnvDisableMigration) != "" {
		logger.Info(fmt.Sprintf("%s set, skip migration", EnvDisableMigration))
		return nil
	}
	hostLocalPath := os.Getenv(EnvHostLocalStorePath)
	if hostLocalPath == "" {
		hostLocalPath = DefaultHostLocalStorePath
	}
	info, err := os.Stat(hostLocalPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("host-local IPAM path not found, skip migration", "path", hostLocalPath)
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("host-local IPAM path is not a dir")
	}
	session, err := m.store.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to open store: %v", err)
	}
	logger.Info("check host-local store path", "path", hostLocalPath)
	err = filepath.Walk(hostLocalPath, getWalkFunc(logger, session))
	if err != nil {
		session.Cancel()
		return err
	}
	err = session.Commit()
	if err != nil {
		return err
	}
	err = os.RemoveAll(hostLocalPath)
	if err != nil {
		logger.Error(err, "failed to remove data from host-local IPAM path")
		return err
	}
	logger.Info("migration complete. host-local IPAM data removed", "path", hostLocalPath)
	return nil
}

func getWalkFunc(logger logr.Logger, session storePkg.Session) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		poolName := filepath.Base(filepath.Dir(path))
		addr := ip.NormalizeIP(net.ParseIP(info.Name()))
		if addr == nil {
			return nil
		}
		allocationContent, err := os.ReadFile(path)
		if err != nil {
			logger.Error(err, "failed to read allocation data for IP",
				"pool", poolName, "ip", info.Name())
			return err
		}
		allocData := strings.Split(string(allocationContent), "\n")
		if len(allocData) != 2 {
			logger.Error(nil, "unexpected allocation data format",
				"pool", poolName, "ip", info.Name(), "data", allocData)
			return fmt.Errorf("unexpected allocation format")
		}
		containerID, interfaceName := strings.Trim(allocData[0], "\r"), allocData[1]
		if err := session.Reserve(poolName, containerID, interfaceName, types.ReservationMetadata{
			CreateTime:         time.Now().Format(time.RFC3339Nano),
			PodUUID:            PlaceholderForUnknownField,
			PodName:            PlaceholderForUnknownField,
			PodNamespace:       PlaceholderForUnknownField,
			DeviceID:           PlaceholderForUnknownField,
			PoolConfigSnapshot: PlaceholderForUnknownField,
		}, addr); err != nil {
			logger.V(1).Info("failed to reserve IP, ignore allocation",
				"pool", poolName, "ip", info.Name(), "reason", err.Error())
			// ignore reservation error and skip the reservation
			return nil
		}
		logger.V(1).Info("IP reservation migrated", "pool", poolName, "ip", info.Name())
		return nil
	}
}
