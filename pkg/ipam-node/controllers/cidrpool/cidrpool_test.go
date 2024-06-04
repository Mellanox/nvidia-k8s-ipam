/*
 Copyright 2024, NVIDIA CORPORATION & AFFILIATES
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

package controllers

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

var _ = Describe("CIDRPool", func() {
	DescribeTable("buildExclusions",
		func(exclusions []ipamv1alpha1.ExcludeRange, nodeSubnet string, firstIP string, lastIP string, result []pool.ExclusionRange) {
			_, subnet, _ := net.ParseCIDR(nodeSubnet)
			first := net.ParseIP(firstIP)
			last := net.ParseIP(lastIP)
			Expect(buildExclusions(exclusions, subnet, first, last)).To(Equal(result))
		},
		Entry("start and end are part of the subnet",
			[]ipamv1alpha1.ExcludeRange{
				{StartIP: "192.168.0.10", EndIP: "192.168.0.20"},
				{StartIP: "192.168.0.30", EndIP: "192.168.0.40"}},
			"192.168.0.0/24", "192.168.0.1", "192.168.0.254",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.10", EndIP: "192.168.0.20"},
				{StartIP: "192.168.0.30", EndIP: "192.168.0.40"},
			},
		),
		Entry("start and end are out of the subnet",
			[]ipamv1alpha1.ExcludeRange{
				{StartIP: "192.168.100.10", EndIP: "192.168.100.20"},
				{StartIP: "192.168.0.30", EndIP: "192.168.0.40"}},
			"192.168.0.0/24", "192.168.0.1", "192.168.0.254",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.30", EndIP: "192.168.0.40"},
			},
		),
		Entry("start is out of the subnet",
			[]ipamv1alpha1.ExcludeRange{
				{StartIP: "192.168.0.30", EndIP: "192.168.1.40"}},
			"192.168.1.0/24", "192.168.1.1", "192.168.1.254",
			[]pool.ExclusionRange{
				{StartIP: "192.168.1.1", EndIP: "192.168.1.40"},
			},
		),
		Entry("end is out of the subnet",
			[]ipamv1alpha1.ExcludeRange{
				{StartIP: "192.168.1.30", EndIP: "192.168.2.40"}},
			"192.168.1.0/24", "192.168.1.1", "192.168.1.254",
			[]pool.ExclusionRange{
				{StartIP: "192.168.1.30", EndIP: "192.168.1.254"},
			},
		),
	)
})
