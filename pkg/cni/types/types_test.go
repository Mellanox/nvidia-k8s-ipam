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

package types_test

import (
	"fmt"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
)

var _ = Describe("Types Tests", func() {
	var (
		tmpDir      string
		testConfDir string
	)

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
		testConfDir = path.Join(tmpDir, "nv-ipam.d")
		err := os.Mkdir(testConfDir, 0o755)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(path.Join(testConfDir, types.K8sNodeNameFile), []byte("test-node"), 0o644)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("LoadConf()", func() {
		It("loads default configuration when no overwrites provided", func() {
			// write empty config file
			err := os.WriteFile(path.Join(testConfDir, types.ConfFileName), []byte("{}"), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// write kubeconfig file
			data, err := os.ReadFile(path.Join("..", "..", "..", "testdata", "test.kubeconfig"))
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(path.Join(testConfDir, types.DefaultKubeConfigFileName), []byte(data), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// Load config
			testConf := fmt.Sprintf(`{"name": "my-net", "ipam": {"confDir": %q}}`, testConfDir)
			conf, err := types.NewConfLoader().LoadConf([]byte(testConf))

			// Validate
			Expect(err).ToNot(HaveOccurred())
			Expect(conf.IPAM.PoolName).To(Equal(conf.Name))
			Expect(conf.IPAM.ConfDir).To(Equal(testConfDir))
			Expect(conf.IPAM.DataDir).To(Equal(types.DefaultDataDir))
			Expect(conf.IPAM.LogFile).To(Equal(types.DefaultLogFile))
			Expect(conf.IPAM.LogLevel).To(Equal("info"))
			Expect(conf.IPAM.Kubeconfig).To(Equal(path.Join(conf.IPAM.ConfDir, types.DefaultKubeConfigFileName)))
		})

		It("overwrites configuration from file", func() {
			// write config file
			confData := fmt.Sprintf(`
			{"logLevel": "debug", "logFile": "some/path.log", "dataDir": "some/data/path",
			"kubeconfig": "%s/alternate.kubeconfig"}`, testConfDir)
			err := os.WriteFile(path.Join(testConfDir, types.ConfFileName), []byte(confData), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// write kubeconfig file
			data, err := os.ReadFile(path.Join("..", "..", "..", "testdata", "test.kubeconfig"))
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(path.Join(testConfDir, "alternate.kubeconfig"), []byte(data), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// Load config
			testConf := fmt.Sprintf(`{"name": "my-net", "ipam": {"confDir": %q}}`, testConfDir)
			conf, err := types.NewConfLoader().LoadConf([]byte(testConf))

			// Validate
			Expect(err).ToNot(HaveOccurred())
			Expect(conf.IPAM.ConfDir).To(Equal(testConfDir))
			Expect(conf.IPAM.DataDir).To(Equal("some/data/path"))
			Expect(conf.IPAM.LogFile).To(Equal("some/path.log"))
			Expect(conf.IPAM.LogLevel).To(Equal("debug"))
			Expect(conf.IPAM.Kubeconfig).To(Equal(path.Join(conf.IPAM.ConfDir, "alternate.kubeconfig")))
		})

		It("overwrites configuration from json input", func() {
			// write config file
			confData := `{"logLevel": "debug", "dataDir": "some/data/path"}`
			err := os.WriteFile(path.Join(testConfDir, types.ConfFileName), []byte(confData), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// write kubeconfig file
			data, err := os.ReadFile(path.Join("..", "..", "..", "testdata", "test.kubeconfig"))
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(path.Join(testConfDir, types.DefaultKubeConfigFileName), []byte(data), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// Load config
			testConf := fmt.Sprintf(`{"name": "my-net", "ipam": {"confDir": %q, "poolName": "my-pool", "logLevel": "error"}}`, testConfDir)
			conf, err := types.NewConfLoader().LoadConf([]byte(testConf))

			// Validate
			Expect(err).ToNot(HaveOccurred())
			Expect(conf.IPAM.PoolName).To(Equal("my-pool"))
			Expect(conf.IPAM.ConfDir).To(Equal(testConfDir))
			Expect(conf.IPAM.DataDir).To(Equal("some/data/path"))
			Expect(conf.IPAM.LogFile).To(Equal(types.DefaultLogFile))
			Expect(conf.IPAM.LogLevel).To(Equal("error"))
			Expect(conf.IPAM.Kubeconfig).To(Equal(path.Join(conf.IPAM.ConfDir, types.DefaultKubeConfigFileName)))
		})

		It("Fails if config is invalid json", func() {
			_, err := types.NewConfLoader().LoadConf([]byte("{garbage%^&*"))
			Expect(err).To(HaveOccurred())
		})

		It("Fails if config does not contain ipam key", func() {
			_, err := types.NewConfLoader().LoadConf([]byte(`{"name": "my-net", "type": "sriov"}`))
			Expect(err).To(HaveOccurred())
		})
	})
})
