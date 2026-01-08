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

	DescribeTable("ExcludeIndexRangeToExcludeRange",
		func(excludeIndexRanges []v1alpha1.ExcludeIndexRange, startIP string, expected []v1alpha1.ExcludeRange) {
			result := v1alpha1.ExcludeIndexRangeToExcludeRange(excludeIndexRanges, startIP)
			Expect(result).To(Equal(expected))
		},
		Entry("nil input", nil, "192.168.1.0", nil),

		Entry("empty input", []v1alpha1.ExcludeIndexRange{}, "192.168.1.0", nil),

		Entry("invalid startIP", []v1alpha1.ExcludeIndexRange{{StartIndex: 1, EndIndex: 5}}, "invalid", nil),

		Entry("ipv4 - single index range at beginning",
			[]v1alpha1.ExcludeIndexRange{{StartIndex: 0, EndIndex: 0}},
			"192.168.1.0",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.0", EndIP: "192.168.1.0"}}),

		Entry("ipv4 - single index range offset",
			[]v1alpha1.ExcludeIndexRange{{StartIndex: 5, EndIndex: 10}},
			"192.168.1.0",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.5", EndIP: "192.168.1.10"}}),

		Entry("ipv4 - multiple non-overlapping index ranges",
			[]v1alpha1.ExcludeIndexRange{
				{StartIndex: 1, EndIndex: 5},
				{StartIndex: 10, EndIndex: 15},
				{StartIndex: 100, EndIndex: 110},
			},
			"192.168.1.0",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.1", EndIP: "192.168.1.5"},
				{StartIP: "192.168.1.10", EndIP: "192.168.1.15"},
				{StartIP: "192.168.1.100", EndIP: "192.168.1.110"},
			}),

		Entry("ipv6 - single index range at beginning",
			[]v1alpha1.ExcludeIndexRange{{StartIndex: 0, EndIndex: 0}},
			"2001:db8:3333:4444::",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8:3333:4444::", EndIP: "2001:db8:3333:4444::"}}),

		Entry("ipv6 - single index range offset",
			[]v1alpha1.ExcludeIndexRange{{StartIndex: 5, EndIndex: 10}},
			"2001:db8:3333:4444::",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8:3333:4444::5", EndIP: "2001:db8:3333:4444::a"}}),

		Entry("ipv6 - single index range larger offset",
			[]v1alpha1.ExcludeIndexRange{{StartIndex: 256, EndIndex: 512}},
			"2001:db8:3333:4444::",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8:3333:4444::100", EndIP: "2001:db8:3333:4444::200"}}),

		Entry("ipv6 - multiple non-overlapping index ranges",
			[]v1alpha1.ExcludeIndexRange{
				{StartIndex: 1, EndIndex: 5},
				{StartIndex: 16, EndIndex: 32},
			},
			"2001:db8:3333:4444::",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8:3333:4444::1", EndIP: "2001:db8:3333:4444::5"},
				{StartIP: "2001:db8:3333:4444::10", EndIP: "2001:db8:3333:4444::20"},
			}),

		Entry("ipv4 - same start and end index",
			[]v1alpha1.ExcludeIndexRange{{StartIndex: 100, EndIndex: 100}},
			"10.0.0.0",
			[]v1alpha1.ExcludeRange{{StartIP: "10.0.0.100", EndIP: "10.0.0.100"}}),

		Entry("ipv6 - same start and end index",
			[]v1alpha1.ExcludeIndexRange{{StartIndex: 255, EndIndex: 255}},
			"2001:db8::",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::ff", EndIP: "2001:db8::ff"}}),
	)

	DescribeTable("ClampExcludeRanges",
		func(excludeRanges []v1alpha1.ExcludeRange, startIP string, endIP string, expected []v1alpha1.ExcludeRange) {
			result := v1alpha1.ClampExcludeRanges(excludeRanges, startIP, endIP)
			Expect(result).To(Equal(expected))
		},
		Entry("nil input", nil, "192.168.1.0", "192.168.1.255", nil),

		Entry("empty input", []v1alpha1.ExcludeRange{}, "192.168.1.0", "192.168.1.255", nil),

		Entry("invalid startIP", []v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.20"}},
			"invalid", "192.168.1.255", nil),

		Entry("invalid endIP", []v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.20"}},
			"192.168.1.0", "invalid", nil),

		Entry("invalid exclude range startIP",
			[]v1alpha1.ExcludeRange{{StartIP: "invalid", EndIP: "192.168.1.20"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{}),

		Entry("invalid exclude range endIP",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "invalid"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{}),

		Entry("ipv4 - exclude range completely within bounds",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.50", EndIP: "192.168.1.100"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.50", EndIP: "192.168.1.100"}}),

		Entry("ipv4 - exclude range completely before range",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.0.1", EndIP: "192.168.0.50"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{}),

		Entry("ipv4 - exclude range completely after range",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.2.1", EndIP: "192.168.2.50"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{}),

		Entry("ipv4 - exclude range start before, end inside",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.0.200", EndIP: "192.168.1.50"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.0", EndIP: "192.168.1.50"}}),

		Entry("ipv4 - exclude range start inside, end after",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.200", EndIP: "192.168.2.50"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.200", EndIP: "192.168.1.255"}}),

		Entry("ipv4 - exclude range spanning entire range",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.0.0", EndIP: "192.168.2.255"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.0", EndIP: "192.168.1.255"}}),

		Entry("ipv4 - multiple ranges mixed scenarios",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.0.100", EndIP: "192.168.0.200"}, // completely before - omitted
				{StartIP: "192.168.0.200", EndIP: "192.168.1.50"},  // start before, end inside - clamped
				{StartIP: "192.168.1.100", EndIP: "192.168.1.150"}, // completely within - unchanged
				{StartIP: "192.168.1.200", EndIP: "192.168.2.50"},  // start inside, end after - clamped
				{StartIP: "192.168.2.100", EndIP: "192.168.2.200"}, // completely after - omitted
			},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.0", EndIP: "192.168.1.50"},
				{StartIP: "192.168.1.100", EndIP: "192.168.1.150"},
				{StartIP: "192.168.1.200", EndIP: "192.168.1.255"},
			}),

		Entry("ipv4 - exclude range exactly matches range",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.0", EndIP: "192.168.1.255"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.0", EndIP: "192.168.1.255"}}),

		Entry("ipv4 - single IP exclude range within bounds",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.100", EndIP: "192.168.1.100"}},
			"192.168.1.0", "192.168.1.255",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.100", EndIP: "192.168.1.100"}}),

		Entry("ipv6 - exclude range completely within bounds",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::50", EndIP: "2001:db8::100"}},
			"2001:db8::", "2001:db8::ffff",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::50", EndIP: "2001:db8::100"}}),

		Entry("ipv6 - exclude range completely before range",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db7::1", EndIP: "2001:db7::50"}},
			"2001:db8::", "2001:db8::ffff",
			[]v1alpha1.ExcludeRange{}),

		Entry("ipv6 - exclude range completely after range",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db9::1", EndIP: "2001:db9::50"}},
			"2001:db8::", "2001:db8::ffff",
			[]v1alpha1.ExcludeRange{}),

		Entry("ipv6 - exclude range start before, end inside",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db7:ffff:ffff:ffff:ffff:ffff:ff00", EndIP: "2001:db8::50"}},
			"2001:db8::", "2001:db8::ffff",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::", EndIP: "2001:db8::50"}}),

		Entry("ipv6 - exclude range start inside, end after",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::ff00", EndIP: "2001:db8::1:50"}},
			"2001:db8::", "2001:db8::ffff",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::ff00", EndIP: "2001:db8::ffff"}}),

		Entry("ipv6 - multiple ranges mixed scenarios",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db7::100", EndIP: "2001:db7::200"},  // completely before - omitted
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},    // within bounds
				{StartIP: "2001:db8::ff00", EndIP: "2001:db8::1:0"}, // end after - clamped
			},
			"2001:db8::", "2001:db8::ffff",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},
				{StartIP: "2001:db8::ff00", EndIP: "2001:db8::ffff"},
			}),
	)

	DescribeTable("MergeExcludeRanges",
		func(excludeRanges []v1alpha1.ExcludeRange, expected []v1alpha1.ExcludeRange) {
			result := v1alpha1.MergeExcludeRanges(excludeRanges)
			Expect(result).To(Equal(expected))
		},
		Entry("nil input", nil, nil),

		Entry("empty input", []v1alpha1.ExcludeRange{}, nil),

		Entry("invalid startIP in range",
			[]v1alpha1.ExcludeRange{{StartIP: "invalid", EndIP: "192.168.1.20"}},
			nil),

		Entry("invalid endIP in range",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "invalid"}},
			nil),

		Entry("mix of valid and invalid ranges",
			[]v1alpha1.ExcludeRange{
				{StartIP: "invalid", EndIP: "192.168.1.20"},
				{StartIP: "192.168.1.50", EndIP: "192.168.1.60"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.50", EndIP: "192.168.1.60"}}),

		Entry("ipv4 - single range",
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.20"}},
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.20"}}),

		Entry("ipv4 - duplicate range",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.20"}}),

		Entry("ipv4 - non-overlapping ranges",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
				{StartIP: "192.168.1.50", EndIP: "192.168.1.60"},
				{StartIP: "192.168.1.100", EndIP: "192.168.1.110"},
			},
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
				{StartIP: "192.168.1.50", EndIP: "192.168.1.60"},
				{StartIP: "192.168.1.100", EndIP: "192.168.1.110"},
			}),

		// IPv4 - non-overlapping ranges unsorted (should be sorted in output)
		Entry("ipv4 - non-overlapping ranges unsorted",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.100", EndIP: "192.168.1.110"},
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
				{StartIP: "192.168.1.50", EndIP: "192.168.1.60"},
			},
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
				{StartIP: "192.168.1.50", EndIP: "192.168.1.60"},
				{StartIP: "192.168.1.100", EndIP: "192.168.1.110"},
			}),

		Entry("ipv4 - two overlapping ranges",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.30"},
				{StartIP: "192.168.1.20", EndIP: "192.168.1.40"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.40"}}),

		Entry("ipv4 - adjacent ranges merged",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
				{StartIP: "192.168.1.21", EndIP: "192.168.1.30"},
			},
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.30"},
			}),

		Entry("ipv4 - touching ranges merged",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
				{StartIP: "192.168.1.20", EndIP: "192.168.1.30"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.30"}}),

		Entry("ipv4 - range contained within another",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.30", EndIP: "192.168.1.50"},
				{StartIP: "192.168.1.10", EndIP: "192.168.1.100"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.100"}}),

		Entry("ipv4 - multiple overlapping ranges",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.30"},
				{StartIP: "192.168.1.25", EndIP: "192.168.1.50"},
				{StartIP: "192.168.1.45", EndIP: "192.168.1.70"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.10", EndIP: "192.168.1.70"}}),

		Entry("ipv4 - complex scenario",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},   // group 1
				{StartIP: "192.168.1.21", EndIP: "192.168.1.30"},   // group 1 - adjacent
				{StartIP: "192.168.1.100", EndIP: "192.168.1.110"}, // group 2 - separate
				{StartIP: "192.168.1.200", EndIP: "192.168.1.220"}, // group 3
				{StartIP: "192.168.1.210", EndIP: "192.168.1.250"}, // group 3 - overlaps
			},
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.10", EndIP: "192.168.1.30"},
				{StartIP: "192.168.1.100", EndIP: "192.168.1.110"},
				{StartIP: "192.168.1.200", EndIP: "192.168.1.250"},
			}),

		Entry("ipv4 - all ranges merge into one",
			[]v1alpha1.ExcludeRange{
				{StartIP: "192.168.1.1", EndIP: "192.168.1.50"},
				{StartIP: "192.168.1.40", EndIP: "192.168.1.100"},
				{StartIP: "192.168.1.90", EndIP: "192.168.1.150"},
				{StartIP: "192.168.1.140", EndIP: "192.168.1.200"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "192.168.1.1", EndIP: "192.168.1.200"}}),

		Entry("ipv6 - single range",
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::10", EndIP: "2001:db8::20"}},
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::10", EndIP: "2001:db8::20"}}),

		Entry("ipv6 - non-overlapping ranges",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},
				{StartIP: "2001:db8::50", EndIP: "2001:db8::60"},
			},
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},
				{StartIP: "2001:db8::50", EndIP: "2001:db8::60"},
			}),

		Entry("ipv6 - two overlapping ranges",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::30"},
				{StartIP: "2001:db8::20", EndIP: "2001:db8::40"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::10", EndIP: "2001:db8::40"}}),

		Entry("ipv6 - touching ranges merged",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},
				{StartIP: "2001:db8::20", EndIP: "2001:db8::30"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::10", EndIP: "2001:db8::30"}}),

		Entry("ipv6 - adjacent ranges merged",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},
				{StartIP: "2001:db8::21", EndIP: "2001:db8::30"},
				{StartIP: "2001:db8::31", EndIP: "2001:db8::40"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::10", EndIP: "2001:db8::40"}}),

		Entry("ipv6 - range contained within another",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::100"},
				{StartIP: "2001:db8::30", EndIP: "2001:db8::50"},
			},
			[]v1alpha1.ExcludeRange{{StartIP: "2001:db8::10", EndIP: "2001:db8::100"}}),

		Entry("ipv6 - complex scenario",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},
				{StartIP: "2001:db8::15", EndIP: "2001:db8::30"},
				{StartIP: "2001:db8::100", EndIP: "2001:db8::110"},
				{StartIP: "2001:db8::200", EndIP: "2001:db8::220"},
				{StartIP: "2001:db8::210", EndIP: "2001:db8::250"},
			},
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::30"},
				{StartIP: "2001:db8::100", EndIP: "2001:db8::110"},
				{StartIP: "2001:db8::200", EndIP: "2001:db8::250"},
			}),

		Entry("ipv6 - unsorted input",
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::100", EndIP: "2001:db8::110"},
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},
			},
			[]v1alpha1.ExcludeRange{
				{StartIP: "2001:db8::10", EndIP: "2001:db8::20"},
				{StartIP: "2001:db8::100", EndIP: "2001:db8::110"},
			}),
	)

	DescribeTable("ExcludeRange.Contains",
		func(excludeRange v1alpha1.ExcludeRange, ip net.IP, expected bool) {
			Expect(excludeRange.Contains(ip)).To(Equal(expected))
		},
		Entry("ipv4 - IP before range",
			v1alpha1.ExcludeRange{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
			net.ParseIP("192.168.1.9"),
			false),
		Entry("ipv4 - IP after range",
			v1alpha1.ExcludeRange{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
			net.ParseIP("192.168.1.21"),
			false),
		Entry("ipv4 - IP at start of range",
			v1alpha1.ExcludeRange{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
			net.ParseIP("192.168.1.10"),
			true),
		Entry("ipv4 - IP at end of range",
			v1alpha1.ExcludeRange{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
			net.ParseIP("192.168.1.20"),
			true),
		Entry("ipv4 - IP in range",
			v1alpha1.ExcludeRange{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
			net.ParseIP("192.168.1.15"),
			true),
		Entry("ipv4 - IP nil",
			v1alpha1.ExcludeRange{StartIP: "192.168.1.10", EndIP: "192.168.1.20"},
			nil,
			false),
		Entry("ipv4 - invalid startIP",
			v1alpha1.ExcludeRange{StartIP: "invalid", EndIP: "192.168.1.20"},
			net.ParseIP("192.168.1.15"),
			false),
		Entry("ipv4 - invalid endIP",
			v1alpha1.ExcludeRange{StartIP: "192.168.1.10", EndIP: "invalid"},
			net.ParseIP("192.168.1.15"),
			false),
	)
})
