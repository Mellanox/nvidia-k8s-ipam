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

package types

import (
	"encoding/json"
	"net"
	"time"
)

// ImplementedVersion contains implemented version of the store file layout
const ImplementedVersion = 1

// NewRoot returns initialized store Root
func NewRoot() *Root {
	return &Root{Version: ImplementedVersion, Pools: map[string]PoolReservations{}}
}

// Root is the root object of the store
type Root struct {
	Version int                         `json:"version"`
	Pools   map[string]PoolReservations `json:"pools"`
}

// NewPoolReservations returns initialized PoolReservations object
func NewPoolReservations(name string) *PoolReservations {
	return &PoolReservations{
		Name:    name,
		Entries: map[string]Reservation{},
	}
}

// PoolReservations is object which used to store IP reservations for the IP pool
type PoolReservations struct {
	Name string `json:"name"`
	// key is container_id + _ + interface_name
	Entries map[string]Reservation `json:"entries"`
	// contains pool config from the latest store update
	LastPoolConfig string `json:"last_pool_config"`
	LastReservedIP net.IP `json:"last_reserved_ip"`
}

// Reservation is used to store Reservation in a checkpoint file
type Reservation struct {
	ContainerID   string              `json:"container_id"`
	InterfaceName string              `json:"interface_name"`
	IPAddress     net.IP              `json:"ip_address"`
	Metadata      ReservationMetadata `json:"metadata"`
}

// String returns string representation of the Reservation
func (r Reservation) String() string {
	//nolint:errchkjson
	data, _ := json.Marshal(&r)
	return string(data)
}

// ReservationMetadata contains meta information for reservation
type ReservationMetadata struct {
	CreateTime         time.Time `json:"create_time"`
	PodUUID            string    `json:"pod_uuid"`
	PodName            string    `json:"pod_name"`
	PodNamespace       string    `json:"pod_namespace"`
	DeviceID           string    `json:"device_id"`
	PoolConfigSnapshot string    `json:"pool_config_snapshot"`
}
