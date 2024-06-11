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
	"net"
	"os"
	"path"

	"github.com/containernetworking/cni/pkg/skel"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cniTypes "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
)

var _ = Describe("Types Tests", func() {
	var (
		tmpDir      string
		testConfDir string
		testArgs    = "K8S_POD_NAME=test;K8S_POD_NAMESPACE=test"
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
			conf, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{
				StdinData: []byte(testConf), Args: testArgs})

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
			conf, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{
				StdinData: []byte(testConf), Args: testArgs})

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
			conf, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{
				StdinData: []byte(testConf), Args: testArgs})

			// Validate
			Expect(err).ToNot(HaveOccurred())
			Expect(conf.IPAM.PoolName).To(Equal("my-pool"))
			Expect(conf.IPAM.ConfDir).To(Equal(testConfDir))
			Expect(conf.IPAM.LogFile).To(Equal(cniTypes.DefaultLogFile))
			Expect(conf.IPAM.LogLevel).To(Equal("error"))
		})

		It("Fails if config is invalid json", func() {
			_, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{
				StdinData: []byte("{garbage%^&*"), Args: testArgs})
			Expect(err).To(HaveOccurred())
		})

		It("Fails if config does not contain ipam key", func() {
			_, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{
				StdinData: []byte(`{"name": "my-net", "type": "sriov"}`), Args: testArgs})
			Expect(err).To(HaveOccurred())
		})

		It("Missing metadata arguments", func() {
			// write config file
			confData := `{"logLevel": "debug", "logFile": "some/path.log"}`
			Expect(os.WriteFile(
				path.Join(testConfDir, cniTypes.ConfFileName), []byte(confData), 0o644)).NotTo(HaveOccurred())
			// Load config
			testConf := fmt.Sprintf(`{"name": "my-net", "ipam": {"confDir": %q}}`, testConfDir)
			_, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{StdinData: []byte(testConf), Args: "K8S_POD_NAME=test"})
			Expect(err).To(MatchError(ContainSubstring("K8S_POD_NAMESPACE")))
			_, err = cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{
				StdinData: []byte(testConf), Args: "K8S_POD_NAMESPACE=test"})
			Expect(err).To(MatchError(ContainSubstring("K8S_POD_NAME")))
		})
		DescribeTable("Static IPs",
			func(stdinContent string, args string, expectedValue []net.IP, isValid bool) {
				Expect(os.WriteFile(path.Join(testConfDir, cniTypes.ConfFileName),
					[]byte("{}"), 0o644)).NotTo(HaveOccurred())
				stdinContent = fmt.Sprintf(stdinContent, testConfDir)
				if args == "" {
					args = testArgs
				} else {
					args = testArgs + ";" + args
				}
				conf, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{
					StdinData: []byte(stdinContent), Args: args})
				if isValid {
					Expect(err).NotTo(HaveOccurred())
					Expect(conf.IPAM.RequestedIPs).To(Equal(expectedValue))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("no IPs by default", `{"name": "my-net", "ipam": {"confDir": %q}}`, "", nil, true),
			Entry("env variable", `{"name": "my-net", "ipam": {"confDir": %q}}`,
				"IP=192.168.1.1",
				[]net.IP{net.ParseIP("192.168.1.1")}, true),
			Entry("STDIN CNI args", `{"name": "my-net", 
				"args": {"cni": {"ips": ["1.1.1.1", "2.2.2.2/24"]}}, "ipam": {"confDir": %q}}`, "",
				[]net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("2.2.2.2")}, true),
			Entry("STDIN RuntimeConfig", `{"name": "my-net", 
				"runtimeConfig": {"ips": ["1.1.1.1", "2.2.2.2/24"]}, "ipam": {"confDir": %q}}`, "",
				[]net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("2.2.2.2")}, true),
			Entry("ipv6", `{"name": "my-net", 
				"runtimeConfig": {"ips": ["fd52:2eb5:44::1"]}, "ipam": {"confDir": %q}}`, "",
				[]net.IP{net.ParseIP("fd52:2eb5:44::1")}, true),
			Entry("invalid ip", `{"name": "my-net", 
				"runtimeConfig": {"ips": ["adfdsaf"]}, "ipam": {"confDir": %q}}`, "",
				nil, false),
		)
		DescribeTable("PoolName",
			func(fileContent string, stdinContent string, expectedValue []string, isValid bool) {
				Expect(os.WriteFile(path.Join(testConfDir, cniTypes.ConfFileName),
					[]byte(fileContent), 0o644)).NotTo(HaveOccurred())
				stdinContent = fmt.Sprintf(stdinContent, testConfDir)
				conf, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{StdinData: []byte(stdinContent), Args: testArgs})
				if isValid {
					Expect(err).NotTo(HaveOccurred())
					Expect(conf.IPAM.Pools).To(Equal(expectedValue))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("use network name if not set",
				`{}`,
				`{"name": "my-net", "ipam": {"confDir": %q}}`,
				[]string{"my-net"}, true),
			Entry("from conf file",
				`{"poolName": "fromFile"}`,
				`{"name": "my-net", "ipam": {"confDir": %q}}`,
				[]string{"fromFile"}, true),
			Entry("from STDIN",
				`{"poolName": "fromFile"}`,
				`{"name": "my-net", "ipam": {"confDir": %q, "poolName": "fromSTDIN"}}`,
				[]string{"fromSTDIN"}, true),
			Entry("from STDIN CNI args",
				`{"poolName": "fromFile"}`,
				`{"name": "my-net", "args":{"cni": {"poolNames": ["fromArgs", "fromArgs2"]}}, 
							"ipam": {"confDir": %q, "poolName": "fromSTDIN"}}`,
				[]string{"fromArgs", "fromArgs2"}, true),
			Entry("too many pools",
				`{}`,
				`{"name": "my-net", "ipam": {"confDir": %q, "poolName": "pool1,pool2,pool3"}}`,
				[]string{}, false),
		)
		DescribeTable("PoolType",
			func(fileContent string, stdinContent string, expectedValue string, isValid bool) {
				Expect(os.WriteFile(path.Join(testConfDir, cniTypes.ConfFileName),
					[]byte(fileContent), 0o644)).NotTo(HaveOccurred())
				stdinContent = fmt.Sprintf(stdinContent, testConfDir)
				conf, err := cniTypes.NewConfLoader().LoadConf(&skel.CmdArgs{StdinData: []byte(stdinContent), Args: testArgs})
				if isValid {
					Expect(err).NotTo(HaveOccurred())
					Expect(conf.IPAM.PoolType).To(Equal(expectedValue))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("use IPPool by default", `{}`, `{"name": "my-net", "ipam": {"confDir": %q}}`, "ippool", true),
			Entry("from conf file", `{"poolType": "CIDRPool"}`, `{"name": "my-net", "ipam": {"confDir": %q}}`, "cidrpool", true),
			Entry("from STDIN", `{}`, `{"name": "my-net", "ipam": {"confDir": %q, "poolType": "cidrPool"}}`, "cidrpool", true),
			Entry("from STDIN CNI Args", `{}`, `{"name": "my-net", 
				"args": {"cni": {"poolType":"cidrpool"}}, "ipam": {"confDir": %q}}`, "cidrpool", true),
			Entry("unknown type", `{}`, `{"name": "my-net", "ipam": {"confDir": %q, "poolType": "foobar"}}`, "", false),
		)
	})
})
