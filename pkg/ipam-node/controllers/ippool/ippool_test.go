/*
 Copyright 2025, NVIDIA CORPORATION & AFFILIATES
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

var _ = Describe("IPPool", func() {
	DescribeTable("buildPerNodeExclusions",
		func(exclusions []ipamv1alpha1.ExcludeIndexRange, startIP string, endIP string, result []pool.ExclusionRange) {
			if result == nil {
				Expect(buildPerNodeExclusions(exclusions, startIP, endIP)).To(BeEmpty())
			} else {
				Expect(buildPerNodeExclusions(exclusions, startIP, endIP)).To(Equal(result))
			}
		},
		Entry("empty exclusions",
			[]ipamv1alpha1.ExcludeIndexRange{},
			"192.168.0.1", "192.168.0.100",
			nil,
		),
		Entry("single range",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 0, EndIndex: 10},
			},
			"192.168.0.1", "192.168.0.100",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.1", EndIP: "192.168.0.11"},
			},
		),
		Entry("multiple ranges",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 0, EndIndex: 10},
				{StartIndex: 50, EndIndex: 60},
			},
			"192.168.0.1", "192.168.0.100",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.1", EndIP: "192.168.0.11"},
				{StartIP: "192.168.0.51", EndIP: "192.168.0.61"},
			},
		),
		Entry("range at the end of allocation",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 90, EndIndex: 99},
			},
			"192.168.0.1", "192.168.0.100",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.91", EndIP: "192.168.0.100"},
			},
		),
		Entry("range exceeds allocation - should be clamped",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 90, EndIndex: 200},
			},
			"192.168.0.1", "192.168.0.100",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.91", EndIP: "192.168.0.100"},
			},
		),
		Entry("range completely outside allocation - should be skipped",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 200, EndIndex: 300},
			},
			"192.168.0.1", "192.168.0.100",
			[]pool.ExclusionRange{},
		),
		Entry("IPv6 addresses",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 0, EndIndex: 5},
				{StartIndex: 250, EndIndex: 255},
			},
			"2001:db8::1", "2001:db8::100",
			[]pool.ExclusionRange{
				{StartIP: "2001:db8::1", EndIP: "2001:db8::6"},
				{StartIP: "2001:db8::fb", EndIP: "2001:db8::100"},
			},
		),
		Entry("single IP exclusion (start equals end)",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 5, EndIndex: 5},
			},
			"192.168.0.1", "192.168.0.100",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.6", EndIP: "192.168.0.6"},
			},
		),
		Entry("invalid start IP",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 0, EndIndex: 10},
			},
			"invalid", "192.168.0.100",
			nil,
		),
		Entry("invalid end IP",
			[]ipamv1alpha1.ExcludeIndexRange{
				{StartIndex: 0, EndIndex: 10},
			},
			"192.168.0.1", "invalid",
			nil,
		),
	)
})
