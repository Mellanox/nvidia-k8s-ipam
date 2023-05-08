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

package allocator_test

import (
	"context"
	"errors"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/allocator"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

const (
	testNodeName1          = "node1"
	testNodeName2          = "node2"
	testNodeName3          = "node3"
	testNodeName4          = "node4"
	testPoolName1          = "pool1"
	testPoolName2          = "pool2"
	testPerNodeBlockCount1 = 15
	testPerNodeBlockCount2 = 10
)

func getPool1Config() allocator.AllocationConfig {
	_, network, _ := net.ParseCIDR("192.168.0.0/24")
	return allocator.AllocationConfig{
		PoolName:         testPoolName1,
		Subnet:           network,
		Gateway:          net.ParseIP("192.168.0.1"),
		PerNodeBlockSize: testPerNodeBlockCount1,
	}
}

func getPool2Config() allocator.AllocationConfig {
	_, network, _ := net.ParseCIDR("172.16.0.0/16")
	return allocator.AllocationConfig{
		PoolName:         testPoolName2,
		Subnet:           network,
		Gateway:          net.ParseIP("172.16.0.1"),
		PerNodeBlockSize: testPerNodeBlockCount2,
	}
}

var _ = Describe("Allocator", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Allocated/Deallocate without config", func() {
		a := allocator.New()
		alloc, err := a.Allocate(ctx, testNodeName1)
		Expect(alloc).To(BeEmpty())
		Expect(err).To(BeNil())
		a.Deallocate(ctx, testNodeName1)
	})

	It("Allocate/Deallocate", func() {
		pool1 := getPool1Config()
		pool2 := getPool2Config()
		a := allocator.New()
		a.Configure(ctx, []allocator.AllocationConfig{pool1, pool2})
		node1Alloc, err := a.Allocate(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(node1Alloc[testPoolName1].StartIP).To(BeEquivalentTo("192.168.0.1"))
		Expect(node1Alloc[testPoolName1].EndIP).To(BeEquivalentTo("192.168.0.15"))
		Expect(node1Alloc[testPoolName2].StartIP).To(BeEquivalentTo("172.16.0.1"))
		Expect(node1Alloc[testPoolName2].EndIP).To(BeEquivalentTo("172.16.0.10"))

		node1AllocSecondCall, err := a.Allocate(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(node1AllocSecondCall).To(Equal(node1Alloc))

		node2Alloc, err := a.Allocate(ctx, testNodeName2)
		Expect(err).NotTo(HaveOccurred())
		Expect(node2Alloc[testPoolName1].StartIP).To(BeEquivalentTo("192.168.0.16"))
		Expect(node2Alloc[testPoolName1].EndIP).To(BeEquivalentTo("192.168.0.30"))
		Expect(node2Alloc[testPoolName2].StartIP).To(BeEquivalentTo("172.16.0.11"))
		Expect(node2Alloc[testPoolName2].EndIP).To(BeEquivalentTo("172.16.0.20"))

		node3Alloc, err := a.Allocate(ctx, testNodeName3)
		Expect(node3Alloc[testPoolName1].StartIP).To(BeEquivalentTo("192.168.0.31"))
		Expect(node3Alloc[testPoolName1].EndIP).To(BeEquivalentTo("192.168.0.45"))
		Expect(node3Alloc[testPoolName2].StartIP).To(BeEquivalentTo("172.16.0.21"))
		Expect(node3Alloc[testPoolName2].EndIP).To(BeEquivalentTo("172.16.0.30"))

		node4Alloc, err := a.Allocate(ctx, testNodeName4)
		Expect(node4Alloc[testPoolName1].StartIP).To(BeEquivalentTo("192.168.0.46"))
		Expect(node4Alloc[testPoolName1].EndIP).To(BeEquivalentTo("192.168.0.60"))
		Expect(node4Alloc[testPoolName2].StartIP).To(BeEquivalentTo("172.16.0.31"))
		Expect(node4Alloc[testPoolName2].EndIP).To(BeEquivalentTo("172.16.0.40"))

		// deallocate for node3 and node1
		a.Deallocate(ctx, testNodeName1)
		a.Deallocate(ctx, testNodeName3)

		// allocate again, testNodeName3 should have IPs from index 0, testNodeName3 IPs from index 2
		node3Alloc, err = a.Allocate(ctx, testNodeName3)
		Expect(node3Alloc[testPoolName1].StartIP).To(BeEquivalentTo("192.168.0.1"))
		Expect(node3Alloc[testPoolName1].EndIP).To(BeEquivalentTo("192.168.0.15"))
		Expect(node3Alloc[testPoolName2].StartIP).To(BeEquivalentTo("172.16.0.1"))
		Expect(node3Alloc[testPoolName2].EndIP).To(BeEquivalentTo("172.16.0.10"))

		node1Alloc, err = a.Allocate(ctx, testNodeName1)
		Expect(node1Alloc[testPoolName1].StartIP).To(BeEquivalentTo("192.168.0.31"))
		Expect(node1Alloc[testPoolName1].EndIP).To(BeEquivalentTo("192.168.0.45"))
		Expect(node1Alloc[testPoolName2].StartIP).To(BeEquivalentTo("172.16.0.21"))
		Expect(node1Alloc[testPoolName2].EndIP).To(BeEquivalentTo("172.16.0.30"))
	})

	It("No free ranges", func() {
		pool1 := getPool1Config()
		// pool is /24, must fail on the second allocation
		pool1.PerNodeBlockSize = 200
		a := allocator.New()
		a.Configure(ctx, []allocator.AllocationConfig{pool1})
		node1Alloc, err := a.Allocate(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(node1Alloc).To(HaveLen(1))

		_, err = a.Allocate(ctx, testNodeName2)
		Expect(errors.Is(err, allocator.ErrNoFreeRanges)).To(BeTrue())
	})
	It("return NoFreeRanges in case if IP is too large", func() {
		_, subnet, _ := net.ParseCIDR("255.255.255.0/24")
		a := allocator.New()
		a.Configure(ctx, []allocator.AllocationConfig{{
			PoolName:         "pool",
			Subnet:           subnet,
			Gateway:          net.ParseIP("255.255.255.1"),
			PerNodeBlockSize: 200}})
		_, err := a.Allocate(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		_, err = a.Allocate(ctx, testNodeName2)
		Expect(errors.Is(err, allocator.ErrNoFreeRanges)).To(BeTrue())
	})

	It("Configure - reset allocations", func() {
		a := allocator.New()
		origConfig := getPool1Config()
		a.Configure(ctx, []allocator.AllocationConfig{origConfig})
		_, err := a.Allocate(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		node2Alloc, err := a.Allocate(ctx, testNodeName2)
		Expect(err).NotTo(HaveOccurred())

		// update config with same configuration, should not reset allocations
		a.Configure(ctx, []allocator.AllocationConfig{origConfig})
		node2AllocSecondCall, err := a.Allocate(ctx, testNodeName2)
		Expect(err).NotTo(HaveOccurred())
		Expect(node2AllocSecondCall[testPoolName1].StartIP).To(Equal(node2Alloc[testPoolName1].StartIP))

		// reset config
		newCfg := origConfig
		newCfg.Gateway = net.ParseIP("192.168.0.2")
		a.Configure(ctx, []allocator.AllocationConfig{newCfg})
		node2AllocThirdCall, err := a.Allocate(ctx, testNodeName2)
		Expect(err).NotTo(HaveOccurred())
		// allocation begins from the start of the subnet
		Expect(node2AllocThirdCall[testPoolName1].StartIP).NotTo(Equal(node2Alloc[testPoolName1].StartIP))
	})

	It("ConfigureAndLoadAllocations - Data load test", func() {
		getValidData := func() *pool.IPPool {
			return &pool.IPPool{Name: testPoolName1,
				Subnet:  "192.168.0.0/24",
				StartIP: "192.168.0.16",
				EndIP:   "192.168.0.30",
				Gateway: "192.168.0.1",
			}
		}

		testCases := []struct {
			in     *pool.IPPool
			loaded bool
		}{
			{ // valid data
				in:     getValidData(),
				loaded: true,
			},
			{ // different subnet, should ignore
				in: &pool.IPPool{Name: testPoolName1,
					Subnet:  "1.1.1.0/24",
					StartIP: "1.1.1.1",
					EndIP:   "1.1.1.2",
					Gateway: "1.1.1.1",
				},
				loaded: false,
			},
			{ // no subnet, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.Subnet = ""
					return d
				}(),
				loaded: false,
			},
			{ // no gw, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.Gateway = ""
					return d
				}(),
				loaded: false,
			},
			{ // no startIP, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.StartIP = ""
					return d
				}(),
				loaded: false,
			},
			{ // no endIP, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.EndIP = ""
					return d
				}(),
				loaded: false,
			},
			{ // start and end IPs are the same, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.StartIP = "192.168.0.1"
					d.EndIP = "192.168.0.1"
					return d
				}(),
				loaded: false,
			},
			{ // IPs out of subnet, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.StartIP = "192.168.1.1"
					d.EndIP = "192.168.1.15"
					return d
				}(),
				loaded: false,
			},
			{ // duplicate range, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.StartIP = "192.168.0.1"
					d.EndIP = "192.168.0.15"
					return d
				}(),
				loaded: false,
			},
			{ // ip invalid offset, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.StartIP = "192.168.0.17"
					d.EndIP = "192.168.0.31"
					return d
				}(),
				loaded: false,
			},
			{ // bad IP count, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.StartIP = "192.168.0.16"
					d.EndIP = "192.168.0.25"
					return d
				}(),
				loaded: false,
			},
			{ // different GW, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.Gateway = "192.168.0.2"
					return d
				}(),
				loaded: false,
			},
			{ // wrong GW, should ignore
				in: func() *pool.IPPool {
					d := getValidData()
					d.Gateway = "192.168.100.1"
					return d
				}(),
				loaded: false,
			},
		}
		for _, test := range testCases {
			a := allocator.New()
			Expect(a.IsConfigured()).To(BeFalse())

			pool1 := getPool1Config()

			defNode := corev1.Node{}
			defNode.SetName(testPoolName1)
			defNodeAlloc := map[string]*pool.IPPool{
				testPoolName1: {Name: testPoolName1,
					Subnet:  "192.168.0.0/24",
					StartIP: "192.168.0.1",
					EndIP:   "192.168.0.15",
					Gateway: "192.168.0.1",
				},
			}
			Expect(pool.SetIPBlockAnnotation(&defNode, defNodeAlloc)).NotTo(HaveOccurred())

			testNodeAlloc := map[string]*pool.IPPool{
				testPoolName1: test.in,
			}
			testNode := corev1.Node{}
			testNode.SetName(testNodeName2)
			Expect(pool.SetIPBlockAnnotation(&testNode, testNodeAlloc)).NotTo(HaveOccurred())

			a.ConfigureAndLoadAllocations(ctx, []allocator.AllocationConfig{pool1}, []corev1.Node{defNode, {}, testNode})
			Expect(a.IsConfigured()).To(BeTrue())
			node1AllocFromAllocator, err := a.Allocate(ctx, testNodeName2)
			Expect(err).NotTo(HaveOccurred())
			if test.loaded {
				Expect(node1AllocFromAllocator).To(Equal(testNodeAlloc))
			} else {
				Expect(node1AllocFromAllocator).NotTo(Equal(testNodeAlloc))
			}
		}
	})
})
