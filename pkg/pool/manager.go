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

package pool

import "sync"

// Manager provide access to pools configuration
//
//go:generate mockery --name Manager
type Manager interface {
	ConfigReader
	// Update Pool's config from IPPool CR
	UpdatePool(pool *IPPool)
	// Remove Pool's config
	RemovePool(poolName string)
}

// NewManager create and initialize new manager instance
func NewManager() Manager {
	return &manager{
		poolByName: make(map[string]*IPPool),
	}
}

type manager struct {
	lock       sync.Mutex
	poolByName map[string]*IPPool
}

func (m *manager) UpdatePool(pool *IPPool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.poolByName[pool.Name] = pool
}

func (m *manager) RemovePool(poolName string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.poolByName, poolName)
}

// GetPoolByName is the Manager interface implementation for the manager
func (m *manager) GetPoolByName(name string) *IPPool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.poolByName[name]
}

// GetPools is the Manager interface implementation for the manager
func (m *manager) GetPools() map[string]*IPPool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.poolByName
}
