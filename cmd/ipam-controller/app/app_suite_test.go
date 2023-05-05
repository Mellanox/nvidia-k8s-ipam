/*
 Copyright (c) 2022, NVIDIA CORPORATION & AFFILIATES. All rights reserved.

 NVIDIA CORPORATION, its affiliates and licensors retain all intellectual
 property and proprietary rights in and to this material, related
 documentation and any modifications thereto. Any use, reproduction,
 disclosure or distribution of this material and related documentation
 without an express license agreement from NVIDIA CORPORATION or
 its affiliates is strictly prohibited.

 SPDX-FileCopyrightText: Copyright (c) 2022 NVIDIA CORPORATION & AFFILIATES
 SPDX-License-Identifier: LicenseRef-NvidiaProprietary
*/

package app_test

import (
	"context"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-controller/app"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-controller/app/options"
)

const (
	TestNamespace     = "test-ns"
	TestConfigMapName = "test-config"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	cFunc     context.CancelFunc
	ctx       context.Context
	wg        sync.WaitGroup
)

func TestApp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IPAM Controller Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}

	ctx, cFunc = context.WithCancel(context.Background())

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: TestNamespace}})).To(BeNil())

	wg.Add(1)
	go func() {
		err := app.RunController(logr.NewContext(ctx, klog.NewKlogr()), cfg, &options.Options{
			ConfigMapName:      TestConfigMapName,
			ConfigMapNamespace: TestNamespace,
		})
		if err != nil {
			panic(err.Error())
		}
		wg.Done()
	}()
})

var _ = AfterSuite(func() {
	By("stop controller")
	cFunc()
	wg.Wait()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
