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
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
)

var (
	cfg                        *rest.Config
	testClient                 client.Client
	testEnv                    *envtest.Environment
	nodeEventCh                chan event.GenericEvent
	testScheme                 *runtime.Scheme
	ctx, testManagerCancelFunc = context.WithCancel(ctrl.SetupSignalHandler())
)

const (
	testNamespace = "test-ns"
)

func TestCIDRPoolController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "CIDRPool controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "deploy", "crds")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	testScheme = runtime.NewScheme()
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	Expect(clientgoscheme.AddToScheme(testScheme)).To(Succeed())
	Expect(ipamv1alpha1.AddToScheme(testScheme)).To(Succeed())

	By("setting up and running the test reconciler")
	testManager, err := ctrl.NewManager(cfg,
		ctrl.Options{
			Scheme: testScheme,
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{testNamespace: {}},
			},
			// Set metrics server bind address to 0 to disable it.
			Metrics: server.Options{
				BindAddress: "0",
			}})
	Expect(err).ToNot(HaveOccurred())

	// Use the manager's client so field indexers are available
	testClient = testManager.GetClient()
	Expect(testClient).NotTo(BeNil())

	nodeEventCh = make(chan event.GenericEvent, 1)
	reconciler := &CIDRPoolReconciler{
		PoolsNamespace: "test-ns",
		NodeEventCh:    nodeEventCh,
		Client:         testClient,
		Scheme:         testManager.GetScheme(),
	}
	err = reconciler.SetupWithManager(testManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = testManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	// create test namespace
	Expect(testClient.Create(ctx, getTestNS())).To(Succeed())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if testManagerCancelFunc != nil {
		testManagerCancelFunc()
	}
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func getTestNS() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
}
