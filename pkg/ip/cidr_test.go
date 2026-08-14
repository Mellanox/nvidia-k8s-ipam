// Copyright 2022 CNI authors
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

package ip

import (
	"math/big"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CIDR functions", func() {
	It("NextIP", func() {
		testCases := []struct {
			ip     net.IP
			nextIP net.IP
		}{
			{
				[]byte{192, 0, 2},
				nil,
			},
			{
				net.ParseIP("192.168.0.1"),
				net.IPv4(192, 168, 0, 2).To4(),
			},
			{
				net.ParseIP("192.168.0.255"),
				net.IPv4(192, 168, 1, 0).To4(),
			},
			{
				net.ParseIP("0.1.0.5"),
				net.IPv4(0, 1, 0, 6).To4(),
			},
			{
				net.ParseIP("AB12::123"),
				net.ParseIP("AB12::124"),
			},
			{
				net.ParseIP("AB12::FFFF"),
				net.ParseIP("AB12::1:0"),
			},
			{
				net.ParseIP("0::123"),
				net.ParseIP("0::124"),
			},
			{net.ParseIP("255.255.255.255"),
				nil,
			},
		}

		for _, test := range testCases {
			ip := NextIP(test.ip)

			Expect(ip).To(Equal(test.nextIP))
		}
	})

	It("NextIPWithOffset", func() {
		testCases := []struct {
			ip          net.IP
			offset      *big.Int
			nextIP      net.IP
			expectError bool
		}{
			{
				[]byte{192, 0, 2},
				big.NewInt(10),
				nil,
				true,
			},
			{
				net.ParseIP("192.168.0.1"),
				big.NewInt(10),
				net.IPv4(192, 168, 0, 11).To4(),
				false,
			},
			{
				net.ParseIP("192.168.0.254"),
				big.NewInt(10),
				net.IPv4(192, 168, 1, 8).To4(),
				false,
			},
			{
				net.ParseIP("192.168.0.254"),
				big.NewInt(-10),
				nil,
				true,
			},
			{
				net.ParseIP("0::123"),
				big.NewInt(3),
				net.ParseIP("0::126"),
				false,
			},
			{
				net.ParseIP("AB12::FFFF"),
				big.NewInt(3),
				net.ParseIP("AB12::1:2"),
				false,
			},
			{
				net.ParseIP("192.168.0.1"),
				nil,
				nil,
				true,
			},
		}

		for _, test := range testCases {
			nextIP, err := NextIPWithOffset(test.ip, test.offset)
			if test.expectError {
				Expect(err).To(HaveOccurred())
				Expect(nextIP).To(BeNil())
				continue
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(nextIP).To(Equal(test.nextIP))
		}
	})

	It("Distance", func() {
		testCases := []struct {
			ipA         net.IP
			ipB         net.IP
			count       int64
			expectError bool
		}{
			{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.11"),
				10,
				false,
			},
			{
				net.ParseIP("192.168.0.2"),
				net.ParseIP("192.168.0.2"),
				0,
				false,
			},
			{
				net.ParseIP("AB12::FFFF"),
				net.ParseIP("AB12::1:2"),
				3,
				false,
			},
			{
				net.ParseIP("192.168.0.11"),
				net.ParseIP("192.168.0.1"),
				0,
				true,
			},
			{
				net.ParseIP("192.168.0.11"),
				[]byte{192, 0, 2},
				0,
				true,
			},
			{
				net.ParseIP("192.168.0.11"),
				net.ParseIP("AB12::FFFF"),
				0,
				true,
			},
		}

		for _, test := range testCases {
			distance, err := Distance(test.ipA, test.ipB)
			if test.expectError {
				Expect(err).To(HaveOccurred())
				Expect(distance).To(BeNil())
				continue
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(distance.Cmp(big.NewInt(test.count))).To(BeZero())
		}
	})

	It("Distance and NextIPWithOffset support distances larger than int64", func() {
		start := net.ParseIP("2001:db8::11")
		end := net.ParseIP("2001:db8:0:1::11")
		expected := new(big.Int).Lsh(big.NewInt(1), 64)

		distance, err := Distance(start, end)
		Expect(err).NotTo(HaveOccurred())
		Expect(distance).To(Equal(expected))

		result, err := NextIPWithOffset(start, distance)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(end))
	})

	It("exact address arithmetic rejects invalid operations", func() {
		_, err := Distance(net.ParseIP("2001:db8::1"), net.ParseIP("192.0.2.1"))
		Expect(err).To(HaveOccurred())
		_, err = Distance(net.ParseIP("2001:db8::2"), net.ParseIP("2001:db8::1"))
		Expect(err).To(HaveOccurred())
		_, err = NextIPWithOffset(net.ParseIP("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"), big.NewInt(1))
		Expect(err).To(HaveOccurred())
	})

	It("PrevIP", func() {
		testCases := []struct {
			ip     net.IP
			prevIP net.IP
		}{
			{
				[]byte{192, 0, 2},
				nil,
			},
			{
				net.ParseIP("192.168.0.2"),
				net.IPv4(192, 168, 0, 1).To4(),
			},
			{
				net.ParseIP("192.168.1.0"),
				net.IPv4(192, 168, 0, 255).To4(),
			},
			{
				net.ParseIP("0.1.0.5"),
				net.IPv4(0, 1, 0, 4).To4(),
			},
			{
				net.ParseIP("AB12::123"),
				net.ParseIP("AB12::122"),
			},
			{
				net.ParseIP("AB12::1:0"),
				net.ParseIP("AB12::FFFF"),
			},
			{
				net.ParseIP("0::124"),
				net.ParseIP("0::123"),
			},
			{
				net.ParseIP("0.0.0.0"),
				nil,
			},
		}

		for _, test := range testCases {
			ip := PrevIP(test.ip)

			Expect(ip).To(Equal(test.prevIP))
		}
	})

	It("Cmp", func() {
		testCases := []struct {
			a      net.IP
			b      net.IP
			result int
		}{
			{
				net.ParseIP("192.168.0.2"),
				nil,
				-2,
			},
			{
				net.ParseIP("192.168.0.2"),
				[]byte{192, 168, 5},
				-2,
			},
			{
				net.ParseIP("192.168.0.2"),
				net.ParseIP("AB12::123"),
				-2,
			},
			{
				net.ParseIP("192.168.0.2"),
				net.ParseIP("192.168.0.5"),
				-1,
			},
			{
				net.ParseIP("192.168.0.2"),
				net.ParseIP("192.168.0.5").To4(),
				-1,
			},
			{
				net.ParseIP("192.168.0.10"),
				net.ParseIP("192.168.0.5"),
				1,
			},
			{
				net.ParseIP("192.168.0.10"),
				net.ParseIP("192.168.0.10"),
				0,
			},
			{
				net.ParseIP("192.168.0.10"),
				net.ParseIP("192.168.0.10").To4(),
				0,
			},
			{
				net.ParseIP("AB12::122"),
				net.ParseIP("AB12::123"),
				-1,
			},
			{
				net.ParseIP("AB12::210"),
				net.ParseIP("AB12::123"),
				1,
			},
			{
				net.ParseIP("AB12::210"),
				net.ParseIP("AB12::210"),
				0,
			},
		}

		for _, test := range testCases {
			result := Cmp(test.a, test.b)

			Expect(result).To(Equal(test.result))
		}
	})

	It("IsBroadcast", func() {
		_, testNet, _ := net.ParseCIDR("192.168.0.0/24")
		_, testNet6, _ := net.ParseCIDR("fd8d:45a0:3ceb:a19c::/64")
		testCases := []struct {
			ip      net.IP
			network *net.IPNet
			result  bool
		}{
			{
				nil,
				nil,
				false,
			},
			{
				net.ParseIP("192.168.0.10"),
				testNet,
				false,
			},
			{
				net.ParseIP("192.168.1.255"),
				testNet,
				false,
			},
			{
				net.ParseIP("192.168.1.255"),
				testNet6,
				false,
			},
			{
				net.ParseIP("fd8d:45a0:3ceb:a19c:ffff:ffff:ffff:ffff"),
				testNet6,
				false,
			},
			{
				net.ParseIP("192.168.0.255"),
				testNet,
				true,
			},
			{
				net.ParseIP("192.168.0.10"),
				func() *net.IPNet {
					_, testNet, _ := net.ParseCIDR("192.168.0.10/32")
					return testNet
				}(),
				false,
			},
			{
				net.ParseIP("192.168.0.1"),
				func() *net.IPNet {
					_, testNet, _ := net.ParseCIDR("192.168.0.0/31")
					return testNet
				}(),
				false,
			},
		}

		for _, test := range testCases {
			result := IsBroadcast(test.ip, test.network)

			Expect(result).To(Equal(test.result))
		}
	})

	Context("NewSubnetIterator", func() {
		It("rejects a prefix shorter than the network prefix", func() {
			_, network, err := net.ParseCIDR("192.168.0.0/16")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 8)
			Expect(err).To(HaveOccurred())
			Expect(iterator).To(BeNil())
		})
		It("rejects a prefix longer than the address width", func() {
			_, network, err := net.ParseCIDR("192.168.0.0/16")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 120)
			Expect(err).To(HaveOccurred())
			Expect(iterator).To(BeNil())
		})
		It("rejects invalid network input", func() {
			iterator, err := NewSubnetIterator(nil, 24)
			Expect(err).To(HaveOccurred())
			Expect(iterator).To(BeNil())

			iterator, err = NewSubnetIterator(&net.IPNet{
				IP:   net.ParseIP("192.168.0.0"),
				Mask: net.IPMask{0xff, 0x00, 0xff, 0x00},
			}, 24)
			Expect(err).To(HaveOccurred())
			Expect(iterator).To(BeNil())
		})
		It("rejects an IP address that does not match the mask family", func() {
			iterator, err := NewSubnetIterator(&net.IPNet{
				IP:   net.ParseIP("2001:db8::"),
				Mask: net.CIDRMask(24, 32),
			}, 24)
			Expect(err).To(HaveOccurred())
			Expect(iterator).To(BeNil())
		})
		It("Valid - single subnet IPv4", func() {
			_, network, err := net.ParseCIDR("192.168.0.0/24")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 24)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("192.168.0.0/24"))
			Expect(iterator.Next()).To(BeNil())
		})
		It("Valid - single subnet IPv6", func() {
			_, network, err := net.ParseCIDR("2002:0:0:1234::/64")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 64)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("2002:0:0:1234::/64"))
			Expect(iterator.Next()).To(BeNil())
		})
		It("valid - IPv4", func() {
			_, network, err := net.ParseCIDR("192.168.4.0/23")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 25)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("192.168.4.0/25"))
			Expect(iterator.Next().String()).To(Equal("192.168.4.128/25"))
			Expect(iterator.Next().String()).To(Equal("192.168.5.0/25"))
			Expect(iterator.Next().String()).To(Equal("192.168.5.128/25"))
			Expect(iterator.Next()).To(BeNil())
		})
		It("valid - IPv6", func() {
			_, network, err := net.ParseCIDR("2002:0:0:1234::/64")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 124)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("2002:0:0:1234::/124"))
			Expect(iterator.Next().String()).To(Equal("2002:0:0:1234::10/124"))
			Expect(iterator.Next().String()).To(Equal("2002:0:0:1234::20/124"))
		})
		It("valid - large IPv6 subnet (overflow test)", func() {
			_, network, err := net.ParseCIDR("::/0")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 127)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("::/127"))
			Expect(iterator.Next().String()).To(Equal("::2/127"))
			Expect(iterator.Next().String()).To(Equal("::4/127"))
		})
		It("valid - single IP IPv4 subnet", func() {
			_, network, err := net.ParseCIDR("192.168.0.0/16")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 32)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("192.168.0.0/32"))
			Expect(iterator.Next().String()).To(Equal("192.168.0.1/32"))
			Expect(iterator.Next().String()).To(Equal("192.168.0.2/32"))
		})
		It("valid - single IP IPv6 subnet", func() {
			_, network, err := net.ParseCIDR("2002:0:0:1234::/64")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 128)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("2002:0:0:1234::/128"))
			Expect(iterator.Next().String()).To(Equal("2002:0:0:1234::1/128"))
			Expect(iterator.Next().String()).To(Equal("2002:0:0:1234::2/128"))
		})
		It("valid - single IP IPv4 subnet, point to point network", func() {
			_, network, err := net.ParseCIDR("192.168.0.0/31")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 32)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("192.168.0.0/32"))
			Expect(iterator.Next().String()).To(Equal("192.168.0.1/32"))
			Expect(iterator.Next()).To(BeNil())
		})
	})
	Context("SubnetIterator.AdvancePastIP", func() {
		It("advances across an exhausted IPv6 /64 in constant time", func() {
			_, network, err := net.ParseCIDR("2001:db8::/64")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 128)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("2001:db8::/128"))
			Expect(iterator.AdvancePastIP(net.ParseIP("2001:db8::ffff:ffff:ffff:ffff"))).To(Succeed())
			Expect(iterator.Next()).To(BeNil())
		})

		It("reevaluates the prefix containing an exclusion boundary", func() {
			_, network, err := net.ParseCIDR("2001:db8::/120")
			Expect(err).NotTo(HaveOccurred())
			iterator, err := NewSubnetIterator(network, 124)
			Expect(err).NotTo(HaveOccurred())
			Expect(iterator.Next().String()).To(Equal("2001:db8::/124"))
			Expect(iterator.AdvancePastIP(net.ParseIP("2001:db8::24"))).To(Succeed())
			Expect(iterator.Next().String()).To(Equal("2001:db8::20/124"))
		})
	})
	Context("IsPointToPointSubnet", func() {
		It("/31", func() {
			_, network, _ := net.ParseCIDR("192.168.1.0/31")
			Expect(IsPointToPointSubnet(network)).To(BeTrue())
		})
		It("/127", func() {
			_, network, _ := net.ParseCIDR("2002:0:0:1234::1/127")
			Expect(IsPointToPointSubnet(network)).To(BeTrue())
		})
		It("/24", func() {
			_, network, _ := net.ParseCIDR("192.168.1.0/24")
			Expect(IsPointToPointSubnet(network)).To(BeFalse())
		})
	})
	Context("IsSingleIPSubnet", func() {
		It("/32", func() {
			_, network, _ := net.ParseCIDR("192.168.1.0/32")
			Expect(IsSingleIPSubnet(network)).To(BeTrue())
		})
		It("/128", func() {
			_, network, _ := net.ParseCIDR("2002:0:0:1234::1/128")
			Expect(IsSingleIPSubnet(network)).To(BeTrue())
		})
		It("/24", func() {
			_, network, _ := net.ParseCIDR("192.168.1.0/24")
			Expect(IsSingleIPSubnet(network)).To(BeFalse())
		})
		It("/31", func() {
			_, network, _ := net.ParseCIDR("192.168.1.0/31")
			Expect(IsSingleIPSubnet(network)).To(BeFalse())
		})
	})
	Context("LastIP", func() {
		It("/31", func() {
			_, network, _ := net.ParseCIDR("192.168.1.0/31")
			Expect(LastIP(network).String()).To(Equal("192.168.1.1"))
		})
		It("/127", func() {
			_, network, _ := net.ParseCIDR("2002:0:0:1234::0/127")
			Expect(LastIP(network).String()).To(Equal("2002:0:0:1234::1"))
		})
		It("/32", func() {
			_, network, _ := net.ParseCIDR("192.168.1.10/32")
			Expect(LastIP(network).String()).To(Equal("192.168.1.10"))
		})
		It("/128", func() {
			_, network, _ := net.ParseCIDR("2002:0:0:1234::10/128")
			Expect(LastIP(network).String()).To(Equal("2002:0:0:1234::10"))
		})
		It("/24", func() {
			_, network, _ := net.ParseCIDR("192.168.1.0/24")
			Expect(LastIP(network).String()).To(Equal("192.168.1.254"))
		})
		It("/64", func() {
			_, network, _ := net.ParseCIDR("2002:0:0:1234::0/64")
			Expect(LastIP(network).String()).To(Equal("2002::1234:ffff:ffff:ffff:ffff"))
		})
	})
})
