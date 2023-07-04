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

package handlers

import (
	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	poolPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

type GetAllocatorFunc = func(s *allocator.RangeSet, poolName string, session storePkg.Session) allocator.IPAllocator

// New create and initialize new instance of grpc Handlers
func New(poolConfReader poolPkg.ConfigReader, store storePkg.Store, getAllocFunc GetAllocatorFunc) *Handlers {
	return &Handlers{
		poolConfReader: poolConfReader,
		store:          store,
		getAllocFunc:   getAllocFunc,
	}
}

// Handlers contains implementation of the GRPC endpoints handlers for ipam-daemon
type Handlers struct {
	poolConfReader poolPkg.ConfigReader
	store          storePkg.Store
	getAllocFunc   GetAllocatorFunc
	nodev1.UnsafeIPAMServiceServer
}
