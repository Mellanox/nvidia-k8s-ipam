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

package handlers_test

import (
	"context"
	"fmt"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	allocatorPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator"
	allocatorMockPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator/mocks"
	handlersPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/handlers"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	storeMockPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store/mocks"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
	poolMockPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool/mocks"
)

const (
	testPoolName1 = "pool1"
	testPoolName2 = "pool2"
	testPodName   = "test-pod"
	testNamespace = "default"
	testPodUID    = "aaf0a0fc-9869-41ef-9214-48599f85b4fa"
)

func getPoolConfigs() map[string]*pool.Pool {
	return map[string]*pool.Pool{
		testPoolName1: {
			Name:    testPoolName1,
			Subnet:  "192.168.0.0/16",
			StartIP: "192.168.0.2",
			EndIP:   "192.168.0.254",
			Gateway: "192.168.0.1",
		},
		testPoolName2: {Name: testPoolName2,
			Subnet:  "10.100.0.0/16",
			StartIP: "10.100.0.2",
			EndIP:   "10.100.0.254",
			Gateway: "10.100.0.1",
		},
	}
}

func getValidIPAMParams() *nodev1.IPAMParameters {
	return &nodev1.IPAMParameters{
		Pools:          []string{testPoolName1, testPoolName2},
		CniContainerid: "id1",
		CniIfname:      "net0",
		Metadata: &nodev1.IPAMMetadata{
			K8SPodName:      testPodName,
			K8SPodNamespace: testNamespace,
			K8SPodUid:       testPodUID,
			DeviceId:        "0000:d8:00.1",
		},
	}
}

