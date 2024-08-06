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

package v1alpha1_test

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
)

var _ = Describe("Helpers", func() {
	DescribeTable("GetGatewayForSubnet",
		func(subnet string, index int, gw string) {
			_, network, err := net.ParseCIDR(subnet)
			Expect(err).NotTo(HaveOccurred())
			ret := v1alpha1.GetGatewayForSubnet(network, int32(index))
			Expect(ret).To(Equal(gw))
		},
		Entry("ipv4 - start range", "192.168.1.0/24", 1, "192.168.1.1"),
		Entry("ipv4 - mid range", "192.168.1.0/24", 33, "192.168.1.33"),
		Entry("ipv4 - end range", "192.168.1.0/24", 254, "192.168.1.254"),
		Entry("ipv4 - out of range, can't use index 0", "192.168.1.0/24", 0, ""),
		Entry("ipv4 - out of range, too big", "192.168.1.0/24", 255, ""),
		Entry("ipv4 - single IP", "192.168.1.100/32", 0, "192.168.1.100"),
		Entry("ipv4 - single IP, out of range", "192.168.1.100/32", 1, ""),
		Entry("ipv4 - point to point, can use index 0", "192.168.1.0/31", 0, "192.168.1.0"),
		Entry("ipv4 - point to point, can use index 1", "192.168.1.0/31", 1, "192.168.1.1"),
		Entry("ipv4 - point to point, out of range", "192.168.1.0/31", 2, ""),
		Entry("ipv6 - start range", "2001:db8:3333:4444::0/120", 1, "2001:db8:3333:4444::1"),
		Entry("ipv6 - mid range", "2001:db8:3333:4444::0/120", 10, "2001:db8:3333:4444::a"),
		Entry("ipv6 - end range", "2001:db8:3333:4444::0/120", 255, "2001:db8:3333:4444::ff"),
		Entry("ipv6 - out of range, can't use index 0", "2001:db8:3333:4444::0/120", 0, ""),
		Entry("ipv6 - out of range, too big", "2001:db8:3333:4444::0/120", 256, ""),
		Entry("ipv6 - point to point, can use index 0", "2001:db8:3333:4444::0/127", 0, "2001:db8:3333:4444::"),
		Entry("ipv6 - point to point, can use index 1", "2001:db8:3333:4444::0/127", 1, "2001:db8:3333:4444::1"),
		Entry("ipv6 - point to point, out of range", "2001:db8:3333:4444::0/127", 2, ""),
	)
	DescribeTable("GetPossibleIPCount",
		func(subnet string, count int) {
			_, network, err := net.ParseCIDR(subnet)
			Expect(err).NotTo(HaveOccurred())
			ret := v1alpha1.GetPossibleIPCount(network)
			Expect(ret).NotTo(BeNil())
			Expect(ret.IsUint64()).To(BeTrue())
			Expect(ret.Uint64()).To(Equal(uint64(count)))
		},
		Entry("ipv4 /24", "192.168.0.0/24", 254),
		Entry("ipv4 /25", "192.168.0.0/25", 126),
		Entry("ipv4 /16", "192.168.0.0/16", 65534),
		Entry("ipv4 /31", "192.20.20.0/31", 2),
		Entry("ipv4 /32", "10.10.10.10/32", 1),
		Entry("ipv6 /124", "2001:db8:85a3::8a2e:370:7334/120", 255),
		Entry("ipv6 /96", "2001:db8:85a3::8a2e:370:7334/96", 4294967295),
		Entry("ipv6 /127", "2001:db8:85a3::8a2e:370:7334/127", 2),
		Entry("ipv6 /128", "2001:db8:85a3::8a2e:370:7334/128", 1),
	)
})
