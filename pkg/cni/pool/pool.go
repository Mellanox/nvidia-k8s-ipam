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
	ipBlocksAnnotation = "ipam.nvidia.com/ip-blocks"
)

type IPPool struct {
	Name    string `json:"-"`
	Subnet  string `json:"subnet"`
	StartIP string `json:"startIP"`
	EndIP   string `json:"endIP"`
	Gateway string `json:"gateway"`
}

type Manager interface {
	// GetPoolByName returns IPPool for the provided pool name or nil if pool doesnt exist
	GetPoolByName(name string) *IPPool
	// GetPools returns map with information about all pools
	GetPools() map[string]*IPPool
}

type ManagerImpl struct {
	poolByName map[string]*IPPool
}

func NewManagerImpl(node *v1.Node) (*ManagerImpl, error) {
	if node == nil {
		return nil, fmt.Errorf("nil node provided")
	}

	blocks, ok := node.Annotations[ipBlocksAnnotation]
	if !ok {
		return nil, fmt.Errorf("%s node annotation not found", ipBlocksAnnotation)
	}

	poolByName := make(map[string]*IPPool)
	err := json.Unmarshal([]byte(blocks), &poolByName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s annotation content. %w", ipBlocksAnnotation, err)
	}

	for poolName, pool := range poolByName {
		pool.Name = poolName
	}

	return &ManagerImpl{
		poolByName: poolByName,
	}, nil
}

// GetPoolByName implements Manager interface
func (pm *ManagerImpl) GetPoolByName(name string) *IPPool {
	return pm.poolByName[name]
}

// GetPools implements Manager interface
func (pm *ManagerImpl) GetPools() map[string]*IPPool {
	return pm.poolByName
}

// SetPools serialize IP pools settings for the node and add this info as annotation
func SetPools(node *v1.Node, pools map[string]*IPPool) error {
	annotations := node.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	data, err := json.Marshal(pools)
	if err != nil {
		return fmt.Errorf("failed to serialize pools config: %v", err)
	}
	annotations[ipBlocksAnnotation] = string(data)
	node.SetAnnotations(annotations)
	return nil
}

// AnnotationExist returns true if ip-block annotation exist
func AnnotationExist(node *v1.Node) bool {
	_, exist := node.GetAnnotations()[ipBlocksAnnotation]
	return exist
}

// RemoveAnnotation removes annotation with ip-block from the node object
func RemoveAnnotation(node *v1.Node) {
	annotations := node.GetAnnotations()
	delete(annotations, ipBlocksAnnotation)
	node.SetAnnotations(annotations)
}
