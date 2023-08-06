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
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
)

func createNode(name string) *corev1.Node {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	Expect(k8sClient.Create(ctx, node)).NotTo(HaveOccurred())
	return node
}

func getNode(name string) *corev1.Node {
	node := &corev1.Node{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, node)).NotTo(HaveOccurred())
	return node
}

func updateNode(node *corev1.Node) *corev1.Node {
	Expect(k8sClient.Update(ctx, node)).NotTo(HaveOccurred())
	return node
}

func getRangeForNode(nodeName string, poolName string) *ipamv1alpha1.Allocation {
	allocations := getAllocationsFromIPPools(poolName)

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

func checkAllocationExists(allocations []ipamv1alpha1.Allocation, allocation ipamv1alpha1.Allocation) bool {
	for _, a := range allocations {
		if reflect.DeepEqual(a, allocation) {
			return true
		}
	}
	return false
}

func updatePoolSpec(poolName string, spec ipamv1alpha1.IPPoolSpec) {
	pool := &ipamv1alpha1.IPPool{}
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
	It("Basic tests", func() {
		done := make(chan interface{})
		go func() {
			testNode1 := "node1"
			testNode2 := "node2"
			testNode3 := "node3"

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
			createNode(testNode1)
			createNode(testNode2)
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
					MetricsAddr:      "0", // disable
					ProbeAddr:        "0", // disable
					IPPoolsNamespace: TestNamespace,
				})).NotTo(HaveOccurred())
				close(controllerStopped)
			}()

			By("Create node3")
			createNode(testNode3)

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
				g.Expect(checkAllocationExists(pool1Allocations, node1InitialRanges[pool1Name])).To(BeTrue())
				g.Expect(checkAllocationExists(pool2Allocations, node1InitialRanges[pool2Name])).To(BeTrue())

			}, 15, 2)

			By("Set invalid config for pool1")
			invalidPool1Spec := ipamv1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				Gateway:          "10.10.0.1",
				PerNodeBlockSize: 10,
			}
			updatePoolSpec(pool1Name, invalidPool1Spec)
			Eventually(func(g Gomega) bool {
				pool := &ipamv1alpha1.IPPool{}
				g.Expect(k8sClient.Get(
					ctx, types.NamespacedName{Name: pool1Name, Namespace: TestNamespace}, pool)).NotTo(HaveOccurred())
				return len(pool.Status.Allocations) == 0
			}, 30, 5).Should(BeTrue())

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
									Key:      "foo",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"bar"},
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

			Consistently(func(g Gomega) bool {
				pool := &ipamv1alpha1.IPPool{}
				g.Expect(k8sClient.Get(
					ctx, types.NamespacedName{Name: pool3Name, Namespace: TestNamespace}, pool)).NotTo(HaveOccurred())
				return len(pool.Status.Allocations) == 0
			}, 30, 5).Should(BeTrue())

			By("Update nodes to match selector in pool3")

			// node1 should have range
			node1 := getNode(testNode1)
			node1.Labels = map[string]string{"foo": "bar"}
			updateNode(node1)
			WaitAndCheckForStability(func(g Gomega) {
				g.Expect(getRangeForNode(testNode1, pool3Name)).NotTo(BeNil())
			}, 15, 2)

			// node2 should have range
			node2 := getNode(testNode2)
			node2.Labels = map[string]string{"foo": "bar"}
			updateNode(node2)
			WaitAndCheckForStability(func(g Gomega) {
				g.Expect(getRangeForNode(testNode2, pool3Name)).NotTo(BeNil())
			}, 15, 2)

			// node3 should have no range, because no free ranges available
			node3 := getNode(testNode3)
			node3.Labels = map[string]string{"foo": "bar"}
			updateNode(node3)
			WaitAndCheckForStability(func(g Gomega) {
				g.Expect(getRangeForNode(testNode3, pool3Name)).To(BeNil())
			}, 15, 5)

			node2Range := getRangeForNode(testNode2, pool3Name)
			// remove label from node2, node3 should have a range now
			node2 = getNode(testNode2)
			node2.Labels = nil
			updateNode(node2)
			WaitAndCheckForStability(func(g Gomega) {
				node3Range := getRangeForNode(testNode3, pool3Name)
				g.Expect(node3Range).NotTo(BeNil())
				// should reuse ranges from node2
				matchRange := node3Range.StartIP == node2Range.StartIP &&
					node3Range.EndIP == node2Range.EndIP
				g.Expect(matchRange).To(BeTrue())
			}, 15, 2)

			By("Stop controller")
			ctrlStopFunc()
			Eventually(controllerStopped, 30*time.Second).Should(BeClosed())

			close(done)
		}()
		Eventually(done, 5*time.Minute).Should(BeClosed())
	})
})
