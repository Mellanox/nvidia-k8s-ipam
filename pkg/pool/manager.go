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

import (
	"encoding/json"
	"sync"
)

// Pool represents generic pool configuration
type Pool struct {
	Name           string           `json:"-"`
	Subnet         string           `json:"subnet"`
	StartIP        string           `json:"startIP"`
	EndIP          string           `json:"endIP"`
	Gateway        string           `json:"gateway"`
	Exclusions     []ExclusionRange `json:"exclusions"`
	Routes         []Route          `json:"routes"`
	DefaultGateway bool             `json:"defaultGateway"`
}

// ExclusionRange contains range of IP to exclude from the allocation
type ExclusionRange struct {
	StartIP string `json:"startIP"`
	EndIP   string `json:"endIP"`
}

// Route contains a destination CIDR to be added as static route via gateway
type Route struct {
	Dst string `json:"dst"`
}

// String return string representation of the IPPool config
func (p *Pool) String() string {
	//nolint:errchkjson
	data, _ := json.Marshal(p)
	return string(data)
}

// ConfigReader is an interface to which provides access to the pool configuration
type ConfigReader interface {
	// GetPoolByKey returns IPPool for the provided pool name or nil if pool doesn't exist
	GetPoolByKey(key string) *Pool
	// GetPools returns map with information about all pools
	GetPools() map[string]*Pool
}

// Manager provide access to pools configuration
type Manager interface {
	ConfigReader
	// Update Pool's config from provided Pool object
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
