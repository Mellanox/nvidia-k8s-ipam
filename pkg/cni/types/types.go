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
	// DefaultConfDir is the default dir where configurations are found
	DefaultConfDir = "/etc/cni/net.d/nv-ipam.d"
	// DefaultDataDir is the default dir where cni stores data and binaries
	DefaultDataDir = "/var/lib/cni/nv-ipam"
	// DefaultLogFile default log file path to be used for logging
	DefaultLogFile = "/var/log/nv-ipam-cni.log"

	// HostLocalDataDir is the relative path within the data dir for host-local state data
	HostLocalDataDir = "state/host-local"
	// K8sNodeNameFile is the file name containing k8s node name
	K8sNodeNameFile = "k8s-node-name"
	// DefaultKubeConfigFileName is the default name of kubeconfig file
	DefaultKubeConfigFileName = "nv-ipam.kubeconfig"
	// ConfFileName is the name of CNI configuration file found in conf dir
	ConfFileName = "nv-ipam.conf"
)

// IPAMConf is the configuration supported by our CNI plugin
type IPAMConf struct {
	types.IPAM

	// PoolName is the name of the pool to be used to allocate IP
	PoolName   string `json:"poolName,omitempty"`
	Kubeconfig string `json:"kubeconfig,omitempty"`
	DataDir    string `json:"dataDir,omitempty"`
	ConfDir    string `json:"confDir,omitempty"`
	LogFile    string `json:"logFile,omitempty"`
	LogLevel   string `json:"logLevel,omitempty"`

	// internal configuration
	NodeName  string
	K8sClient *kubernetes.Clientset
}

// NetConf is CNI network config
type NetConf struct {
	Name       string    `json:"name"`
	CNIVersion string    `json:"cniVersion"`
	IPAM       *IPAMConf `json:"ipam"`
}

// LoadConf Loads NetConf from json string provided as []byte
func LoadConf(bytes []byte) (*NetConf, error) {
	n := &NetConf{}

	if err := json.Unmarshal(bytes, &n); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configurations. %w", err)
	}

	if n.IPAM == nil {
		return nil, fmt.Errorf("IPAM config missing 'ipam' key")
	}

	if n.IPAM.ConfDir == "" {
		n.IPAM.ConfDir = DefaultConfDir
	}

	// overlay config from conf file if exists.
	confFilePath := filepath.Join(n.IPAM.ConfDir, ConfFileName)
	fileConf, err := LoadFromConfFile(confFilePath)
	if err == nil {
		overlayConf(fileConf, n.IPAM)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read/parse config file(%s). %w", confFilePath, err)
	}

	// overlay config with defaults
	defaultConf := &IPAMConf{
		// use network name as pool name by default
		PoolName:   n.Name,
		Kubeconfig: filepath.Join(n.IPAM.ConfDir, DefaultKubeConfigFileName),
		DataDir:    DefaultDataDir,
		ConfDir:    DefaultConfDir,
		LogFile:    DefaultLogFile,
		LogLevel:   "info",
	}
	overlayConf(defaultConf, n.IPAM)

	// get Node name
	p := filepath.Join(n.IPAM.ConfDir, K8sNodeNameFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read k8s node name from path: %s. %w", p, err)
	}
	n.IPAM.NodeName = strings.TrimSpace(string(data))
	if n.IPAM.NodeName == "" {
		return nil, fmt.Errorf("failed to parse k8s node name from path: %s", p)
	}

	// create k8s client
	n.IPAM.K8sClient, err = k8sclient.FromKubeconfig(n.IPAM.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client from kubeconfig path: %s. %w", n.IPAM.Kubeconfig, err)
	}

	return n, nil
}

// LoadFromConfFile returns *IPAMConf with values from config file located in filePath.
func LoadFromConfFile(filePath string) (*IPAMConf, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	confFromFile := &IPAMConf{}
	err = json.Unmarshal(data, confFromFile)
	if err != nil {
		return nil, err
	}

	return confFromFile, nil
}

// overlayConf overlays IPAMConf "from" onto "to"
// fields in to are overlayed if they are empty in "to".
func overlayConf(from, to *IPAMConf) {
	if to.ConfDir == "" {
		to.ConfDir = from.ConfDir
	}

	if to.DataDir == "" {
		to.DataDir = from.DataDir
	}

	if to.Kubeconfig == "" {
		to.Kubeconfig = from.Kubeconfig
	}

	if to.LogFile == "" {
		to.LogFile = from.LogFile
	}

	if to.LogLevel == "" {
		to.LogLevel = from.LogLevel
	}

	if to.PoolName == "" {
		to.PoolName = from.PoolName
	}
}
