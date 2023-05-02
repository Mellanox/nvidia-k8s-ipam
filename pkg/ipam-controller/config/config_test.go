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
