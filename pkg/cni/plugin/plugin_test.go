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
	"path"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/stretchr/testify/mock"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/plugin"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/version"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pluginMocks "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/plugin/mocks"
	typesMocks "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types/mocks"
)

var _ = Describe("plugin tests", func() {
	var (
		tmpDir           string
		p                plugin.Plugin
		mockConfLoader   *typesMocks.ConfLoader
		mockDaemonClient *pluginMocks.GRPCClient
		testConf         *types.NetConf
		args             *skel.CmdArgs
	)

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
		mockConfLoader = typesMocks.NewConfLoader(GinkgoT())
		mockDaemonClient = pluginMocks.NewGRPCClient(GinkgoT())

		p = plugin.Plugin{
			Name:       plugin.CNIPluginName,
			Version:    version.GetVersionString(),
			ConfLoader: mockConfLoader,
			NewGRPCClientFunc: func(_ string) (plugin.GRPCClient, error) {
				return mockDaemonClient, nil
			},
		}

		testConf = &types.NetConf{
			Name:       "my-net",
			CNIVersion: "0.4.0",
			IPAM: &types.IPAMConf{
				IPAM: cnitypes.IPAM{
					Type: "nv-ipam",
				},
				Pools:    []string{"my-pool"},
				LogFile:  path.Join(tmpDir, "nv-ipam.log"),
				LogLevel: "debug",
				K8SMetadata: struct {
					PodName      string
					PodNamespace string
					PodUID       string
				}{
					PodName:      "test",
					PodNamespace: "test",
				},
			},
		}

		args = &skel.CmdArgs{
			ContainerID: "1234",
			Netns:       "/proc/19783/ns",
			IfName:      "net1",
			StdinData:   []byte("doesnt-matter"),
			Args:        "K8S_POD_NAME=test;K8S_POD_NAMESPACE=test",
		}
	})

	Context("CmdAdd()", func() {
		It("executes successfully", func() {
			mockConfLoader.On("LoadConf", args).Return(testConf, nil)
			mockDaemonClient.On("Allocate", mock.Anything, &nodev1.AllocateRequest{
				Parameters: &nodev1.IPAMParameters{
					Pools:          []string{"my-pool"},
					PoolType:       nodev1.PoolType_POOL_TYPE_IPPOOL,
					CniIfname:      "net1",
					CniContainerid: "1234",
					Metadata: &nodev1.IPAMMetadata{
						K8SPodName:      "test",
						K8SPodNamespace: "test",
					},
					RequestedIps: []string{},
					Features:     &nodev1.IPAMFeatures{},
				}}).Return(&nodev1.AllocateResponse{
				Allocations: []*nodev1.AllocationInfo{{
					Pool:     "my-pool",
					Ip:       "192.168.1.10/24",
					Gateway:  "192.168.1.1",
					PoolType: nodev1.PoolType_POOL_TYPE_IPPOOL,
				}},
			}, nil)
			err := p.CmdAdd(args)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("CmdDel()", func() {
		It("executes successfully", func() {
			mockConfLoader.On("LoadConf", args).Return(testConf, nil)
			mockDaemonClient.On("Deallocate", mock.Anything, mock.Anything).Return(nil, nil)
			err := p.CmdDel(args)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("CmdCheck()", func() {
		It("executes successfully", func() {
			mockConfLoader.On("LoadConf", args).Return(testConf, nil)
			mockDaemonClient.On("IsAllocated", mock.Anything, mock.Anything).Return(nil, nil)
			err := p.CmdCheck(args)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
