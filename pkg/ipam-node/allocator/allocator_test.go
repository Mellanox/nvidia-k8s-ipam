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
	"context"
	"fmt"
	"net"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cniTypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

const (
	testPoolName    = "test"
	testIFName      = "eth0"
	testContainerID = "ID"
)

type AllocatorTestCase struct {
	subnets      []string
	ipMap        map[string]string
	expectResult string
	lastIP       string
}

func mkAlloc(session storePkg.Session) allocator.IPAllocator {
	p := allocator.RangeSet{
		allocator.Range{Subnet: mustSubnet("192.168.1.0/29"), Gateway: net.ParseIP("192.168.1.1")},
	}
	Expect(p.Canonicalize()).NotTo(HaveOccurred())
	return allocator.NewIPAllocator(&p, nil, testPoolName, session)
}

func newAllocatorWithMultiRanges(session storePkg.Session) allocator.IPAllocator {
	p := allocator.RangeSet{
		allocator.Range{RangeStart: net.IP{192, 168, 1, 0}, RangeEnd: net.IP{192, 168, 1, 3}, Subnet: mustSubnet("192.168.1.0/30")},
		allocator.Range{RangeStart: net.IP{192, 168, 2, 0}, RangeEnd: net.IP{192, 168, 2, 3}, Subnet: mustSubnet("192.168.2.0/30")},
	}
	Expect(p.Canonicalize()).NotTo(HaveOccurred())
	return allocator.NewIPAllocator(&p, nil, testPoolName, session)
}

func (t AllocatorTestCase) run(idx int, session storePkg.Session) (*current.IPConfig, error) {
	_, _ = fmt.Fprintln(GinkgoWriter, "Index:", idx)
	p := allocator.RangeSet{}
	for _, s := range t.subnets {
		subnet, err := cniTypes.ParseCIDR(s)
		if err != nil {
			return nil, err
		}
		p = append(p, allocator.Range{Subnet: cniTypes.IPNet(*subnet), Gateway: ip.NextIP(subnet.IP)})
	}
	for r := range t.ipMap {
		Expect(session.Reserve(testPoolName, r, "net1",
			types.ReservationMetadata{}, net.ParseIP(r))).NotTo(HaveOccurred())
	}
	session.SetLastReservedIP(testPoolName, net.ParseIP(t.lastIP))

	Expect(p.Canonicalize()).To(Succeed())
	alloc := allocator.NewIPAllocator(&p, nil, testPoolName, session)
	return alloc.Allocate(testContainerID, testIFName, types.ReservationMetadata{}, nil)
}

func checkAlloc(a allocator.IPAllocator, id string, expectedIP net.IP) {
	cfg, err := a.Allocate(id, testIFName, types.ReservationMetadata{}, nil)
	if expectedIP == nil {
		ExpectWithOffset(1, err).To(HaveOccurred())
		return
	}
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, cfg.Address.IP).To(Equal(expectedIP))
}

