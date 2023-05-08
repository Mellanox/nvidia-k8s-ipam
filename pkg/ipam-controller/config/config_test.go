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

package config_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/config"
)

func getValidPool() config.PoolConfig {
	return config.PoolConfig{Gateway: "192.168.1.1", Subnet: "192.168.1.0/24", PerNodeBlockSize: 10}
}

var _ = Describe("Config", func() {
	It("Empty config", func() {
		cfg := &config.Config{}
		Expect(cfg.Validate()).To(HaveOccurred())
	})
	It("Valid config", func() {
		cfg := &config.Config{Pools: map[string]config.PoolConfig{"pool1": getValidPool()}}
		Expect(cfg.Validate()).NotTo(HaveOccurred())
	})
	It("Invalid nodeSelector", func() {
		cfg := &config.Config{Pools: map[string]config.PoolConfig{"pool1": getValidPool()}}
		cfg.NodeSelector = map[string]string{",-_invalid": ""}
		Expect(cfg.Validate()).To(HaveOccurred())
	})
	It("Invalid pool: invalid subnet", func() {
		poolConfig := getValidPool()
		poolConfig.Subnet = "aaaa"
		cfg := &config.Config{Pools: map[string]config.PoolConfig{"pool1": poolConfig}}
		Expect(cfg.Validate()).To(HaveOccurred())
	})
	It("Invalid pool: no gw", func() {
		poolConfig := getValidPool()
		poolConfig.Gateway = ""
		cfg := &config.Config{Pools: map[string]config.PoolConfig{"pool1": poolConfig}}
		Expect(cfg.Validate()).To(HaveOccurred())
	})
	It("Invalid pool: gw outside of the subnet", func() {
		poolConfig := getValidPool()
		poolConfig.Gateway = "10.10.10.1"
		cfg := &config.Config{Pools: map[string]config.PoolConfig{"pool1": poolConfig}}
		Expect(cfg.Validate()).To(HaveOccurred())
	})
	It("Invalid pool: subnet is too small", func() {
		poolConfig := getValidPool()
		poolConfig.PerNodeBlockSize = 255
		cfg := &config.Config{Pools: map[string]config.PoolConfig{"pool1": poolConfig}}
		Expect(cfg.Validate()).To(HaveOccurred())
	})
	It("Invalid pool: perNodeBlockSize too small", func() {
		poolConfig := getValidPool()
		poolConfig.PerNodeBlockSize = 1
		cfg := &config.Config{Pools: map[string]config.PoolConfig{"pool1": poolConfig}}
		Expect(cfg.Validate()).To(HaveOccurred())
	})
})
