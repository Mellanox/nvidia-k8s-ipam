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
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

var _ = Describe("host-local types tests", func() {
	Context("HostLocalNetConfFromNetConfAndPool", func() {
		It("Converts to host-local netconf", func() {
			conf := types.NetConf{
				Name:       "my-net",
				CNIVersion: "0.4.0",
				IPAM: &types.IPAMConf{
					PoolName: "my-pool",
					DataDir:  "/foo/bar",
				},
			}
			pool := pool.IPPool{
				Name:    "my-pool",
				Subnet:  "192.168.0.0/16",
				StartIP: "192.168.0.2",
				EndIP:   "192.168.0.254",
				Gateway: "192.168.0.1",
			}

			hostlocalConf := types.HostLocalNetConfFromNetConfAndPool(&conf, &pool)

			Expect(hostlocalConf.Name).To(Equal(pool.Name))
			Expect(hostlocalConf.CNIVersion).To(Equal(conf.CNIVersion))
			Expect(hostlocalConf.IPAM.Type).To(Equal("host-local"))
			Expect(hostlocalConf.IPAM.DataDir).To(Equal(path.Join(conf.IPAM.DataDir, types.HostLocalDataDir)))
			Expect(hostlocalConf.IPAM.Ranges[0][0].Subnet).To(Equal(pool.Subnet))
			Expect(hostlocalConf.IPAM.Ranges[0][0].RangeStart).To(Equal(pool.StartIP))
			Expect(hostlocalConf.IPAM.Ranges[0][0].RangeEnd).To(Equal(pool.EndIP))
			Expect(hostlocalConf.IPAM.Ranges[0][0].Gateway).To(Equal(pool.Gateway))
		})
	})
})
