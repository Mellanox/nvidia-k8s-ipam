/*
 Copyright 2024, NVIDIA CORPORATION & AFFILIATES
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
	"context"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
	poolPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

const (
	testNamespace = "test-ns"
	testNodeName  = "test-node"
)

var _ = Describe("CIDRPool", func() {
	DescribeTable("buildExclusions",
		func(exclusions []ipamv1alpha1.ExcludeRange, nodeSubnet string, firstIP string, lastIP string, result []pool.ExclusionRange) {
			_, subnet, _ := net.ParseCIDR(nodeSubnet)
			first := net.ParseIP(firstIP)
			last := net.ParseIP(lastIP)
			Expect(buildExclusions(exclusions, subnet, first, last)).To(Equal(result))
		},
		Entry("start and end are part of the subnet",
			[]ipamv1alpha1.ExcludeRange{
				{StartIP: "192.168.0.10", EndIP: "192.168.0.20"},
				{StartIP: "192.168.0.30", EndIP: "192.168.0.40"}},
			"192.168.0.0/24", "192.168.0.1", "192.168.0.254",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.10", EndIP: "192.168.0.20"},
				{StartIP: "192.168.0.30", EndIP: "192.168.0.40"},
			},
		),
		Entry("start and end are out of the subnet",
			[]ipamv1alpha1.ExcludeRange{
				{StartIP: "192.168.100.10", EndIP: "192.168.100.20"},
				{StartIP: "192.168.0.30", EndIP: "192.168.0.40"}},
			"192.168.0.0/24", "192.168.0.1", "192.168.0.254",
			[]pool.ExclusionRange{
				{StartIP: "192.168.0.30", EndIP: "192.168.0.40"},
			},
		),
		Entry("start is out of the subnet",
			[]ipamv1alpha1.ExcludeRange{
				{StartIP: "192.168.0.30", EndIP: "192.168.1.40"}},
			"192.168.1.0/24", "192.168.1.1", "192.168.1.254",
			[]pool.ExclusionRange{
				{StartIP: "192.168.1.1", EndIP: "192.168.1.40"},
			},
		),
		Entry("end is out of the subnet",
			[]ipamv1alpha1.ExcludeRange{
				{StartIP: "192.168.1.30", EndIP: "192.168.2.40"}},
			"192.168.1.0/24", "192.168.1.1", "192.168.1.254",
			[]pool.ExclusionRange{
				{StartIP: "192.168.1.30", EndIP: "192.168.1.254"},
			},
		),
	)
	Context("Controller tests", Ordered, func() {
		var (
			err         error
			cfg         *rest.Config
			k8sClient   client.Client
			testEnv     *envtest.Environment
			cancelFunc  context.CancelFunc
			ctx         context.Context
			poolManager poolPkg.Manager
		)

		BeforeAll(func() {
			poolManager = poolPkg.NewManager()
			By("bootstrapping test environment")
			testEnv = &envtest.Environment{
				CRDDirectoryPaths: []string{"../../../../deploy/crds"},
				CRDInstallOptions: envtest.CRDInstallOptions{
					ErrorIfPathMissing: true,
				},
			}
			ctx, cancelFunc = context.WithCancel(context.Background())
			Expect(ipamv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

			cfg, err = testEnv.Start()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).NotTo(BeNil())

			k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient).NotTo(BeNil())

			Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}})).To(BeNil())

			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:  scheme.Scheme,
				Metrics: metricsserver.Options{BindAddress: "0"},
				Cache: cache.Options{
					DefaultNamespaces: map[string]cache.Config{testNamespace: {}},
					ByObject: map[client.Object]cache.ByObject{
						&corev1.Pod{}: {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}}},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect((&CIDRPoolReconciler{
				PoolManager: poolManager,
				Client:      mgr.GetClient(),
				Scheme:      mgr.GetScheme(),
				NodeName:    testNodeName,
			}).SetupWithManager(mgr)).NotTo(HaveOccurred())

			go func() {
				defer GinkgoRecover()
				Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
			}()
		})
		AfterAll(func() {
			cancelFunc()
			By("tearing down the test environment")
			err := testEnv.Stop()
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func() {
			cidrPoolList := &ipamv1alpha1.CIDRPoolList{}
			Expect(k8sClient.List(ctx, cidrPoolList)).NotTo(HaveOccurred())
			for _, p := range cidrPoolList.Items {
				Expect(k8sClient.Delete(ctx, &p)).NotTo(HaveOccurred())
			}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, cidrPoolList)).NotTo(HaveOccurred())
				g.Expect(cidrPoolList.Items).To(BeEmpty())
				g.Expect(poolManager.GetPools()).To(BeEmpty())
			}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
		})
		It("Valid pool config", func() {
			p := &ipamv1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: testNamespace},
				Spec: ipamv1alpha1.CIDRPoolSpec{
					CIDR:                 "10.10.0.0/16",
					GatewayIndex:         ptr.To[int32](1),
					PerNodeNetworkPrefix: 24,
					Exclusions: []ipamv1alpha1.ExcludeRange{
						{StartIP: "10.10.33.10", EndIP: "10.10.33.20"},
						{StartIP: "10.10.10.10", EndIP: "10.10.10.20"},
					},
					DefaultGateway: true,
					Routes:         []ipamv1alpha1.Route{{Dst: "5.5.5.5/32"}},
				},
			}
			Expect(k8sClient.Create(ctx, p)).NotTo(HaveOccurred())
			p.Status.Allocations = []ipamv1alpha1.CIDRPoolAllocation{{
				NodeName: testNodeName,
				Gateway:  "10.10.10.1",
				Prefix:   "10.10.10.0/24",
			}}
			Expect(k8sClient.Status().Update(ctx, p)).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				g.Expect(poolManager.GetPoolByKey(
					common.GetPoolKey("test-pool", common.PoolTypeCIDRPool))).To(
					Equal(&pool.Pool{
						Name:    "test-pool",
						Subnet:  "10.10.10.0/24",
						StartIP: "10.10.10.1",
						EndIP:   "10.10.10.254",
						Gateway: "10.10.10.1",
						Exclusions: []pool.ExclusionRange{{
							StartIP: "10.10.10.10",
							EndIP:   "10.10.10.20",
						}},
						Routes:         []pool.Route{{Dst: "5.5.5.5/32"}},
						DefaultGateway: true,
					}))
			}).WithTimeout(time.Second * 15).WithPolling(time.Second).Should(Succeed())
		})
		It("Valid pool config /32 cidr", func() {
			p := &ipamv1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: testNamespace},
				Spec: ipamv1alpha1.CIDRPoolSpec{
					CIDR:                 "10.10.10.0/24",
					PerNodeNetworkPrefix: 32,
				},
			}
			Expect(k8sClient.Create(ctx, p)).NotTo(HaveOccurred())
			p.Status.Allocations = []ipamv1alpha1.CIDRPoolAllocation{{
				NodeName: testNodeName,
				Prefix:   "10.10.10.12/32",
			}}
			Expect(k8sClient.Status().Update(ctx, p)).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				g.Expect(poolManager.GetPoolByKey(
					common.GetPoolKey("test-pool", common.PoolTypeCIDRPool))).To(
					Equal(&pool.Pool{
						Name:       "test-pool",
						Subnet:     "10.10.10.12/32",
						StartIP:    "10.10.10.12",
						EndIP:      "10.10.10.12",
						Exclusions: []pool.ExclusionRange{},
						Routes:     []pool.Route{},
					}))
			}).WithTimeout(time.Second * 15).WithPolling(time.Second).Should(Succeed())
		})
		It("Update Valid config with invalid", func() {
			p := &ipamv1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: testNamespace},
				Spec: ipamv1alpha1.CIDRPoolSpec{
					CIDR:                 "10.10.0.0/16",
					GatewayIndex:         ptr.To[int32](1),
					PerNodeNetworkPrefix: 24,
				},
			}
			Expect(k8sClient.Create(ctx, p)).NotTo(HaveOccurred())
			p.Status.Allocations = []ipamv1alpha1.CIDRPoolAllocation{{
				NodeName: testNodeName,
				Gateway:  "10.10.10.1",
				Prefix:   "10.10.10.0/24",
			}}
			Expect(k8sClient.Status().Update(ctx, p)).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				g.Expect(poolManager.GetPoolByKey(
					common.GetPoolKey("test-pool", common.PoolTypeCIDRPool))).To(
					Equal(&pool.Pool{
						Name:       "test-pool",
						Subnet:     "10.10.10.0/24",
						StartIP:    "10.10.10.1",
						EndIP:      "10.10.10.254",
						Gateway:    "10.10.10.1",
						Exclusions: []pool.ExclusionRange{},
						Routes:     []pool.Route{},
					}))
			}).WithTimeout(time.Second * 15).WithPolling(time.Second).Should(Succeed())
			p.Spec.GatewayIndex = ptr.To[int32](10)
			Expect(k8sClient.Update(ctx, p)).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				g.Expect(poolManager.GetPoolByKey(common.GetPoolKey("test-pool", common.PoolTypeCIDRPool))).To(BeNil())
			}).WithTimeout(time.Second * 15).WithPolling(time.Second).Should(Succeed())
		})
	})
})