var _ = Describe("Handlers", func() {
	var (
		poolManager  *poolMockPkg.Manager
		store        *storeMockPkg.Store
		session      *storeMockPkg.Session
		allocators   map[string]*allocatorMockPkg.IPAllocator
		getAllocFunc handlersPkg.GetAllocatorFunc
		handlers     *handlersPkg.Handlers
		ctx          context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
		poolManager = poolMockPkg.NewManager(GinkgoT())
		store = storeMockPkg.NewStore(GinkgoT())
		session = storeMockPkg.NewSession(GinkgoT())
		allocators = map[string]*allocatorMockPkg.IPAllocator{
			testPoolName1: allocatorMockPkg.NewIPAllocator(GinkgoT()),
			testPoolName2: allocatorMockPkg.NewIPAllocator(GinkgoT())}
		getAllocFunc = func(s *allocatorPkg.RangeSet, exclusions *allocatorPkg.RangeSet,
			poolName string, store storePkg.Session) allocatorPkg.IPAllocator {
			return allocators[poolName]
		}
		handlers = handlersPkg.New(poolManager, store, getAllocFunc)
	})

	It("Allocate succeed", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		poolManager.On("GetPoolByKey", testPoolName1).Return(getPoolConfigs()[testPoolName1])
		poolManager.On("GetPoolByKey", testPoolName2).Return(getPoolConfigs()[testPoolName2])
		allocators[testPoolName1].On("Allocate", "id1", "net0", mock.Anything, mock.Anything).Return(
			&current.IPConfig{
				Gateway: net.ParseIP("192.168.0.1"),
				Address: getIPWithMask("192.168.0.2/16"),
			}, nil)
		allocators[testPoolName2].On("Allocate", "id1", "net0", mock.Anything, mock.Anything).Return(
			&current.IPConfig{
				Gateway: net.ParseIP("10.100.0.1"),
				Address: getIPWithMask("10.100.0.2/16"),
			}, nil)
		session.On("Commit").Return(nil)

		resp, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: getValidIPAMParams()})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Allocations).To(HaveLen(2))
		Expect(resp.Allocations).To(ContainElement(
			And(
				HaveField("Pool", testPoolName1),
				HaveField("Ip", "192.168.0.2/16"),
				HaveField("Gateway", "192.168.0.1"),
			)))
		Expect(resp.Allocations).To(ContainElement(
			And(
				HaveField("Pool", testPoolName2),
				HaveField("Ip", "10.100.0.2/16"),
				HaveField("Gateway", "10.100.0.1"),
			)))
	})
	It("Allocate static IP", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		poolManager.On("GetPoolByKey", testPoolName1).Return(getPoolConfigs()[testPoolName1])
		allocators[testPoolName1].On("Allocate", "id1", "net0", mock.Anything, net.ParseIP("192.168.0.2")).Return(
			&current.IPConfig{
				Gateway: net.ParseIP("192.168.0.1"),
				Address: getIPWithMask("192.168.0.2/16"),
			}, nil)
		session.On("Commit").Return(nil)
		ipamParams := getValidIPAMParams()
		ipamParams.Pools = []string{testPoolName1}
		ipamParams.RequestedIps = []string{"192.168.0.2"}
		resp, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: ipamParams})
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Allocations).To(HaveLen(1))
		Expect(resp.Allocations).To(ContainElement(
			And(
				HaveField("Pool", testPoolName1),
				HaveField("Ip", "192.168.0.2/16"),
				HaveField("Gateway", "192.168.0.1"),
			)))
	})
	It("Allocate failed: static IP was not allocated", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		poolManager.On("GetPoolByKey", testPoolName1).Return(getPoolConfigs()[testPoolName1])
		allocators[testPoolName1].On("Allocate", "id1", "net0", mock.Anything, mock.Anything).Return(
			&current.IPConfig{
				Gateway: net.ParseIP("192.168.0.1"),
				Address: getIPWithMask("192.168.0.2/16"),
			}, nil)
		session.On("Cancel").Return()
		ipamParams := getValidIPAMParams()
		ipamParams.Pools = []string{testPoolName1}
		ipamParams.RequestedIps = []string{"10.10.10.10"}
		_, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: ipamParams})
		Expect(err).To(MatchError(ContainSubstring("not all requested static IPs can be allocated")))
	})
	It("Allocation failed: unknown pool", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		poolManager.On("GetPoolByKey", testPoolName1).Return(nil)
		session.On("Cancel").Return()
		_, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.NotFound).To(BeTrue())
	})
	It("Allocation failed: bad pool config", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		pool1Cfg := getPoolConfigs()[testPoolName1]
		endIP := pool1Cfg.EndIP
		startIP := pool1Cfg.StartIP
		pool1Cfg.StartIP = endIP
		pool1Cfg.EndIP = startIP
		poolManager.On("GetPoolByKey", testPoolName1).Return(pool1Cfg)
		session.On("Cancel").Return()
		_, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.Internal).To(BeTrue())
	})
	It("Allocation failed: pool2 has no free IPs", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		poolManager.On("GetPoolByKey", testPoolName1).Return(getPoolConfigs()[testPoolName1])
		poolManager.On("GetPoolByKey", testPoolName2).Return(getPoolConfigs()[testPoolName2])
		allocators[testPoolName1].On("Allocate", "id1", "net0", mock.Anything, mock.Anything).Return(
			&current.IPConfig{
				Gateway: net.ParseIP("192.168.0.1"),
				Address: getIPWithMask("192.168.0.2/16"),
			}, nil)
		allocators[testPoolName2].On("Allocate", "id1", "net0", mock.Anything, mock.Anything).Return(
			nil, allocatorPkg.ErrNoFreeAddresses)
		session.On("Cancel").Return()

		_, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.ResourceExhausted).To(BeTrue())
	})
	It("Allocation failed: already allocated", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		poolManager.On("GetPoolByKey", testPoolName1).Return(getPoolConfigs()[testPoolName1])
		allocators[testPoolName1].On("Allocate", "id1", "net0", mock.Anything, mock.Anything).Return(
			nil, storePkg.ErrReservationAlreadyExist)
		session.On("Cancel").Return()
		_, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.AlreadyExists).To(BeTrue())
	})
	It("Allocation failed: failed to commit", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		poolManager.On("GetPoolByKey", testPoolName1).Return(getPoolConfigs()[testPoolName1])
		poolManager.On("GetPoolByKey", testPoolName2).Return(getPoolConfigs()[testPoolName2])
		allocators[testPoolName1].On("Allocate", "id1", "net0", mock.Anything, mock.Anything).Return(
			&current.IPConfig{
				Gateway: net.ParseIP("192.168.0.1"),
				Address: getIPWithMask("192.168.0.2/16"),
			}, nil)
		allocators[testPoolName2].On("Allocate", "id1", "net0", mock.Anything, mock.Anything).Return(
			&current.IPConfig{
				Gateway: net.ParseIP("10.100.0.1"),
				Address: getIPWithMask("10.100.0.2/16"),
			}, nil)
		session.On("Commit").Return(fmt.Errorf("test"))
		_, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.Internal).To(BeTrue())
	})
	It("Allocation failed: canceled", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		session.On("Cancel").Return()
		ctx, cFunc := context.WithCancel(ctx)
		cFunc()
		_, err := handlers.Allocate(ctx, &nodev1.AllocateRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.Canceled).To(BeTrue())
	})
	It("IsAllocated succeed", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		session.On("GetReservationByID", testPoolName1, "id1", "net0").Return(&types.Reservation{})
		session.On("GetReservationByID", testPoolName2, "id1", "net0").Return(&types.Reservation{})
		session.On("Commit").Return(nil)
		_, err := handlers.IsAllocated(ctx, &nodev1.IsAllocatedRequest{Parameters: getValidIPAMParams()})
		Expect(err).NotTo(HaveOccurred())
	})
	It("IsAllocated failed: no reservation", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		session.On("GetReservationByID", testPoolName1, "id1", "net0").Return(nil)
		session.On("Cancel").Return(nil)
		_, err := handlers.IsAllocated(ctx, &nodev1.IsAllocatedRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.NotFound).To(BeTrue())
	})
	It("IsAllocated failed: canceled", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		session.On("Cancel").Return()
		ctx, cFunc := context.WithCancel(ctx)
		cFunc()
		_, err := handlers.IsAllocated(ctx, &nodev1.IsAllocatedRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.Canceled).To(BeTrue())
	})
	It("Deallocate succeed", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		session.On("ReleaseReservationByID", testPoolName1, "id1", "net0").Return()
		session.On("ReleaseReservationByID", testPoolName2, "id1", "net0").Return()
		session.On("Commit").Return(nil)
		_, err := handlers.Deallocate(ctx, &nodev1.DeallocateRequest{Parameters: getValidIPAMParams()})
		Expect(err).NotTo(HaveOccurred())
	})
	It("Deallocate failed: failed to commit", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		session.On("ReleaseReservationByID", testPoolName1, "id1", "net0").Return()
		session.On("ReleaseReservationByID", testPoolName2, "id1", "net0").Return()
		session.On("Commit").Return(fmt.Errorf("test error"))
		_, err := handlers.Deallocate(ctx, &nodev1.DeallocateRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.Internal).To(BeTrue())
	})
	It("Deallocate failed: canceled", func() {
		store.On("Open", mock.Anything).Return(session, nil)
		session.On("Cancel").Return()
		ctx, cFunc := context.WithCancel(ctx)
		cFunc()
		_, err := handlers.Deallocate(ctx, &nodev1.DeallocateRequest{Parameters: getValidIPAMParams()})
		Expect(status.Code(err) == codes.Canceled).To(BeTrue())
	})
})

func getIPWithMask(addr string) net.IPNet {
	ipAddr, netAddr, err := net.ParseCIDR(addr)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	netAddr.IP = ipAddr
	return *netAddr
}
