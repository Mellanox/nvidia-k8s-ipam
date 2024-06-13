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
			ip     net.IP
			offset int64
			nextIP net.IP
		}{
			{
				[]byte{192, 0, 2},
				10,
				nil,
			},
			{
				net.ParseIP("192.168.0.1"),
				10,
				net.IPv4(192, 168, 0, 11).To4(),
			},
			{
				net.ParseIP("192.168.0.254"),
				10,
				net.IPv4(192, 168, 1, 8).To4(),
			},
			{
				net.ParseIP("192.168.0.254"),
				-10,
				nil,
			},
			{
				net.ParseIP("0::123"),
				3,
				net.ParseIP("0::126"),
			},
			{
				net.ParseIP("AB12::FFFF"),
				3,
				net.ParseIP("AB12::1:2"),
			},
		}

		for _, test := range testCases {
			ip := NextIPWithOffset(test.ip, test.offset)

			Expect(ip).To(Equal(test.nextIP))
		}
	})

	It("Distance", func() {
		testCases := []struct {
			ipA   net.IP
			ipB   net.IP
			count int64
		}{
			{
				net.ParseIP("192.168.0.1"),
				net.ParseIP("192.168.0.11"),
				10,
			},
			{
				net.ParseIP("192.168.0.2"),
				net.ParseIP("192.168.0.2"),
				0,
			},
			{
				net.ParseIP("AB12::FFFF"),
				net.ParseIP("AB12::1:2"),
				3,
			},
			{
				net.ParseIP("192.168.0.11"),
				net.ParseIP("192.168.0.1"),
				-1,
			},
			{
				net.ParseIP("192.168.0.11"),
				[]byte{192, 0, 2},
				-2,
			},
			{
				net.ParseIP("192.168.0.11"),
				net.ParseIP("AB12::FFFF"),
				-2,
			},
		}

		for _, test := range testCases {
			ip := Distance(test.ipA, test.ipB)

			Expect(ip).To(Equal(test.count))
		}
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
		}

		for _, test := range testCases {
			result := IsBroadcast(test.ip, test.network)

			Expect(result).To(Equal(test.result))
		}
	})

	Context("GetSubnetGen", func() {
		It("Invalid args - prefix is larger then network", func() {
			_, net, _ := net.ParseCIDR("192.168.0.0/16")
			gen := GetSubnetGen(net, 8)
			Expect(gen).NotTo(BeNil())
			Expect(gen()).To(BeNil())
		})
		It("Invalid args - prefix is too small for IPv4", func() {
			_, net, _ := net.ParseCIDR("192.168.0.0/16")
			gen := GetSubnetGen(net, 120)
			Expect(gen).NotTo(BeNil())
			Expect(gen()).To(BeNil())
		})
		It("Valid - single subnet IPv4", func() {
			_, net, _ := net.ParseCIDR("192.168.0.0/24")
			gen := GetSubnetGen(net, 24)
			Expect(gen).NotTo(BeNil())
			Expect(gen().String()).To(Equal("192.168.0.0/24"))
			Expect(gen()).To(BeNil())
		})
		It("Valid - single subnet IPv6", func() {
			_, net, _ := net.ParseCIDR("2002:0:0:1234::/64")
			gen := GetSubnetGen(net, 64)
			Expect(gen).NotTo(BeNil())
			Expect(gen().String()).To(Equal("2002:0:0:1234::/64"))
			Expect(gen()).To(BeNil())
		})
		It("valid - IPv4", func() {
			_, net, _ := net.ParseCIDR("192.168.4.0/23")
			gen := GetSubnetGen(net, 25)
			Expect(gen).NotTo(BeNil())
			Expect(gen().String()).To(Equal("192.168.4.0/25"))
			Expect(gen().String()).To(Equal("192.168.4.128/25"))
			Expect(gen().String()).To(Equal("192.168.5.0/25"))
			Expect(gen().String()).To(Equal("192.168.5.128/25"))
			Expect(gen()).To(BeNil())
		})
		It("valid - IPv6", func() {
			_, net, _ := net.ParseCIDR("2002:0:0:1234::/64")
			gen := GetSubnetGen(net, 124)
			Expect(gen).NotTo(BeNil())
			Expect(gen().String()).To(Equal("2002:0:0:1234::/124"))
			Expect(gen().String()).To(Equal("2002:0:0:1234::10/124"))
			Expect(gen().String()).To(Equal("2002:0:0:1234::20/124"))
		})
		It("valid - large IPv6 subnet (overflow test)", func() {
			_, net, _ := net.ParseCIDR("::/0")
			gen := GetSubnetGen(net, 127)
			Expect(gen).NotTo(BeNil())
			Expect(gen().String()).To(Equal("::/127"))
			Expect(gen().String()).To(Equal("::2/127"))
			Expect(gen().String()).To(Equal("::4/127"))
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
	Context("LastIP", func() {
		It("/31", func() {
			_, network, _ := net.ParseCIDR("192.168.1.0/31")
			Expect(LastIP(network).String()).To(Equal("192.168.1.1"))
		})
		It("/127", func() {
			_, network, _ := net.ParseCIDR("2002:0:0:1234::0/127")
			Expect(LastIP(network).String()).To(Equal("2002:0:0:1234::1"))
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