var _ = Describe("allocator", func() {
	Context("RangeIter", func() {
		It("should loop correctly from the beginning", func() {
			store, err := storePkg.New(filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = store.Commit()
			}()
			a := mkAlloc(store)
			checkAlloc(a, "1", net.IP{192, 168, 1, 2})
			checkAlloc(a, "2", net.IP{192, 168, 1, 3})
			checkAlloc(a, "3", net.IP{192, 168, 1, 4})
			checkAlloc(a, "4", net.IP{192, 168, 1, 5})
			checkAlloc(a, "5", net.IP{192, 168, 1, 6})
			checkAlloc(a, "6", nil)
		})

		It("should loop correctly from the end", func() {
			store, err := storePkg.New(filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = store.Commit()
			}()
			a := mkAlloc(store)
			Expect(store.Reserve(testPoolName, testContainerID, testIFName, types.ReservationMetadata{}, net.IP{192, 168, 1, 6})).
				NotTo(HaveOccurred())
			checkAlloc(a, "1", net.IP{192, 168, 1, 2})
			checkAlloc(a, "2", net.IP{192, 168, 1, 3})
		})
		It("should loop correctly from the middle", func() {
			store, err := storePkg.New(filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = store.Commit()
			}()
			a := mkAlloc(store)
			Expect(store.Reserve(testPoolName, testContainerID, testIFName, types.ReservationMetadata{}, net.IP{192, 168, 1, 3})).
				NotTo(HaveOccurred())
			store.ReleaseReservationByID(testPoolName, testContainerID, testIFName)
			checkAlloc(a, "0", net.IP{192, 168, 1, 4})
			checkAlloc(a, "1", net.IP{192, 168, 1, 5})
			checkAlloc(a, "2", net.IP{192, 168, 1, 6})
			checkAlloc(a, "3", net.IP{192, 168, 1, 2})
			checkAlloc(a, "4", net.IP{192, 168, 1, 3})
			checkAlloc(a, "5", nil)
		})
	})

	Context("when has free ip", func() {
		It("should allocate ips in round robin", func() {
			testCases := []AllocatorTestCase{
				// fresh start
				{
					subnets:      []string{"10.0.0.0/29"},
					ipMap:        map[string]string{},
					expectResult: "10.0.0.2",
					lastIP:       "",
				},
				{
					subnets:      []string{"2001:db8:1::0/64"},
					ipMap:        map[string]string{},
					expectResult: "2001:db8:1::2",
					lastIP:       "",
				},
				{
					subnets:      []string{"10.0.0.0/30"},
					ipMap:        map[string]string{},
					expectResult: "10.0.0.2",
					lastIP:       "",
				},
				{
					subnets: []string{"10.0.0.0/29"},
					ipMap: map[string]string{
						"10.0.0.2": testContainerID,
					},
					expectResult: "10.0.0.3",
					lastIP:       "",
				},
				// next ip of last reserved ip
				{
					subnets:      []string{"10.0.0.0/29"},
					ipMap:        map[string]string{},
					expectResult: "10.0.0.6",
					lastIP:       "10.0.0.5",
				},
				{
					subnets: []string{"10.0.0.0/29"},
					ipMap: map[string]string{
						"10.0.0.4": testContainerID,
						"10.0.0.5": testContainerID,
					},
					expectResult: "10.0.0.6",
					lastIP:       "10.0.0.3",
				},
				// round-robin to the beginning
				{
					subnets: []string{"10.0.0.0/29"},
					ipMap: map[string]string{
						"10.0.0.6": testContainerID,
					},
					expectResult: "10.0.0.2",
					lastIP:       "10.0.0.5",
				},
				// lastIP is out of range
				{
					subnets: []string{"10.0.0.0/29"},
					ipMap: map[string]string{
						"10.0.0.2": testContainerID,
					},
					expectResult: "10.0.0.3",
					lastIP:       "10.0.0.128",
				},
				// subnet is completely full except for lastip
				// wrap around and reserve lastIP
				{
					subnets: []string{"10.0.0.0/29"},
					ipMap: map[string]string{
						"10.0.0.2": testContainerID,
						"10.0.0.4": testContainerID,
						"10.0.0.5": testContainerID,
						"10.0.0.6": testContainerID,
					},
					expectResult: "10.0.0.3",
					lastIP:       "10.0.0.3",
				},
				// allocate from multiple subnets
				{
					subnets:      []string{"10.0.0.0/30", "10.0.1.0/30"},
					expectResult: "10.0.0.2",
					ipMap:        map[string]string{},
				},
				// advance to next subnet
				{
					subnets:      []string{"10.0.0.0/30", "10.0.1.0/30"},
					lastIP:       "10.0.0.2",
					expectResult: "10.0.1.2",
					ipMap:        map[string]string{},
				},
				// Roll to start subnet
				{
					subnets:      []string{"10.0.0.0/30", "10.0.1.0/30", "10.0.2.0/30"},
					lastIP:       "10.0.2.2",
					expectResult: "10.0.0.2",
					ipMap:        map[string]string{},
				},
				// Already allocated
				{
					subnets:      []string{"10.0.2.0/30"},
					lastIP:       "10.33.33.1",
					expectResult: "10.0.2.2",
					ipMap: map[string]string{
						"10.0.2.1": testContainerID,
					},
				},
				// IP overflow
				{
					subnets: []string{"255.255.255.0/24"},
					lastIP:  "255.255.255.255",
					// skip GW ip
					expectResult: "255.255.255.2",
					ipMap:        map[string]string{},
				},
			}

			for idx, tc := range testCases {
				store, err := storePkg.New(
					filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
				Expect(err).NotTo(HaveOccurred())
				res, err := tc.run(idx, store)
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Address.IP.String()).To(Equal(tc.expectResult))
				Expect(store.Commit()).NotTo(HaveOccurred())
			}
		})

		It("should not allocate the broadcast address", func() {
			store, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			defer func() {
				_ = store.Commit()
			}()
			alloc := mkAlloc(store)
			for i := 2; i < 7; i++ {
				res, err := alloc.Allocate(fmt.Sprintf("ID%d", i), testIFName, types.ReservationMetadata{}, nil)
				Expect(err).ToNot(HaveOccurred())
				s := fmt.Sprintf("192.168.1.%d/29", i)
				Expect(s).To(Equal(res.Address.String()))
				_, _ = fmt.Fprintln(GinkgoWriter, "got ip", res.Address.String())
			}

			x, err := alloc.Allocate("ID8", testIFName, types.ReservationMetadata{}, nil)
			_, _ = fmt.Fprintln(GinkgoWriter, "got ip", x)
			Expect(err).To(HaveOccurred())
		})

		It("should allocate in a round-robin fashion", func() {
			store, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			defer func() {
				_ = store.Commit()
			}()
			alloc := mkAlloc(store)
			res, err := alloc.Allocate(testContainerID, testIFName, types.ReservationMetadata{}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Address.String()).To(Equal("192.168.1.2/29"))

			store.ReleaseReservationByID(testPoolName, testContainerID, testIFName)

			res, err = alloc.Allocate(testContainerID, testIFName, types.ReservationMetadata{}, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Address.String()).To(Equal("192.168.1.3/29"))
		})
	})
	Context("when out of ips", func() {
		It("returns a meaningful error", func() {
			testCases := []AllocatorTestCase{
				{
					subnets: []string{"10.0.0.0/30"},
					ipMap: map[string]string{
						"10.0.0.2": testContainerID,
					},
				},
				{
					subnets: []string{"10.0.0.0/29"},
					ipMap: map[string]string{
						"10.0.0.2": testContainerID,
						"10.0.0.3": testContainerID,
						"10.0.0.4": testContainerID,
						"10.0.0.5": testContainerID,
						"10.0.0.6": testContainerID,
					},
				},
				{
					subnets: []string{"10.0.0.0/30", "10.0.1.0/30"},
					ipMap: map[string]string{
						"10.0.0.2": testContainerID,
						"10.0.1.2": testContainerID,
					},
				},
			}
			for idx, tc := range testCases {
				store, err := storePkg.New(
					filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
				Expect(err).NotTo(HaveOccurred())
				_, err = tc.run(idx, store)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(HavePrefix("no free addresses in the allocated range"))
				Expect(store.Commit()).NotTo(HaveOccurred())
			}
		})
	})

	Context("when lastReservedIP is at the end of one of multi ranges", func() {
		It("should use the first IP of next range as startIP after Next", func() {
			store, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = store.Commit()
			}()
			a := newAllocatorWithMultiRanges(store)

			// reserve the last IP of the first range
			err = store.Reserve(testPoolName, testContainerID, testIFName, types.ReservationMetadata{}, net.IP{192, 168, 1, 3})
			Expect(err).NotTo(HaveOccurred())

			// check that IP from the next range is used
			checkAlloc(a, "0", net.IP{192, 168, 2, 0})
		})
	})

	Context("when no lastReservedIP", func() {
		It("should use the first IP of the first range as startIP after Next", func() {
			store, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = store.Commit()
			}()
			a := newAllocatorWithMultiRanges(store)

			// get range iterator and do the first Next
			checkAlloc(a, "0", net.IP{192, 168, 1, 0})
		})
	})
	Context("no gateway", func() {
		It("should use the first IP of the range", func() {
			session, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = session.Commit()
			}()
			p := allocator.RangeSet{
				allocator.Range{Subnet: mustSubnet("192.168.1.0/30")},
				allocator.Range{Subnet: mustSubnet("192.168.1.4/30")},
			}
			Expect(p.Canonicalize()).NotTo(HaveOccurred())
			a := allocator.NewIPAllocator(&p, nil, testPoolName, session)
			// get range iterator and do the first Next
			checkAlloc(a, "0", net.IP{192, 168, 1, 1})
			checkAlloc(a, "1", net.IP{192, 168, 1, 2})
			checkAlloc(a, "2", net.IP{192, 168, 1, 5})
		})
	})
	Context("point to point ranges", func() {
		It("should allocate two IPs", func() {
			session, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = session.Commit()
			}()
			p := allocator.RangeSet{
				allocator.Range{Subnet: mustSubnet("192.168.1.0/31")},
			}
			Expect(p.Canonicalize()).NotTo(HaveOccurred())
			a := allocator.NewIPAllocator(&p, nil, testPoolName, session)
			// get range iterator and do the first Next
			checkAlloc(a, "0", net.IP{192, 168, 1, 0})
			checkAlloc(a, "1", net.IP{192, 168, 1, 1})
		})
	})
	Context("IP address exclusion", func() {
		It("should exclude IPs", func() {
			session, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = session.Commit()
			}()
			p := allocator.RangeSet{
				allocator.Range{Subnet: mustSubnet("192.168.0.0/29")},
			}
			Expect(p.Canonicalize()).NotTo(HaveOccurred())
			e := allocator.RangeSet{
				allocator.Range{Subnet: mustSubnet("192.168.0.0/29"),
					RangeStart: net.ParseIP("192.168.0.2"), RangeEnd: net.ParseIP("192.168.0.3")},
				allocator.Range{Subnet: mustSubnet("192.168.0.0/29"),
					RangeStart: net.ParseIP("192.168.0.5"), RangeEnd: net.ParseIP("192.168.0.5")},
			}
			Expect(e.Canonicalize()).NotTo(HaveOccurred())
			a := allocator.NewIPAllocator(&p, &e, testPoolName, session)
			// get range iterator and do the first Next
			checkAlloc(a, "0", net.IP{192, 168, 0, 1})
			checkAlloc(a, "1", net.IP{192, 168, 0, 4})
			checkAlloc(a, "2", net.IP{192, 168, 0, 6})
		})
	})
	Context("Static IP allocation", func() {
		It("should allocate IP", func() {
			session, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = session.Commit()
			}()
			p := allocator.RangeSet{
				allocator.Range{Subnet: mustSubnet("192.168.0.0/24"), Gateway: net.ParseIP("192.168.0.10")},
			}
			// should ignore exclusions while allocating static IPs
			e := allocator.RangeSet{
				allocator.Range{Subnet: mustSubnet("192.168.0.0/24"),
					RangeStart: net.ParseIP("192.168.0.33"), RangeEnd: net.ParseIP("192.168.0.33")},
			}
			staticIP := net.ParseIP("192.168.0.33")
			Expect(p.Canonicalize()).NotTo(HaveOccurred())
			Expect(e.Canonicalize()).NotTo(HaveOccurred())
			a := allocator.NewIPAllocator(&p, &e, testPoolName, session)
			cfg, err := a.Allocate("0", testIFName, types.ReservationMetadata{}, staticIP)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Address.IP).To(Equal(staticIP))
			Expect(cfg.Address.Mask).To(Equal(p[0].Subnet.Mask))
			Expect(cfg.Gateway).To(Equal(p[0].Gateway))
			_, err = a.Allocate("1", testIFName, types.ReservationMetadata{}, staticIP)
			Expect(err).To(MatchError(ContainSubstring("ip address is already reserved")))
		})
		It("should fail if static IP is not in the allocator's subnet", func() {
			session, err := storePkg.New(
				filepath.Join(GinkgoT().TempDir(), "test_store")).Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				_ = session.Commit()
			}()
			p := allocator.RangeSet{
				allocator.Range{Subnet: mustSubnet("192.168.0.0/24"), Gateway: net.ParseIP("192.168.0.10")},
			}
			staticIP := net.ParseIP("10.10.10.10")
			Expect(p.Canonicalize()).NotTo(HaveOccurred())
			a := allocator.NewIPAllocator(&p, nil, testPoolName, session)
			_, err = a.Allocate("0", testIFName, types.ReservationMetadata{}, staticIP)
			Expect(err).To(MatchError(ContainSubstring("can't find IP range")))
		})
	})
})
