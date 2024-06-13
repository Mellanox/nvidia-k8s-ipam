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
	"fmt"

	v1 "k8s.io/api/core/v1"
)

const (
	IPBlocksAnnotation = "ipam.nvidia.com/ip-blocks"
)

// Pool represents generic pool configuration
type Pool struct {
	Name       string           `json:"-"`
	Subnet     string           `json:"subnet"`
	StartIP    string           `json:"startIP"`
	EndIP      string           `json:"endIP"`
	Gateway    string           `json:"gateway"`
	Exclusions []ExclusionRange `json:"exclusions"`
}

// ExclusionRange contains range of IP to exclude from the allocation
type ExclusionRange struct {
	StartIP string `json:"startIP"`
	EndIP   string `json:"endIP"`
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

type configReader struct {
	poolByKey map[string]*Pool
}

func NewConfigReader(node *v1.Node) (ConfigReader, error) {
	if node == nil {
		return nil, fmt.Errorf("nil node provided")
	}

	blocks, ok := node.Annotations[IPBlocksAnnotation]
	if !ok {
		return nil, fmt.Errorf("%s node annotation not found", IPBlocksAnnotation)
	}

	poolByKey := make(map[string]*Pool)
	err := json.Unmarshal([]byte(blocks), &poolByKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s annotation content. %w", IPBlocksAnnotation, err)
	}

	for poolName, pool := range poolByKey {
		pool.Name = poolName
	}

	return &configReader{
		poolByKey: poolByKey,
	}, nil
}

// GetPoolByKey implements ConfigReader interface
func (r *configReader) GetPoolByKey(key string) *Pool {
	return r.poolByKey[key]
}

// GetPools implements ConfigReader interface
func (r *configReader) GetPools() map[string]*Pool {
	return r.poolByKey
}
