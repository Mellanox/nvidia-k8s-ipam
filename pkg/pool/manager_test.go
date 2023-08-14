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

package pool_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

var _ = Describe("Manager", func() {
	It("Update pool data", func() {
		testPoolName := "my-pool-1"
		testPool := &pool.IPPool{
			Name:    testPoolName,
			Subnet:  "192.168.0.0/16",
			StartIP: "192.168.0.2",
			EndIP:   "192.168.0.254",
			Gateway: "192.168.0.1",
		}
		mgr := pool.NewManager()
		Expect(mgr.GetPoolByName(testPoolName)).To(BeNil())
		mgr.UpdatePool(testPool)
		Expect(mgr.GetPoolByName(testPoolName)).NotTo(BeNil())
		Expect(mgr.GetPools()).To(HaveLen(1))
		mgr.RemovePool(testPoolName)
		Expect(mgr.GetPoolByName(testPoolName)).To(BeNil())
	})
})
