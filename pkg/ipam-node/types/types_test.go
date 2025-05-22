// Copyright 2025 nvidia-k8s-ipam authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package types_test

import (
	"net"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

var _ = Describe("Types", func() {
	It("Deepcopy", func() {
		orig := &types.Root{
			Version:  1,
			Checksum: 12345,
			Pools: map[string]types.PoolReservations{"foo": {
				Name: "foo",
				Entries: map[string]types.Reservation{"id1_net0": {
					ContainerID:   "id1",
					InterfaceName: "net0",
					IPAddress:     net.ParseIP("192.168.1.100"),
					Metadata: types.ReservationMetadata{
						CreateTime:         time.Now().Format(time.RFC3339Nano),
						PodUUID:            "testPodUUID",
						PodName:            "testPodName",
						PodNamespace:       "testNamespace",
						DeviceID:           "testDeviceID",
						PoolConfigSnapshot: "testPoolConfigSnapshot",
					},
				}},
				LastPoolConfig: "testLastPoolConfig",
				LastReservedIP: net.ParseIP("192.168.1.100"),
			}},
		}
		clone := orig.DeepCopy()
		Expect(reflect.DeepEqual(orig, clone)).To(BeTrue())

		orig.Checksum = 54321
		res := orig.Pools["foo"]
		res.LastPoolConfig = "changed"
		res.LastReservedIP = net.ParseIP("192.168.1.200")
		entry := res.Entries["id1_net0"]
		entry.IPAddress = net.ParseIP("192.168.1.200")
		res.Entries["id1_net0"] = entry
		orig.Pools["foo"] = res

		Expect(reflect.DeepEqual(orig, clone)).To(BeFalse())
		Expect(orig.Checksum).NotTo(Equal(clone.Checksum))
		Expect(orig.Pools["foo"].LastPoolConfig).NotTo(Equal(clone.Pools["foo"].LastPoolConfig))
		Expect(orig.Pools["foo"].LastReservedIP).NotTo(Equal(clone.Pools["foo"].LastReservedIP))
		Expect(orig.Pools["foo"].Entries["id1_net0"].IPAddress).NotTo(
			Equal(clone.Pools["foo"].Entries["id1_net0"].IPAddress))
	})
})
