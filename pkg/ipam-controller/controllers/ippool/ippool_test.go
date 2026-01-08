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
)

var _ = Describe("IPPool controller", func() {
	BeforeEach(func() {
		Expect(testClient.Create(ctx, getTestNode("node-1"))).To(Succeed())
		Expect(testClient.Create(ctx, getTestNode("node-2"))).To(Succeed())
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, getTestNode("node-1"))).To(Succeed())
		Expect(testClient.Delete(ctx, getTestNode("node-2"))).To(Succeed())
	})

	It("should reconcile IPPool", func() {
		// wait for nodes to exist in cache
		Eventually(func(g Gomega) {
			nl := &corev1.NodeList{}
			g.Expect(testClient.List(ctx, nl)).To(Succeed())
			g.Expect(nl.Items).To(HaveLen(2))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())

		// create IPPool
		ipPool := getTestIPPool("test-pool")
		Expect(testClient.Create(ctx, ipPool)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, ipPool)).To(Succeed())
		})

		// validate allocations
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: ipPool.Name, Namespace: ipPool.Namespace}, ipPool)).To(Succeed())
			g.Expect(ipPool.Status.Allocations).To(HaveLen(2))
			g.Expect(ipPool.Status.Allocations[0].NodeName).To(Equal("node-1"))
			g.Expect(ipPool.Status.Allocations[0].StartIP).To(Equal("10.10.0.1"))
			g.Expect(ipPool.Status.Allocations[0].EndIP).To(Equal("10.10.1.0"))
			g.Expect(ipPool.Status.Allocations[1].NodeName).To(Equal("node-2"))
			g.Expect(ipPool.Status.Allocations[1].StartIP).To(Equal("10.10.1.1"))
			g.Expect(ipPool.Status.Allocations[1].EndIP).To(Equal("10.10.2.0"))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())
	})

	It("should reconcile IPPool with exclusions - exclude range", func() {
		// wait for nodes to exist in cache
		Eventually(func(g Gomega) {
			nl := &corev1.NodeList{}
			g.Expect(testClient.List(ctx, nl)).To(Succeed())
			g.Expect(nl.Items).To(HaveLen(2))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())

		// create IPPool
		ipPool := getTestIPPool("test-pool")
		ipPool.Spec.Exclusions = []ipamv1alpha1.ExcludeRange{
			{StartIP: "10.10.0.0", EndIP: "10.10.0.255"}, // exclude the first 256 addresses + network address
			{StartIP: "10.10.1.0", EndIP: "10.10.1.100"}, // partially exclude another 100 addresses
		}
		Expect(testClient.Create(ctx, ipPool)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, ipPool)).To(Succeed())
		})

		// validate allocations
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: ipPool.Name, Namespace: ipPool.Namespace}, ipPool)).To(Succeed())
			g.Expect(ipPool.Status.Allocations).To(HaveLen(2))
			g.Expect(ipPool.Status.Allocations[0].NodeName).To(Equal("node-1"))
			g.Expect(ipPool.Status.Allocations[0].StartIP).To(Equal("10.10.1.1"))
			g.Expect(ipPool.Status.Allocations[0].EndIP).To(Equal("10.10.2.0"))
			g.Expect(ipPool.Status.Allocations[1].NodeName).To(Equal("node-2"))
			g.Expect(ipPool.Status.Allocations[1].StartIP).To(Equal("10.10.2.1"))
			g.Expect(ipPool.Status.Allocations[1].EndIP).To(Equal("10.10.3.0"))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())
	})

	It("should reconcile IPPool with exclusions - exclude index", func() {
		// wait for nodes to exist in cache
		Eventually(func(g Gomega) {
			nl := &corev1.NodeList{}
			g.Expect(testClient.List(ctx, nl)).To(Succeed())
			g.Expect(nl.Items).To(HaveLen(2))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())

		// create IPPool
		ipPool := getTestIPPool("test-pool")
		ipPool.Spec.PerNodeExclusions = []ipamv1alpha1.ExcludeIndexRange{
			{StartIndex: 0, EndIndex: 127}, // exclude the first 128 addresses of each block of 256 addresses
		}
		Expect(testClient.Create(ctx, ipPool)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, ipPool)).To(Succeed())
		})

		// validate allocations
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: ipPool.Name, Namespace: ipPool.Namespace}, ipPool)).To(Succeed())
			g.Expect(ipPool.Status.Allocations).To(HaveLen(2))
			g.Expect(ipPool.Status.Allocations[0].NodeName).To(Equal("node-1"))
			g.Expect(ipPool.Status.Allocations[0].StartIP).To(Equal("10.10.0.1"))
			g.Expect(ipPool.Status.Allocations[0].EndIP).To(Equal("10.10.1.0"))
			g.Expect(ipPool.Status.Allocations[1].NodeName).To(Equal("node-2"))
			g.Expect(ipPool.Status.Allocations[1].StartIP).To(Equal("10.10.1.1"))
			g.Expect(ipPool.Status.Allocations[1].EndIP).To(Equal("10.10.2.0"))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())
	})

	It("should reconcile IPPool with exclusions - exclude range and index", func() {
		// wait for nodes to exist in cache
		Eventually(func(g Gomega) {
			nl := &corev1.NodeList{}
			g.Expect(testClient.List(ctx, nl)).To(Succeed())
			g.Expect(nl.Items).To(HaveLen(2))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())

		// create IPPool
		ipPool := getTestIPPool("test-pool")
		ipPool.Spec.Exclusions = []ipamv1alpha1.ExcludeRange{
			{StartIP: "10.10.0.0", EndIP: "10.10.1.0"},   // exclude the first 256 addresses + network address
			{StartIP: "10.10.1.1", EndIP: "10.10.1.128"}, // exclude the first half of the next 256 addresses
		}
		ipPool.Spec.PerNodeExclusions = []ipamv1alpha1.ExcludeIndexRange{
			{StartIndex: 128, EndIndex: 255}, // exclude the second half of any 256 addresses block
		}
		Expect(testClient.Create(ctx, ipPool)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, ipPool)).To(Succeed())
		})

		// validate allocations
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, types.NamespacedName{Name: ipPool.Name, Namespace: ipPool.Namespace}, ipPool)).To(Succeed())
			g.Expect(ipPool.Status.Allocations).To(HaveLen(2))
			g.Expect(ipPool.Status.Allocations[0].NodeName).To(Equal("node-1"))
			g.Expect(ipPool.Status.Allocations[0].StartIP).To(Equal("10.10.2.1"))
			g.Expect(ipPool.Status.Allocations[0].EndIP).To(Equal("10.10.3.0"))
			g.Expect(ipPool.Status.Allocations[1].NodeName).To(Equal("node-2"))
			g.Expect(ipPool.Status.Allocations[1].StartIP).To(Equal("10.10.3.1"))
			g.Expect(ipPool.Status.Allocations[1].EndIP).To(Equal("10.10.4.0"))
		}).WithTimeout(time.Second * 10).WithPolling(time.Second).Should(Succeed())
	})
})

func getTestNode(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func getTestIPPool(name string) *ipamv1alpha1.IPPool {
	return &ipamv1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: ipamv1alpha1.IPPoolSpec{
			Subnet:           "10.10.0.0/16",
			PerNodeBlockSize: 256,
			Gateway:          "10.10.0.1",
		},
	}
}
