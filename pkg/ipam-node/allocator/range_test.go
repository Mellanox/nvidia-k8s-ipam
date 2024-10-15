/*
 Copyright 2023, NVIDIA CORPORATION & AFFILIATES
 Copyright 2015 CNI authors
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

package allocator_test

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/cni/pkg/types"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator"
)

var _ = Describe("IP ranges", func() {
	It("should generate sane defaults for ipv4 with a clean prefix", func() {
		subnetStr := "192.0.2.0/24"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.IP{192, 0, 2, 1},
			RangeEnd:   net.IP{192, 0, 2, 254},
		}))
	})
	It("should generate sane defaults for a smaller ipv4 subnet", func() {
		subnetStr := "192.0.2.0/25"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.IP{192, 0, 2, 1},
			RangeEnd:   net.IP{192, 0, 2, 126},
		}))
	})
	It("should generate sane defaults for a /31 ipv4 subnet", func() {
		subnetStr := "192.0.2.0/31"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.IP{192, 0, 2, 0},
			RangeEnd:   net.IP{192, 0, 2, 1},
		}))
	})
	It("should generate sane defaults for a /32 ipv4 subnet", func() {
		subnetStr := "192.0.2.10/32"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.IP{192, 0, 2, 10},
			RangeEnd:   net.IP{192, 0, 2, 10},
		}))
	})
	It("should reject ipv4 subnet using a masked address", func() {
		subnetStr := "192.0.2.12/24"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).Should(MatchError("network has host bits set. Expected subnet address is 192.0.2.0"))
	})
	It("should reject ipv6 subnet using a masked address", func() {
		subnetStr := "2001:DB8:1::24:19ff:fee1:c44a/64"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).Should(MatchError("network has host bits set. Expected subnet address is 2001:db8:1::"))
	})
	It("should reject ipv6 prefix with host bit set", func() {
		subnetStr := "2001:DB8:24:19ff::/63"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).Should(MatchError("network has host bits set. Expected subnet address is 2001:db8:24:19fe::"))
	})
	It("should reject ipv4 network with host bit set", func() {
		subnetStr := "192.168.127.0/23"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).Should(MatchError("network has host bits set. Expected subnet address is 192.168.126.0"))
	})
	It("should generate sane defaults for ipv6 with a clean prefix", func() {
		subnetStr := "2001:DB8:1::/64"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.ParseIP("2001:DB8:1::1"),
			RangeEnd:   net.ParseIP("2001:DB8:1::ffff:ffff:ffff:ffff"),
		}))
	})

	It("should generate sane defaults for /127 ipv6 prefix", func() {
		subnetStr := "2001:DB8:1::/127"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.ParseIP("2001:DB8:1::0"),
			RangeEnd:   net.ParseIP("2001:DB8:1::1"),
		}))
	})

	It("should generate sane defaults for /128 ipv6 prefix", func() {
		subnetStr := "2001:DB8:1::10/128"
		r := allocator.Range{Subnet: mustSubnet(subnetStr)}

		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.ParseIP("2001:DB8:1::10"),
			RangeEnd:   net.ParseIP("2001:DB8:1::10"),
		}))
	})

	It("should reject invalid RangeStart and RangeEnd specifications", func() {
		subnetStr := "192.0.2.0/24"
		r := allocator.Range{Subnet: mustSubnet(subnetStr), RangeStart: net.ParseIP("192.0.3.0")}
		err := r.Canonicalize()
		Expect(err).Should(MatchError("RangeStart 192.0.3.0 not in network 192.0.2.0/24"))

		r = allocator.Range{Subnet: mustSubnet(subnetStr), RangeEnd: net.ParseIP("192.0.4.0")}
		err = r.Canonicalize()
		Expect(err).Should(MatchError("RangeEnd 192.0.4.0 not in network 192.0.2.0/24"))

		r = allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.ParseIP("192.0.2.50"),
			RangeEnd:   net.ParseIP("192.0.2.40"),
		}
		err = r.Canonicalize()
		Expect(err).Should(MatchError("RangeStart 192.0.2.50 not in network 192.0.2.0/24"))
	})

	It("should reject invalid RangeStart and RangeEnd for point to point networks", func() {
		subnetStr := "192.0.2.2/31"
		r := allocator.Range{Subnet: mustSubnet(subnetStr), RangeStart: net.ParseIP("192.0.2.1")}
		err := r.Canonicalize()
		Expect(err).Should(MatchError("RangeStart 192.0.2.1 not in network 192.0.2.2/31"))

		r = allocator.Range{Subnet: mustSubnet(subnetStr), RangeEnd: net.ParseIP("192.0.2.4")}
		err = r.Canonicalize()
		Expect(err).Should(MatchError("RangeEnd 192.0.2.4 not in network 192.0.2.2/31"))

		r = allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.ParseIP("192.0.2.3"),
			RangeEnd:   net.ParseIP("192.0.2.2"),
		}
		err = r.Canonicalize()
		Expect(err).Should(MatchError("RangeStart 192.0.2.3 not in network 192.0.2.2/31"))
	})

	It("should reject invalid RangeStart and RangeEnd for single IP networks", func() {
		subnetStr := "192.0.2.2/32"
		r := allocator.Range{Subnet: mustSubnet(subnetStr), RangeStart: net.ParseIP("192.0.2.1")}
		err := r.Canonicalize()
		Expect(err).Should(MatchError("RangeStart 192.0.2.1 not in network 192.0.2.2/32"))

		r = allocator.Range{Subnet: mustSubnet(subnetStr), RangeEnd: net.ParseIP("192.0.2.3")}
		err = r.Canonicalize()
		Expect(err).Should(MatchError("RangeEnd 192.0.2.3 not in network 192.0.2.2/32"))

		r = allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.ParseIP("192.0.2.3"),
			RangeEnd:   net.ParseIP("192.0.2.2"),
		}
		err = r.Canonicalize()
		Expect(err).Should(MatchError("RangeStart 192.0.2.3 not in network 192.0.2.2/32"))
	})

	It("should parse all fields correctly", func() {
		subnetStr := "192.0.2.0/24"
		r := allocator.Range{
			Subnet:     mustSubnet(subnetStr),
			RangeStart: net.ParseIP("192.0.2.40"),
			RangeEnd:   net.ParseIP("192.0.2.50"),
			Gateway:    net.ParseIP("192.0.2.254"),
		}
		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.IP{192, 0, 2, 40},
			RangeEnd:   net.IP{192, 0, 2, 50},
			Gateway:    net.IP{192, 0, 2, 254},
		}))
	})

	It("should parse /31 and /127 correctly", func() {
		subnetStr := "192.0.2.2/31"
		r := allocator.Range{
			Subnet:     mustSubnet(subnetStr),
			RangeStart: net.ParseIP("192.0.2.2"),
			RangeEnd:   net.ParseIP("192.0.2.3"),
			Gateway:    net.ParseIP("192.0.2.3"),
		}
		Expect(r.Canonicalize()).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.IP{192, 0, 2, 2},
			RangeEnd:   net.IP{192, 0, 2, 3},
			Gateway:    net.IP{192, 0, 2, 3},
		}))

		subnetV6Str := "2001:DB8:1::4/127"
		r = allocator.Range{
			Subnet:     mustSubnet(subnetV6Str),
			RangeStart: net.ParseIP("2001:DB8:1::4"),
			RangeEnd:   net.ParseIP("2001:DB8:1::5"),
			Gateway:    net.ParseIP("2001:DB8:1::4"),
		}
		Expect(r.Canonicalize()).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     mustSubnet(subnetV6Str),
			RangeStart: net.ParseIP("2001:DB8:1::4"),
			RangeEnd:   net.ParseIP("2001:DB8:1::5"),
			Gateway:    net.ParseIP("2001:DB8:1::4"),
		}))
	})

	It("Should parse /32 and /128 correctly", func() {
		subnetStr := "192.0.2.2/32"
		r := allocator.Range{
			Subnet:     mustSubnet(subnetStr),
			RangeStart: net.ParseIP("192.0.2.2"),
			RangeEnd:   net.ParseIP("192.0.2.2"),
			Gateway:    net.ParseIP("192.0.2.2"),
		}
		Expect(r.Canonicalize()).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.IP{192, 0, 2, 2},
			RangeEnd:   net.IP{192, 0, 2, 2},
			Gateway:    net.IP{192, 0, 2, 2},
		}))

		subnetV6Str := "2001:DB8:1::4/128"
		r = allocator.Range{
			Subnet:     mustSubnet(subnetV6Str),
			RangeStart: net.ParseIP("2001:DB8:1::4"),
			RangeEnd:   net.ParseIP("2001:DB8:1::4"),
			Gateway:    net.ParseIP("2001:DB8:1::4"),
		}
		Expect(r.Canonicalize()).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     mustSubnet(subnetV6Str),
			RangeStart: net.ParseIP("2001:DB8:1::4"),
			RangeEnd:   net.ParseIP("2001:DB8:1::4"),
			Gateway:    net.ParseIP("2001:DB8:1::4"),
		}))
	})

	It("Should handle single IP range in a large subnet correctly", func() {
		subnetStr := "192.0.2.0/24"
		r := allocator.Range{
			Subnet:     mustSubnet(subnetStr),
			RangeStart: net.ParseIP("192.0.2.10"),
			RangeEnd:   net.ParseIP("192.0.2.10"),
			Gateway:    net.ParseIP("192.0.2.1"),
		}
		Expect(r.Canonicalize()).NotTo(HaveOccurred())

		Expect(r).To(Equal(allocator.Range{
			Subnet:     networkSubnet(subnetStr),
			RangeStart: net.IP{192, 0, 2, 10},
			RangeEnd:   net.IP{192, 0, 2, 10},
			Gateway:    net.IP{192, 0, 2, 1},
		}))
	})

	It("should accept v4 IPs in range and reject IPs out of range", func() {
		r := allocator.Range{
			Subnet:     mustSubnet("192.0.2.0/24"),
			RangeStart: net.ParseIP("192.0.2.40"),
			RangeEnd:   net.ParseIP("192.0.2.50"),
			Gateway:    net.ParseIP("192.0.2.254"),
		}
		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r.Contains(net.ParseIP("192.0.3.0"))).Should(BeFalse())

		Expect(r.Contains(net.ParseIP("192.0.2.39"))).Should(BeFalse())
		Expect(r.Contains(net.ParseIP("192.0.2.40"))).Should(BeTrue())
		Expect(r.Contains(net.ParseIP("192.0.2.50"))).Should(BeTrue())
		Expect(r.Contains(net.ParseIP("192.0.2.51"))).Should(BeFalse())
	})

	It("/31 network Contains", func() {
		r := allocator.Range{
			Subnet: mustSubnet("192.0.2.2/31"),
		}
		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r.Contains(net.ParseIP("192.0.2.0"))).Should(BeFalse())
		Expect(r.Contains(net.ParseIP("192.0.2.1"))).Should(BeFalse())
		Expect(r.Contains(net.ParseIP("192.0.2.2"))).Should(BeTrue())
		Expect(r.Contains(net.ParseIP("192.0.2.3"))).Should(BeTrue())
		Expect(r.Contains(net.ParseIP("192.0.2.4"))).Should(BeFalse())
	})

	It("/127 network Contains", func() {
		r := allocator.Range{
			Subnet: mustSubnet("2001:DB8:1::2/127"),
		}
		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())

		Expect(r.Contains(net.ParseIP("2001:DB8:1::0"))).Should(BeFalse())
		Expect(r.Contains(net.ParseIP("2001:DB8:1::1"))).Should(BeFalse())
		Expect(r.Contains(net.ParseIP("2001:DB8:1::2"))).Should(BeTrue())
		Expect(r.Contains(net.ParseIP("2001:DB8:1::3"))).Should(BeTrue())
		Expect(r.Contains(net.ParseIP("2001:DB8:1::4"))).Should(BeFalse())
	})

	It("should accept v6 IPs in range and reject IPs out of range", func() {
		r := allocator.Range{
			Subnet:     mustSubnet("2001:DB8:1::/64"),
			RangeStart: net.ParseIP("2001:db8:1::40"),
			RangeEnd:   net.ParseIP("2001:db8:1::50"),
		}
		err := r.Canonicalize()
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Contains(net.ParseIP("2001:db8:2::"))).Should(BeFalse())

		Expect(r.Contains(net.ParseIP("2001:db8:1::39"))).Should(BeFalse())
		Expect(r.Contains(net.ParseIP("2001:db8:1::40"))).Should(BeTrue())
		Expect(r.Contains(net.ParseIP("2001:db8:1::50"))).Should(BeTrue())
		Expect(r.Contains(net.ParseIP("2001:db8:1::51"))).Should(BeFalse())
	})

	DescribeTable("Detecting overlap",
		func(r1 allocator.Range, r2 allocator.Range, expected bool) {
			Expect(r1.Canonicalize()).NotTo(HaveOccurred())
			Expect(r2.Canonicalize()).NotTo(HaveOccurred())

			// operation should be commutative
			Expect(r1.Overlaps(&r2)).To(Equal(expected))
			Expect(r2.Overlaps(&r1)).To(Equal(expected))
		},
		Entry("non-overlapping",
			allocator.Range{Subnet: mustSubnet("10.0.0.0/24")},
			allocator.Range{Subnet: mustSubnet("10.0.1.0/24")},
			false),
		Entry("different families",
			// Note that the bits overlap
			allocator.Range{Subnet: mustSubnet("0.0.0.0/24")},
			allocator.Range{Subnet: mustSubnet("::/24")},
			false),
		Entry("Identical",
			allocator.Range{Subnet: mustSubnet("10.0.0.0/24")},
			allocator.Range{Subnet: mustSubnet("10.0.0.0/24")},
			true),
		Entry("Containing",
			allocator.Range{Subnet: mustSubnet("10.0.0.0/20")},
			allocator.Range{Subnet: mustSubnet("10.0.1.0/24")},
			true),
		Entry("same subnet, non overlapping start + end",
			allocator.Range{
				Subnet:   mustSubnet("10.0.0.0/24"),
				RangeEnd: net.ParseIP("10.0.0.127"),
			},
			allocator.Range{
				Subnet:     mustSubnet("10.0.0.0/24"),
				RangeStart: net.ParseIP("10.0.0.128"),
			},
			false),
		Entry("same subnet, overlapping start + end",
			allocator.Range{
				Subnet:   mustSubnet("10.0.0.0/24"),
				RangeEnd: net.ParseIP("10.0.0.127"),
			},
			allocator.Range{
				Subnet:     mustSubnet("10.0.0.0/24"),
				RangeStart: net.ParseIP("10.0.0.127"),
			},
			true),
	)
})

func mustSubnet(s string) types.IPNet {
	n, err := types.ParseCIDR(s)
	if err != nil {
		Fail(err.Error())
	}
	_ = allocator.CanonicalizeIP(&n.IP)
	return types.IPNet(*n)
}

func networkSubnet(s string) types.IPNet {
	ipNet := mustSubnet(s)
	ipNet.IP = ipNet.IP.Mask(ipNet.Mask)
	return ipNet
}
