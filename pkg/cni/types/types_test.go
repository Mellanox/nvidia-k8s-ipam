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

	cniTypes "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
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
		Expect(err).ToNot(HaveOccurred())
	})

	Context("LoadConf()", func() {
		It("loads default configuration when no overwrites provided", func() {
			// write empty config file
			err := os.WriteFile(path.Join(testConfDir, cniTypes.ConfFileName), []byte("{}"), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// Load config
			testConf := fmt.Sprintf(`{"name": "my-net", "ipam": {"confDir": %q}}`, testConfDir)
			conf, err := cniTypes.NewConfLoader().LoadConf([]byte(testConf))

			// Validate
			Expect(err).ToNot(HaveOccurred())
			Expect(conf.IPAM.PoolName).To(Equal(conf.Name))
			Expect(conf.IPAM.ConfDir).To(Equal(testConfDir))
			Expect(conf.IPAM.LogFile).To(Equal(cniTypes.DefaultLogFile))
			Expect(conf.IPAM.LogLevel).To(Equal("info"))
		})

		It("overwrites configuration from file", func() {
			// write config file
			confData := `{"logLevel": "debug", "logFile": "some/path.log"}`
			err := os.WriteFile(path.Join(testConfDir, cniTypes.ConfFileName), []byte(confData), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// Load config
			testConf := fmt.Sprintf(`{"name": "my-net", "ipam": {"confDir": %q}}`, testConfDir)
			conf, err := cniTypes.NewConfLoader().LoadConf([]byte(testConf))

			// Validate
			Expect(err).ToNot(HaveOccurred())
			Expect(conf.IPAM.ConfDir).To(Equal(testConfDir))
			Expect(conf.IPAM.LogFile).To(Equal("some/path.log"))
			Expect(conf.IPAM.LogLevel).To(Equal("debug"))
		})

		It("overwrites configuration from json input", func() {
			// write config file
			confData := `{"logLevel": "debug"}`
			err := os.WriteFile(path.Join(testConfDir, cniTypes.ConfFileName), []byte(confData), 0o644)
			Expect(err).ToNot(HaveOccurred())

			// Load config
			testConf := fmt.Sprintf(`{"name": "my-net", "ipam": {"confDir": %q, "poolName": "my-pool", "logLevel": "error"}}`, testConfDir)
			conf, err := cniTypes.NewConfLoader().LoadConf([]byte(testConf))

			// Validate
			Expect(err).ToNot(HaveOccurred())
			Expect(conf.IPAM.PoolName).To(Equal("my-pool"))
			Expect(conf.IPAM.ConfDir).To(Equal(testConfDir))
			Expect(conf.IPAM.LogFile).To(Equal(cniTypes.DefaultLogFile))
			Expect(conf.IPAM.LogLevel).To(Equal("error"))
		})

		It("Fails if config is invalid json", func() {
			_, err := cniTypes.NewConfLoader().LoadConf([]byte("{garbage%^&*"))
			Expect(err).To(HaveOccurred())
		})

		It("Fails if config does not contain ipam key", func() {
			_, err := cniTypes.NewConfLoader().LoadConf([]byte(`{"name": "my-net", "type": "sriov"}`))
			Expect(err).To(HaveOccurred())
		})
	})
})
