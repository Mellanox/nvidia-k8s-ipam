/*
 Copyright 2025, NVIDIA CORPORATION & AFFILIATES
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

package controllers

import (
	"time"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("CIDRPool controller", func() {
	BeforeEach(func() {
		Expect(testClient.Create(ctx, getTestNode("node-1"))).To(Succeed())
		Expect(testClient.Create(ctx, getTestNode("node-2"))).To(Succeed())
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, getTestNode("node-1"))).To(Succeed())
		Expect(testClient.Delete(ctx, getTestNode("node-2"))).To(Succeed())
	})

	It("should reconcile CIDRPool", func() {
		// wait for nodes to exist in cache
		Eventually(func(g Gomega) {
			nl := &corev1.NodeList{}
			g.Expect(testClient.List(ctx, nl)).To(Succeed())
			g.Expect(nl.Items).To(HaveLen(2))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())

		// create CIDRPool
		cidrPool := getTestCIDRPool("test-pool")
		Expect(testClient.Create(ctx, cidrPool)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, cidrPool)).To(Succeed())
		})

		// validate allocations
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: cidrPool.Name, Namespace: cidrPool.Namespace}, cidrPool)).To(Succeed())
			g.Expect(cidrPool.Status.Allocations).To(HaveLen(2))
			g.Expect(cidrPool.Status.Allocations[0].NodeName).To(Equal("node-1"))
			g.Expect(cidrPool.Status.Allocations[0].Prefix).To(Equal("10.10.0.0/24"))
			g.Expect(cidrPool.Status.Allocations[1].NodeName).To(Equal("node-2"))
			g.Expect(cidrPool.Status.Allocations[1].Prefix).To(Equal("10.10.1.0/24"))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())
	})

	It("should reconcile CIDRPool with exclusions - exclude range", func() {
		// wait for nodes to exist in cache
		Eventually(func(g Gomega) {
			nl := &corev1.NodeList{}
			g.Expect(testClient.List(ctx, nl)).To(Succeed())
			g.Expect(nl.Items).To(HaveLen(2))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())

		// create CIDRPool
		cidrPool := getTestCIDRPool("test-pool")
		cidrPool.Spec.Exclusions = []ipamv1alpha1.ExcludeRange{
			{StartIP: "10.10.0.0", EndIP: "10.10.0.255"}, // exclude the first /24 subnet
			{StartIP: "10.10.1.0", EndIP: "10.10.1.100"}, // partially exclude the second /24 subnet
		}
		Expect(testClient.Create(ctx, cidrPool)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, cidrPool)).To(Succeed())
		})

		// validate allocations
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: cidrPool.Name, Namespace: cidrPool.Namespace}, cidrPool)).To(Succeed())
			g.Expect(cidrPool.Status.Allocations).To(HaveLen(2))
			g.Expect(cidrPool.Status.Allocations[0].NodeName).To(Equal("node-1"))
			g.Expect(cidrPool.Status.Allocations[0].Prefix).To(Equal("10.10.1.0/24"))
			g.Expect(cidrPool.Status.Allocations[1].NodeName).To(Equal("node-2"))
			g.Expect(cidrPool.Status.Allocations[1].Prefix).To(Equal("10.10.2.0/24"))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())
	})

	It("should reconcile CIDRPool with exclusions - exclude index", func() {
		// wait for nodes to exist in cache
		Eventually(func(g Gomega) {
			nl := &corev1.NodeList{}
			g.Expect(testClient.List(ctx, nl)).To(Succeed())
			g.Expect(nl.Items).To(HaveLen(2))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())

		// create CIDRPool
		cidrPool := getTestCIDRPool("test-pool")
		cidrPool.Spec.PerNodeExclusions = []ipamv1alpha1.ExcludeIndexRange{
			{StartIndex: 0, EndIndex: 127}, // exclude the first 128 addresses
		}
		Expect(testClient.Create(ctx, cidrPool)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, cidrPool)).To(Succeed())
		})

		// validate allocations
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: cidrPool.Name, Namespace: cidrPool.Namespace}, cidrPool)).To(Succeed())
			g.Expect(cidrPool.Status.Allocations).To(HaveLen(2))
			g.Expect(cidrPool.Status.Allocations[0].NodeName).To(Equal("node-1"))
			g.Expect(cidrPool.Status.Allocations[0].Prefix).To(Equal("10.10.0.0/24"))
			g.Expect(cidrPool.Status.Allocations[1].NodeName).To(Equal("node-2"))
			g.Expect(cidrPool.Status.Allocations[1].Prefix).To(Equal("10.10.1.0/24"))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())
	})

	It("should reconcile CIDRPool with exclusions - exclude range and index", func() {
		// wait for nodes to exist in cache
		Eventually(func(g Gomega) {
			nl := &corev1.NodeList{}
			g.Expect(testClient.List(ctx, nl)).To(Succeed())
			g.Expect(nl.Items).To(HaveLen(2))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())

		// create CIDRPool
		cidrPool := getTestCIDRPool("test-pool")
		cidrPool.Spec.Exclusions = []ipamv1alpha1.ExcludeRange{
			{StartIP: "10.10.0.0", EndIP: "10.10.0.255"}, // exclude the first /24 subnet
			{StartIP: "10.10.1.0", EndIP: "10.10.1.127"}, // exclude the first half of the second /24 subnet
		}
		cidrPool.Spec.PerNodeExclusions = []ipamv1alpha1.ExcludeIndexRange{
			{StartIndex: 128, EndIndex: 255}, // exclude the second half of the /24 range
		}
		Expect(testClient.Create(ctx, cidrPool)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, cidrPool)).To(Succeed())
		})

		// validate allocations
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: cidrPool.Name, Namespace: cidrPool.Namespace}, cidrPool)).To(Succeed())
			g.Expect(cidrPool.Status.Allocations).To(HaveLen(2))
			g.Expect(cidrPool.Status.Allocations[0].NodeName).To(Equal("node-1"))
			g.Expect(cidrPool.Status.Allocations[0].Prefix).To(Equal("10.10.2.0/24"))
			g.Expect(cidrPool.Status.Allocations[1].NodeName).To(Equal("node-2"))
			g.Expect(cidrPool.Status.Allocations[1].Prefix).To(Equal("10.10.3.0/24"))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())
	})
})

func getTestNode(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func getTestCIDRPool(name string) *ipamv1alpha1.CIDRPool {
	return &ipamv1alpha1.CIDRPool{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: ipamv1alpha1.CIDRPoolSpec{
			CIDR:                 "10.10.0.0/16",
			GatewayIndex:         ptr.To[int32](1),
			PerNodeNetworkPrefix: 24,
		},
	}
}
