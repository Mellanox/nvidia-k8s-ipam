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

package types_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

var _ = Describe("Checksum", func() {
	It("Equal", func() {
		Expect(types.NewChecksum(types.NewRoot()).Verify(types.NewRoot())).NotTo(HaveOccurred())
	})
	It("Diff", func() {
		r1 := types.NewRoot()
		r1.Pools["foo"] = *types.NewPoolReservations("foo")
		Expect(types.NewChecksum(r1).Verify(types.NewRoot())).To(HaveOccurred())
	})
	It("ReleasedAt does not affect the v1 checksum, so an older binary can still verify it", func() {
		r := types.NewRoot()
		r.Pools["pool1"] = *types.NewPoolReservations("pool1")
		r.Pools["pool1"].Entries["id1_net0"] = types.Reservation{ContainerID: "id1", InterfaceName: "net0"}
		checksumBeforeRelease := types.NewChecksum(r)

		entry := r.Pools["pool1"].Entries["id1_net0"]
		entry.ReleasedAt = time.Now()
		r.Pools["pool1"].Entries["id1_net0"] = entry

		Expect(types.NewChecksum(r)).To(Equal(checksumBeforeRelease),
			"setting ReleasedAt must not change the checksum, or a store file with a "+
				"cooldown-pending reservation would fail 'checksum mismatch' on an older "+
				"binary that predates ReleasedAt and silently drops it on load")
		Expect(checksumBeforeRelease.Verify(r)).NotTo(HaveOccurred())
	})
})
