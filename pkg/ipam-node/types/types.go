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
)

// ImplementedVersion contains implemented version of the store file layout
const ImplementedVersion = 1

// NewRoot returns initialized store Root
func NewRoot() *Root {
	return &Root{Version: ImplementedVersion, Pools: map[string]PoolReservations{}}
}

// Root is the root object of the store
type Root struct {
	Version  int                         `json:"version"`
	Checksum Checksum                    `json:"checksum"`
	Pools    map[string]PoolReservations `json:"pools"`
}

// DeepCopy is a deepcopy function for the Root struct
func (in *Root) DeepCopy() *Root {
	if in == nil {
		return nil
	}
	out := new(Root)
	*out = *in
	if in.Pools != nil {
		in, out := &in.Pools, &out.Pools
		*out = make(map[string]PoolReservations, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	return out
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

// DeepCopy is a deepcopy function for the PoolReservations struct
func (in *PoolReservations) DeepCopy() *PoolReservations {
	if in == nil {
		return nil
	}
	out := new(PoolReservations)
	*out = *in
	if in.Entries != nil {
		in, out := &in.Entries, &out.Entries
		*out = make(map[string]Reservation, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.LastReservedIP != nil {
		in, out := &in.LastReservedIP, &out.LastReservedIP
		*out = make(net.IP, len(*in))
		copy(*out, *in)
	}
	return out
}

// Reservation is used to store Reservation in a checkpoint file
type Reservation struct {
	ContainerID   string              `json:"container_id"`
	InterfaceName string              `json:"interface_name"`
	IPAddress     net.IP              `json:"ip_address"`
	Metadata      ReservationMetadata `json:"metadata"`
}

// DeepCopy is a deepcopy function for the Reservation struct
func (in *Reservation) DeepCopy() *Reservation {
	if in == nil {
		return nil
	}
	out := new(Reservation)
	*out = *in
	if in.IPAddress != nil {
		in, out := &in.IPAddress, &out.IPAddress
		*out = make(net.IP, len(*in))
		copy(*out, *in)
	}
	in.Metadata = *out.Metadata.DeepCopy()
	return out
}

// String returns string representation of the Reservation
func (in *Reservation) String() string {
	//nolint:errchkjson
	data, _ := json.Marshal(&in)
	return string(data)
}

// ReservationMetadata contains meta information for reservation
type ReservationMetadata struct {
	CreateTime         string `json:"create_time"`
	PodUUID            string `json:"pod_uuid"`
	PodName            string `json:"pod_name"`
	PodNamespace       string `json:"pod_namespace"`
	DeviceID           string `json:"device_id"`
	PoolConfigSnapshot string `json:"pool_config_snapshot"`
}

// DeepCopy is a deepcopy function for the ReservationMetadata struct
func (in *ReservationMetadata) DeepCopy() *ReservationMetadata {
	if in == nil {
		return nil
	}
	out := new(ReservationMetadata)
	*out = *in
	return out
}
