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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/allocator"
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

func getPool1() *ipamv1alpha1.IPPool {
	return &ipamv1alpha1.IPPool{
		ObjectMeta: v1.ObjectMeta{Name: testPoolName1},
		Spec: ipamv1alpha1.IPPoolSpec{
			Subnet:           "192.168.0.0/24",
			PerNodeBlockSize: testPerNodeBlockCount1,
			Gateway:          "192.168.0.1"},
	}
}

func getPool2() *ipamv1alpha1.IPPool {
	return &ipamv1alpha1.IPPool{
		ObjectMeta: v1.ObjectMeta{Name: testPoolName2},
		Spec: ipamv1alpha1.IPPoolSpec{
			Subnet:           "172.16.0.0/16",
			PerNodeBlockSize: testPerNodeBlockCount2,
			Gateway:          "172.16.0.1"},
	}
}

var _ = Describe("Allocator", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Allocate/Deallocate", func() {
		pool1 := getPool1()
		pool2 := getPool2()
		pa1 := allocator.CreatePoolAllocatorFromIPPool(ctx, pool1, sets.New[string]())
		pa2 := allocator.CreatePoolAllocatorFromIPPool(ctx, pool2, sets.New[string]())
		node1AllocPool1, err := pa1.AllocateFromPool(ctx, testNodeName1)
		Expect(err).ToNot(HaveOccurred())
		node1AllocPool2, err := pa2.AllocateFromPool(ctx, testNodeName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(node1AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.1"))
		Expect(node1AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.15"))
		Expect(node1AllocPool2.StartIP.String()).To(BeEquivalentTo("172.16.0.1"))
		Expect(node1AllocPool2.EndIP.String()).To(BeEquivalentTo("172.16.0.10"))

		node1AllocSecondCall, err := pa1.AllocateFromPool(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(node1AllocSecondCall).To(Equal(node1AllocPool1))

		node1AllocSecondCall, err = pa2.AllocateFromPool(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(node1AllocSecondCall).To(Equal(node1AllocPool2))

		node2AllocPool1, err := pa1.AllocateFromPool(ctx, testNodeName2)
		Expect(err).NotTo(HaveOccurred())
		node2AllocPool2, err := pa2.AllocateFromPool(ctx, testNodeName2)
		Expect(err).NotTo(HaveOccurred())
		Expect(node2AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.16"))
		Expect(node2AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.30"))
		Expect(node2AllocPool2.StartIP.String()).To(BeEquivalentTo("172.16.0.11"))
		Expect(node2AllocPool2.EndIP.String()).To(BeEquivalentTo("172.16.0.20"))

		node3AllocPool1, err := pa1.AllocateFromPool(ctx, testNodeName3)
		Expect(err).NotTo(HaveOccurred())
		node3AllocPool2, err := pa2.AllocateFromPool(ctx, testNodeName3)
		Expect(err).NotTo(HaveOccurred())
		Expect(node3AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.31"))
		Expect(node3AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.45"))
		Expect(node3AllocPool2.StartIP.String()).To(BeEquivalentTo("172.16.0.21"))
		Expect(node3AllocPool2.EndIP.String()).To(BeEquivalentTo("172.16.0.30"))

		node4AllocPool1, err := pa1.AllocateFromPool(ctx, testNodeName4)
		Expect(err).NotTo(HaveOccurred())
		node4AllocPool2, err := pa2.AllocateFromPool(ctx, testNodeName4)
		Expect(err).NotTo(HaveOccurred())
		Expect(node4AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.46"))
		Expect(node4AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.60"))
		Expect(node4AllocPool2.StartIP.String()).To(BeEquivalentTo("172.16.0.31"))
		Expect(node4AllocPool2.EndIP.String()).To(BeEquivalentTo("172.16.0.40"))

		// deallocate for node3 and node1
		pa1.Deallocate(ctx, testNodeName1)
		pa1.Deallocate(ctx, testNodeName3)
		pa2.Deallocate(ctx, testNodeName1)
		pa2.Deallocate(ctx, testNodeName3)

		// allocate again, testNodeName3 should have IPs from index 0, testNodeName3 IPs from index 2
		node3AllocPool1, err = pa1.AllocateFromPool(ctx, testNodeName3)
		Expect(err).NotTo(HaveOccurred())
		node3AllocPool2, err = pa2.AllocateFromPool(ctx, testNodeName3)
		Expect(err).NotTo(HaveOccurred())
		Expect(node3AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.1"))
		Expect(node3AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.15"))
		Expect(node3AllocPool2.StartIP.String()).To(BeEquivalentTo("172.16.0.1"))
		Expect(node3AllocPool2.EndIP.String()).To(BeEquivalentTo("172.16.0.10"))

		node1AllocPool1, err = pa1.AllocateFromPool(ctx, testNodeName1)
		Expect(err).ToNot(HaveOccurred())
		node1AllocPool2, err = pa2.AllocateFromPool(ctx, testNodeName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(node1AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.31"))
		Expect(node1AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.45"))
		Expect(node1AllocPool2.StartIP.String()).To(BeEquivalentTo("172.16.0.21"))
		Expect(node1AllocPool2.EndIP.String()).To(BeEquivalentTo("172.16.0.30"))
	})

	It("Deallocate from pool", func() {
		pool1 := getPool1()
		a := allocator.CreatePoolAllocatorFromIPPool(ctx, pool1, sets.New[string]())
		node1AllocPool1, err := a.AllocateFromPool(ctx, testNodeName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(node1AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.1"))
		Expect(node1AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.15"))

		a.Deallocate(ctx, testNodeName1)

		//Allocate to Node2, should get first range
		node2AllocPool1, err := a.AllocateFromPool(ctx, testNodeName2)
		Expect(err).NotTo(HaveOccurred())
		Expect(node2AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.1"))
		Expect(node2AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.15"))
	})

	It("No free ranges", func() {
		pool1 := getPool1()
		// pool is /24, must fail on the second allocation
		pool1.Spec.PerNodeBlockSize = 200
		a := allocator.CreatePoolAllocatorFromIPPool(ctx, pool1, sets.New[string]())
		node1Alloc, err := a.AllocateFromPool(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		Expect(node1Alloc).NotTo(BeNil())

		_, err = a.AllocateFromPool(ctx, testNodeName2)
		Expect(errors.Is(err, allocator.ErrNoFreeRanges)).To(BeTrue())
	})

	It("return NoFreeRanges in case if IP is too large", func() {
		testPool := &ipamv1alpha1.IPPool{
			ObjectMeta: v1.ObjectMeta{Name: "pool"},
			Spec: ipamv1alpha1.IPPoolSpec{
				Subnet:           "255.255.255.0/24",
				PerNodeBlockSize: 200,
				Gateway:          "255.255.255.1"},
		}

		a := allocator.CreatePoolAllocatorFromIPPool(ctx, testPool, sets.New[string]())
		_, err := a.AllocateFromPool(ctx, testNodeName1)
		Expect(err).NotTo(HaveOccurred())
		_, err = a.AllocateFromPool(ctx, testNodeName2)
		Expect(errors.Is(err, allocator.ErrNoFreeRanges)).To(BeTrue())
	})

	It("Allocate with Pool Status containing not selected Node", func() {
		pool1 := getPool1()
		pool1.Status = ipamv1alpha1.IPPoolStatus{
			Allocations: []ipamv1alpha1.Allocation{
				{
					NodeName: "not-in-selector",
					StartIP:  "192.168.0.1",
					EndIP:    "192.168.0.15",
				},
				{
					NodeName: testNodeName1,
					StartIP:  "192.168.0.16",
					EndIP:    "192.168.0.30",
				},
			},
		}
		selectedNodes := sets.New[string](testNodeName1)
		a := allocator.CreatePoolAllocatorFromIPPool(ctx, pool1, selectedNodes)
		node2AllocPool1, err := a.AllocateFromPool(ctx, testNodeName2)
		Expect(err).ToNot(HaveOccurred())
		Expect(node2AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.1"))
		Expect(node2AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.15"))
	})

	It("Allocate with Pool Status containing duplicate StartIP", func() {
		pool1 := getPool1()
		pool1.Status = ipamv1alpha1.IPPoolStatus{
			Allocations: []ipamv1alpha1.Allocation{
				{
					NodeName: testNodeName1,
					StartIP:  "192.168.0.1",
					EndIP:    "192.168.0.15",
				},
				{
					NodeName: testNodeName2,
					StartIP:  "192.168.0.1",
					EndIP:    "192.168.0.15",
				},
			},
		}
		selectedNodes := sets.New[string](testNodeName1, testNodeName2)
		a := allocator.CreatePoolAllocatorFromIPPool(ctx, pool1, selectedNodes)
		node2AllocPool1, err := a.AllocateFromPool(ctx, testNodeName2)
		Expect(err).ToNot(HaveOccurred())
		Expect(node2AllocPool1.StartIP.String()).To(BeEquivalentTo("192.168.0.16"))
		Expect(node2AllocPool1.EndIP.String()).To(BeEquivalentTo("192.168.0.30"))
	})

	It("ConfigureAndLoadAllocations - Data load test", func() {
		getValidData := func() *allocator.AllocatedRange {
			return &allocator.AllocatedRange{
				StartIP: net.ParseIP("192.168.0.16"),
				EndIP:   net.ParseIP("192.168.0.30"),
			}
		}

		testCases := []struct {
			in     *allocator.AllocatedRange
			loaded bool
		}{
			{ // valid data
				in:     getValidData(),
				loaded: true,
			},
			{ // different subnet, should ignore
				in: &allocator.AllocatedRange{
					StartIP: net.ParseIP("1.1.1.1"),
					EndIP:   net.ParseIP("1.1.1.2"),
				},
				loaded: false,
			},
			{ // no startIP, should ignore
				in: func() *allocator.AllocatedRange {
					d := getValidData()
					d.StartIP = nil
					return d
				}(),
				loaded: false,
			},
			{ // no endIP, should ignore
				in: func() *allocator.AllocatedRange {
					d := getValidData()
					d.EndIP = net.IPv4allrouter
					return d
				}(),
				loaded: false,
			},
			{ // start and end IPs are the same, should ignore
				in: func() *allocator.AllocatedRange {
					d := getValidData()
					d.StartIP = net.ParseIP("192.168.0.1")
					d.EndIP = net.ParseIP("192.168.0.1")
					return d
				}(),
				loaded: false,
			},
			{ // IPs out of subnet, should ignore
				in: func() *allocator.AllocatedRange {
					d := getValidData()
					d.StartIP = net.ParseIP("192.168.1.1")
					d.EndIP = net.ParseIP("192.168.1.15")
					return d
				}(),
				loaded: false,
			},
			{ // duplicate range, should ignore
				in: func() *allocator.AllocatedRange {
					d := getValidData()
					d.StartIP = net.ParseIP("192.168.0.1")
					d.EndIP = net.ParseIP("192.168.0.15")
					return d
				}(),
				loaded: false,
			},
			{ // ip invalid offset, should ignore
				in: func() *allocator.AllocatedRange {
					d := getValidData()
					d.StartIP = net.ParseIP("192.168.0.17")
					d.EndIP = net.ParseIP("192.168.0.31")
					return d
				}(),
				loaded: false,
			},
			{ // bad IP count, should ignore
				in: func() *allocator.AllocatedRange {
					d := getValidData()
					d.StartIP = net.ParseIP("192.168.0.16")
					d.EndIP = net.ParseIP("192.168.0.25")
					return d
				}(),
				loaded: false,
			},
		}
		for _, test := range testCases {
			pool1 := getPool1()
			pool1.Status = ipamv1alpha1.IPPoolStatus{
				Allocations: []ipamv1alpha1.Allocation{
					{
						NodeName: testNodeName1,
						StartIP:  "192.168.0.1",
						EndIP:    "192.168.0.15",
					},
					{
						NodeName: testNodeName2,
						StartIP:  test.in.StartIP.String(),
						EndIP:    test.in.EndIP.String(),
					},
				}}
			nodes := sets.New[string](testNodeName1, testNodeName2)
			pa1 := allocator.CreatePoolAllocatorFromIPPool(ctx, pool1, nodes)

			node1AllocFromAllocator, err := pa1.AllocateFromPool(ctx, testNodeName2)
			Expect(err).NotTo(HaveOccurred())
			if test.loaded {
				Expect(node1AllocFromAllocator).To(BeEquivalentTo(test.in))
			} else {
				Expect(node1AllocFromAllocator).NotTo(BeEquivalentTo(test.in))
			}
		}
	})
})
