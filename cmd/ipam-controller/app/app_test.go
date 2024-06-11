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

package app_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-controller/app"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-controller/app/options"
)

const (
	pool1Name = "pool1"
	pool2Name = "pool2"
	pool3Name = "pool3"
	testNode1 = "node1"
	testNode2 = "node2"
	testNode3 = "node3"
)

func patchNodeLabel(nodeName, key, value string) {
	patchString := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q: %q}}}`, key, value))
	if value == "null" {
		patchString = []byte(fmt.Sprintf(`{"metadata":{"labels":{%q: null}}}`, key))
	}
	patch := client.RawPatch(types.MergePatchType, patchString)
	err := k8sClient.Patch(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}, patch)
	Expect(err).NotTo(HaveOccurred())
}

func getIPPoolRangeForNode(nodeName string, poolName string) *ipamv1alpha1.Allocation {
	allocations := getAllocationsFromIPPools(poolName)
	for _, a := range allocations {
		alloc := a
		if a.NodeName == nodeName {
			return &alloc
		}
	}
	return nil
}

func getCIDRPoolAllocationForNode(nodeName string, poolName string) *ipamv1alpha1.CIDRPoolAllocation {
	allocations := getAllocationsFromCIDRPools(poolName)
	for _, a := range allocations {
		alloc := a
		if a.NodeName == nodeName {
			return &alloc
		}
	}
	return nil
}

func getAllocationsFromIPPools(poolName string) []ipamv1alpha1.Allocation {
	pool := &ipamv1alpha1.IPPool{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: TestNamespace, Name: poolName}, pool)).NotTo(HaveOccurred())
	return pool.Status.Allocations
}

func getAllocationsFromCIDRPools(poolName string) []ipamv1alpha1.CIDRPoolAllocation {
	pool := &ipamv1alpha1.CIDRPool{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: TestNamespace, Name: poolName}, pool)).NotTo(HaveOccurred())
	return pool.Status.Allocations
}

func updateIPPoolSpec(poolName string, spec ipamv1alpha1.IPPoolSpec) {
	pool := &ipamv1alpha1.IPPool{}
	Expect(k8sClient.Get(
		ctx, types.NamespacedName{Name: poolName, Namespace: TestNamespace}, pool)).NotTo(HaveOccurred())
	pool.Spec = spec
	Expect(k8sClient.Update(ctx, pool)).NotTo(HaveOccurred())
}

func updateCIDRPoolSpec(poolName string, spec ipamv1alpha1.CIDRPoolSpec) {
	pool := &ipamv1alpha1.CIDRPool{}
	Expect(k8sClient.Get(
		ctx, types.NamespacedName{Name: poolName, Namespace: TestNamespace}, pool)).NotTo(HaveOccurred())
	pool.Spec = spec
	Expect(k8sClient.Update(ctx, pool)).NotTo(HaveOccurred())
}

// WaitAndCheckForStability wait for condition and then check it is stable for 1 second
func WaitAndCheckForStability(check func(g Gomega), wait interface{}, stability interface{}) {
	Eventually(func(g Gomega) { check(g) }, wait).Should(Succeed())
	Consistently(func(g Gomega) { check(g) }, stability).Should(Succeed())
}

var _ = Describe("App", func() {
	BeforeEach(func() {
		for _, nodeName := range []string{testNode1, testNode2, testNode3} {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
			Expect(k8sClient.Create(ctx, node)).NotTo(HaveOccurred())
		}
	})
	AfterEach(func() {
		Eventually(func(g Gomega) {
			for _, nodeName := range []string{testNode1, testNode2, testNode3} {
				node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
				g.Expect(k8sClient.Delete(ctx, node)).NotTo(HaveOccurred())
			}
			nodeList := &corev1.NodeList{}
			g.Expect(k8sClient.List(ctx, nodeList)).NotTo(HaveOccurred())
			g.Expect(nodeList.Items).To(BeEmpty())
		}, time.Second*15).Should(Succeed())

	})
	It("IPPool Basic tests", func(ctx SpecContext) {
		cfg1pools := []string{pool1Name, pool2Name}

		By("Create valid pools")
		pool1 := &ipamv1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool1Name,
				Namespace: TestNamespace,
			},
			Spec: ipamv1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				Gateway:          "192.168.0.1",
				PerNodeBlockSize: 10,
			},
		}
		Expect(k8sClient.Create(ctx, pool1)).NotTo(HaveOccurred())

		pool2 := &ipamv1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool2Name,
				Namespace: TestNamespace,
			},
			Spec: ipamv1alpha1.IPPoolSpec{
				Subnet:           "172.16.0.0/16",
				Gateway:          "172.16.0.1",
				PerNodeBlockSize: 50,
			},
		}
		Expect(k8sClient.Create(ctx, pool2)).NotTo(HaveOccurred())

		By("Update Pools Status with valid ranges for node1 and invalid for node2 (wrong IP count)")
		node1InitialRanges := map[string]ipamv1alpha1.Allocation{pool1Name: {
			NodeName: testNode1,
			StartIP:  "192.168.0.11",
			EndIP:    "192.168.0.20",
		}, pool2Name: {
			NodeName: testNode1,
			StartIP:  "172.16.0.1",
			EndIP:    "172.16.0.50",
		}}
		pool1.Status = ipamv1alpha1.IPPoolStatus{
			Allocations: []ipamv1alpha1.Allocation{
				node1InitialRanges[pool1Name],
				{
					NodeName: testNode2,
					StartIP:  "192.168.0.11",
					EndIP:    "192.168.0.14",
				},
			},
		}
		pool2.Status = ipamv1alpha1.IPPoolStatus{
			Allocations: []ipamv1alpha1.Allocation{
				node1InitialRanges[pool2Name],
			},
		}
		Expect(k8sClient.Status().Update(ctx, pool1)).NotTo(HaveOccurred())
		Expect(k8sClient.Status().Update(ctx, pool2)).NotTo(HaveOccurred())

		By("Start controller")

		ctrlCtx, ctrlStopFunc := context.WithCancel(ctx)
		defer ctrlStopFunc()
		controllerStopped := make(chan struct{})

		go func() {
			Expect(app.RunController(logr.NewContext(ctrlCtx, klog.NewKlogr()), cfg, &options.Options{
				MetricsAddr:             "0", // disable
				ProbeAddr:               "0", // disable
				IPPoolsNamespace:        TestNamespace,
				EnableLeaderElection:    true,
				LeaderElectionNamespace: TestNamespace,
			})).NotTo(HaveOccurred())
			close(controllerStopped)
		}()

		By("Check how controller handled the state")
		WaitAndCheckForStability(func(g Gomega) {
			uniqStartEndIPs := map[string]struct{}{}
			pool1Allocations := getAllocationsFromIPPools(pool1Name)
			pool2Allocations := getAllocationsFromIPPools(pool2Name)
			for _, r := range [][]ipamv1alpha1.Allocation{pool1Allocations, pool2Allocations} {
				g.Expect(r).To(HaveLen(3)) // 3 allocations, 1 for each node
				for _, a := range r {
					uniqStartEndIPs[a.StartIP] = struct{}{}
					uniqStartEndIPs[a.EndIP] = struct{}{}
				}
			}
			// we should have unique start/end IPs for each node for each pool
			g.Expect(uniqStartEndIPs).To(HaveLen(len(cfg1pools) * 3 * 2))
			// node1 should have restored ranges
			g.Expect(pool1Allocations).To(ContainElement(Equal(node1InitialRanges[pool1Name])))
			g.Expect(pool2Allocations).To(ContainElement(Equal(node1InitialRanges[pool2Name])))

		}, 15, 2)

		By("Set invalid config for pool1")
		invalidPool1Spec := ipamv1alpha1.IPPoolSpec{
			Subnet:           "192.168.0.0/16",
			Gateway:          "10.10.0.1",
			PerNodeBlockSize: 10,
		}
		updateIPPoolSpec(pool1Name, invalidPool1Spec)
		Eventually(func(g Gomega) {
			g.Expect(getAllocationsFromIPPools(pool1Name)).To(BeEmpty())
		}, 15, 1).Should(Succeed())

		By("Create Pool3, with selector which ignores all nodes")
		// ranges for two nodes only can be allocated
		pool3Spec := ipamv1alpha1.IPPoolSpec{
			Subnet:           "172.17.0.0/24",
			Gateway:          "172.17.0.1",
			PerNodeBlockSize: 100,
			NodeSelector: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "ippool",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
						MatchFields: nil,
					},
				},
			},
		}
		pool3 := &ipamv1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool3Name,
				Namespace: TestNamespace,
			},
			Spec: pool3Spec,
		}
		Expect(k8sClient.Create(ctx, pool3)).NotTo(HaveOccurred())

		Consistently(func(g Gomega) {
			g.Expect(getAllocationsFromIPPools(pool3Name)).To(BeEmpty())
		}, 10, 1).Should(Succeed())

		By("Update nodes to match selector in pool3")

		// node1 should have range
		patchNodeLabel(testNode1, "ippool", "true")
		WaitAndCheckForStability(func(g Gomega) {
			g.Expect(getIPPoolRangeForNode(testNode1, pool3Name)).NotTo(BeNil())
		}, 15, 2)

		// node2 should have range
		patchNodeLabel(testNode2, "ippool", "true")
		WaitAndCheckForStability(func(g Gomega) {
			g.Expect(getIPPoolRangeForNode(testNode2, pool3Name)).NotTo(BeNil())
		}, 15, 2)

		// node3 should have no range, because no free ranges available
		patchNodeLabel(testNode3, "ippool", "true")
		WaitAndCheckForStability(func(g Gomega) {
			g.Expect(getIPPoolRangeForNode(testNode3, pool3Name)).To(BeNil())
		}, 15, 5)

		node2Range := getIPPoolRangeForNode(testNode2, pool3Name)
		// remove label from node2, node3 should have a range now
		patchNodeLabel(testNode2, "ippool", "null")
		WaitAndCheckForStability(func(g Gomega) {
			node3Range := getIPPoolRangeForNode(testNode3, pool3Name)
			g.Expect(node3Range).NotTo(BeNil())
			// should reuse ranges from node2
			matchRange := node3Range.StartIP == node2Range.StartIP &&
				node3Range.EndIP == node2Range.EndIP
			g.Expect(matchRange).To(BeTrue())
		}, 15, 2)

		By("Stop controller")
		ctrlStopFunc()
		Eventually(controllerStopped, 30*time.Second).Should(BeClosed())
	}, SpecTimeout(time.Minute*5))

	It("CIDRPool Basic tests", func(ctx SpecContext) {
		By("Create valid pools")
		pool1gatewayIndex := uint(1)
		pool1 := &ipamv1alpha1.CIDRPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool1Name,
				Namespace: TestNamespace,
			},
			Spec: ipamv1alpha1.CIDRPoolSpec{
				CIDR:                 "10.10.0.0/16",
				GatewayIndex:         &pool1gatewayIndex,
				PerNodeNetworkPrefix: 24,
				Exclusions: []ipamv1alpha1.ExcludeRange{
					{StartIP: "10.10.1.22", EndIP: "10.10.1.30"},
				},
				StaticAllocations: []ipamv1alpha1.CIDRPoolStaticAllocation{{
					NodeName: testNode3,
					Prefix:   "10.10.100.0/24",
					Gateway:  "10.10.100.10",
				}, {Prefix: "10.10.1.0/24"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool1)).NotTo(HaveOccurred())

		pool2gatewayIndex := uint(1)
		pool2 := &ipamv1alpha1.CIDRPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool2Name,
				Namespace: TestNamespace,
			},
			Spec: ipamv1alpha1.CIDRPoolSpec{
				CIDR:                 "192.168.0.0/16",
				GatewayIndex:         &pool2gatewayIndex,
				PerNodeNetworkPrefix: 31,
			},
		}
		Expect(k8sClient.Create(ctx, pool2)).NotTo(HaveOccurred())

		By("Update Pools Status with valid allocation for node1 and invalid for node2")
		node1InitialAllocations := map[string]ipamv1alpha1.CIDRPoolAllocation{pool1Name: {
			NodeName: testNode1,
			Prefix:   "10.10.0.0/24",
			Gateway:  "10.10.0.1",
		}, pool2Name: {
			NodeName: testNode1,
			Prefix:   "192.168.100.0/31",
			Gateway:  "192.168.100.1",
		}}
		pool1.Status = ipamv1alpha1.CIDRPoolStatus{
			Allocations: []ipamv1alpha1.CIDRPoolAllocation{
				node1InitialAllocations[pool1Name],
				{
					NodeName: testNode2,
					Prefix:   "10.33.33.0/24",
					Gateway:  "10.33.33.1",
				},
			},
		}
		pool2.Status = ipamv1alpha1.CIDRPoolStatus{
			Allocations: []ipamv1alpha1.CIDRPoolAllocation{
				node1InitialAllocations[pool2Name],
			},
		}
		Expect(k8sClient.Status().Update(ctx, pool1)).NotTo(HaveOccurred())
		Expect(k8sClient.Status().Update(ctx, pool2)).NotTo(HaveOccurred())

		By("Start controller")

		ctrlCtx, ctrlStopFunc := context.WithCancel(ctx)
		defer ctrlStopFunc()
		controllerStopped := make(chan struct{})

		go func() {
			Expect(app.RunController(logr.NewContext(ctrlCtx, klog.NewKlogr()), cfg, &options.Options{
				MetricsAddr:             "0", // disable
				ProbeAddr:               "0", // disable
				IPPoolsNamespace:        TestNamespace,
				EnableLeaderElection:    true,
				LeaderElectionNamespace: TestNamespace,
			})).NotTo(HaveOccurred())
			close(controllerStopped)
		}()

		By("Check how controller handled the state")
		WaitAndCheckForStability(func(g Gomega) {
			pool1Allocations := getAllocationsFromCIDRPools(pool1Name)
			pool2Allocations := getAllocationsFromCIDRPools(pool2Name)
			g.Expect(pool1Allocations).To(HaveLen(3))
			g.Expect(pool2Allocations).To(HaveLen(3))
			g.Expect(pool1Allocations).To(ContainElements(Equal(
				ipamv1alpha1.CIDRPoolAllocation{
					NodeName: testNode3, // has static allocation
					Prefix:   "10.10.100.0/24",
					Gateway:  "10.10.100.10",
				}), Equal(node1InitialAllocations[pool1Name])))
			g.Expect(pool2Allocations).To(ContainElements(Equal(node1InitialAllocations[pool2Name])))
		}, 15, 2)

		By("Set invalid config for pool1")
		invalidPool1Spec := ipamv1alpha1.CIDRPoolSpec{
			CIDR:                 "192.168.0.0/16",
			PerNodeNetworkPrefix: 99,
		}
		updateCIDRPoolSpec(pool1Name, invalidPool1Spec)
		Eventually(func(g Gomega) {
			g.Expect(getAllocationsFromCIDRPools(pool1Name)).To(BeEmpty())
		}, 15, 1).Should(Succeed())

		By("Create Pool3, with selector which ignores all nodes")
		// ranges for two nodes only can be allocated
		pool3gatewayIndex := uint(1)
		pool3 := &ipamv1alpha1.CIDRPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool3Name,
				Namespace: TestNamespace,
			},
			Spec: ipamv1alpha1.CIDRPoolSpec{
				CIDR:                 "10.10.0.0/24",
				GatewayIndex:         &pool3gatewayIndex,
				PerNodeNetworkPrefix: 25,
				NodeSelector: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "cidrpool",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"true"},
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, pool3)).NotTo(HaveOccurred())

		Consistently(func(g Gomega) {
			g.Expect(getAllocationsFromCIDRPools(pool3Name)).To(BeEmpty())
		}, 10, 1).Should(Succeed())

		By("Update nodes to match selector in pool3")

		// node1 should have range
		patchNodeLabel(testNode1, "cidrpool", "true")
		WaitAndCheckForStability(func(g Gomega) {
			g.Expect(getCIDRPoolAllocationForNode(testNode1, pool3Name)).NotTo(BeNil())
		}, 15, 2)

		// node2 should have range
		patchNodeLabel(testNode2, "cidrpool", "true")
		WaitAndCheckForStability(func(g Gomega) {
			g.Expect(getCIDRPoolAllocationForNode(testNode2, pool3Name)).NotTo(BeNil())
		}, 15, 2)

		// node3 should have no range, because no free ranges available
		patchNodeLabel(testNode3, "cidrpool", "true")
		WaitAndCheckForStability(func(g Gomega) {
			g.Expect(getCIDRPoolAllocationForNode(testNode3, pool3Name)).To(BeNil())
		}, 15, 5)

		node2Alloc := getCIDRPoolAllocationForNode(testNode2, pool3Name)
		// remove label from node2, node3 should have a range now
		patchNodeLabel(testNode2, "cidrpool", "null")
		WaitAndCheckForStability(func(g Gomega) {
			node3Alloc := getCIDRPoolAllocationForNode(testNode3, pool3Name)
			g.Expect(node3Alloc).NotTo(BeNil())
			// should reuse ranges from node2
			g.Expect(node3Alloc.Prefix).To(Equal(node2Alloc.Prefix))
		}, 15, 2)

		By("Stop controller")
		ctrlStopFunc()
		Eventually(controllerStopped, 30*time.Second).Should(BeClosed())
	}, SpecTimeout(time.Minute*5))
})
