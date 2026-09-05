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

// Package storetest holds small helpers shared by store.Session tests across packages.
package storetest

import (
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

// FindReservation looks up a reservation via ListReservations, so it can find one still
// pending its release cooldown, unlike Session.GetReservationByID which hides those.
// Returns nil if no matching reservation is found.
func FindReservation(s store.Session, poolKey, containerID, ifName string) *types.Reservation {
	for _, r := range s.ListReservations(poolKey) {
		if r.ContainerID == containerID && r.InterfaceName == ifName {
			return &r
		}
	}
	return nil
}
