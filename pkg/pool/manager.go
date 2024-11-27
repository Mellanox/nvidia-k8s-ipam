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
type Manager interface {
	ConfigReader
	// Update Pool's config from IPPool CR
	UpdatePool(key string, pool *Pool)
	// Remove Pool's config by key
	RemovePool(key string)
}

// NewManager create and initialize new manager instance
func NewManager() Manager {
	return &manager{
		poolByKey: make(map[string]*Pool),
	}
}

type manager struct {
	lock      sync.Mutex
	poolByKey map[string]*Pool
}

func (m *manager) UpdatePool(key string, pool *Pool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.poolByKey[key] = pool
}

func (m *manager) RemovePool(key string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.poolByKey, key)
}

// GetPoolByKey is the Manager interface implementation for the manager
func (m *manager) GetPoolByKey(key string) *Pool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.poolByKey[key]
}

// GetPools is the Manager interface implementation for the manager
func (m *manager) GetPools() map[string]*Pool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.poolByKey
}
