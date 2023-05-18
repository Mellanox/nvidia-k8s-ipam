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

package plugin_test

import (
	"encoding/json"
	"path"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	cnitypes100 "github.com/containernetworking/cni/pkg/types/100"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/plugin"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/version"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	pluginMocks "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/plugin/mocks"
	typesMocks "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types/mocks"
)

var _ = Describe("plugin tests", func() {
	var (
		tmpDir         string
		p              plugin.Plugin
		fakeClient     *fake.Clientset
		testNode       *v1.Node
		mockExecutor   *pluginMocks.IPAMExecutor
		mockConfLoader *typesMocks.ConfLoader
		testConf       *types.NetConf
		args           *skel.CmdArgs
	)

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()

		testNode = &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		}
		nodeAnnot := map[string]string{
			pool.IPBlocksAnnotation: `{"my-pool":
			{"subnet": "192.168.0.0/16", "startIP": "192.168.0.2",
			"endIP": "192.168.0.254", "gateway": "192.168.0.1"}}`,
		}
		testNode.SetAnnotations(nodeAnnot)

		fakeClient = fake.NewSimpleClientset(testNode)
		mockExecutor = pluginMocks.NewIPAMExecutor(GinkgoT())
		mockConfLoader = typesMocks.NewConfLoader(GinkgoT())

		p = plugin.Plugin{
			Name:         plugin.CNIPluginName,
			Version:      version.GetVersionString(),
			ConfLoader:   mockConfLoader,
			IPAMExecutor: mockExecutor,
		}

		testConf = &types.NetConf{
			Name:       "my-net",
			CNIVersion: "0.4.0",
			IPAM: &types.IPAMConf{
				IPAM: cnitypes.IPAM{
					Type: "nv-ipam",
				},
				PoolName:  "my-pool",
				DataDir:   "/foo/bar",
				LogFile:   path.Join(tmpDir, "nv-ipam.log"),
				LogLevel:  "debug",
				NodeName:  "test-node",
				K8sClient: fakeClient,
			},
		}

		args = &skel.CmdArgs{
			ContainerID: "1234",
			Netns:       "/proc/19783/ns",
			IfName:      "net1",
			StdinData:   []byte("doesnt-matter"),
		}
	})

	Context("CmdAdd()", func() {
		It("executes successfully", func() {
			mockConfLoader.On("LoadConf", args.StdinData).Return(testConf, nil)
			mockExecutor.On("ExecAdd", "host-local", mock.Anything).Return(&cnitypes100.Result{}, func(_ string, data []byte) error {
				// fail if we cannot unmarshal data to host-local config
				hostLocalConf := &types.HostLocalNetConf{}
				return json.Unmarshal(data, hostLocalConf)
			})

			err := p.CmdAdd(args)
			Expect(err).ToNot(HaveOccurred())
			mockConfLoader.AssertExpectations(GinkgoT())
			mockExecutor.AssertExpectations(GinkgoT())
		})
	})

	Context("CmdDel()", func() {
		It("executes successfully", func() {
			mockConfLoader.On("LoadConf", args.StdinData).Return(testConf, nil)
			mockExecutor.On("ExecDel", "host-local", mock.Anything).Return(func(_ string, data []byte) error {
				// fail if we cannot unmarshal data to host-local config
				hostLocalConf := &types.HostLocalNetConf{}
				return json.Unmarshal(data, hostLocalConf)
			})

			err := p.CmdDel(args)
			Expect(err).ToNot(HaveOccurred())
			mockConfLoader.AssertExpectations(GinkgoT())
			mockExecutor.AssertExpectations(GinkgoT())
		})
	})

	Context("CmdCheck()", func() {
		It("executes successfully", func() {
			mockConfLoader.On("LoadConf", args.StdinData).Return(testConf, nil)
			mockExecutor.On("ExecCheck", "host-local", mock.Anything).Return(func(_ string, data []byte) error {
				// fail if we cannot unmarshal data to host-local config
				hostLocalConf := &types.HostLocalNetConf{}
				return json.Unmarshal(data, hostLocalConf)
			})

			err := p.CmdCheck(args)
			Expect(err).ToNot(HaveOccurred())
			mockConfLoader.AssertExpectations(GinkgoT())
			mockExecutor.AssertExpectations(GinkgoT())
		})
	})
})
