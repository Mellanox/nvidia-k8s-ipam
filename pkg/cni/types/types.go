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

package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/k8sclient"
)

const (
	DefaultConfDir = "/etc/cni/net.d/nv-ipam.d"
	DefaultDataDir = "/var/lib/cni/nv-ipam"
	DefaultLogFile = "/var/log/nv-ipam-cni.log"

	HostLocalDataDir          = "state/host-local"
	K8sNodeNameFile           = "k8s-node-name"
	DefaultKubeConfigFileName = "nv-ipam.kubeconfig"
)

type IPAMConf struct {
	types.IPAM

	// PoolName is the name of the pool to be used to allocate IP
	PoolName   string `json:"poolName"`
	Kubeconfig string `json:"kubeconfig,omitempty"`
	DataDir    string `json:"dataDir,omitempty"`
	ConfDir    string `json:"confDir,omitempty"`
	LogFile    string `json:"logFile,omitempty"`
	LogLevel   string `json:"logLevel,omitempty"`

	// internal configuration
	NodeName  string
	K8sClient *kubernetes.Clientset
}

type NetConf struct {
	Name       string    `json:"name"`
	CNIVersion string    `json:"cniVersion"`
	IPAM       *IPAMConf `json:"ipam"`
}

func LoadConf(bytes []byte) (*NetConf, error) {
	n := &NetConf{}

	if err := json.Unmarshal(bytes, &n); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configurations: %w", err)
	}

	if n.IPAM == nil {
		return nil, fmt.Errorf("IPAM config missing 'ipam' key")
	}

	if n.IPAM.ConfDir == "" {
		n.IPAM.ConfDir = DefaultConfDir
	}

	if n.IPAM.DataDir == "" {
		n.IPAM.DataDir = DefaultDataDir
	}

	if n.IPAM.Kubeconfig == "" {
		n.IPAM.Kubeconfig = filepath.Join(n.IPAM.ConfDir, DefaultKubeConfigFileName)
	}

	if n.IPAM.LogFile == "" {
		n.IPAM.LogFile = DefaultLogFile
	}

	if n.IPAM.PoolName == "" {
		n.IPAM.PoolName = n.Name
	}

	p := filepath.Join(n.IPAM.ConfDir, K8sNodeNameFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read k8s node name from path: %s. %w", p, err)
	}
	n.IPAM.NodeName = strings.TrimSpace(string(data))
	if n.IPAM.NodeName == "" {
		return nil, fmt.Errorf("failed to parse k8s node name from path: %s", p)
	}

	n.IPAM.K8sClient, err = k8sclient.FromKubeconfig(n.IPAM.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client from kubeconfig path: %s. %w", n.IPAM.Kubeconfig, err)
	}

	return n, nil
}
