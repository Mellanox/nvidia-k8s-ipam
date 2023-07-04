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
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

// Manager provide access to pools configuration
//
//go:generate mockery --name Manager
type Manager interface {
	ConfigReader
	// Update Pool's configs from node object,
	// returns an error if node object doesn't contain valid config
	Update(node *corev1.Node) error
	// Reset clean Pool config which is cached in memory
	Reset()
}

// NewManager create and initialize new manager instance
func NewManager() Manager {
	return &manager{}
}

type manager struct {
	lock   sync.Mutex
	reader ConfigReader
}

// GetPoolByName is the Manager interface implementation for the manager
func (m *manager) GetPoolByName(name string) *IPPool {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.reader == nil {
		return nil
	}
	return m.reader.GetPoolByName(name)
}

// GetPools is the Manager interface implementation for the manager
func (m *manager) GetPools() map[string]*IPPool {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.reader == nil {
		return nil
	}
	return m.reader.GetPools()
}

// Update is the Manager interface implementation for the manager
func (m *manager) Update(node *corev1.Node) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	r, err := NewConfigReader(node)
	if err != nil {
		return fmt.Errorf("failed to update pools configuration from the node object: %v", err)
	}
	m.reader = r
	return nil
}

// Reset is the Manager interface implementation for the manager
func (m *manager) Reset() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.reader = nil
}
