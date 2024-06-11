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
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
)

const (
	// DefaultConfDir is the default dir where configurations are found
	DefaultConfDir = "/etc/cni/net.d/nv-ipam.d"
	// DefaultDaemonSocket is the default socket path for the daemon
	DefaultDaemonSocket = "unix:///var/lib/cni/nv-ipam/daemon.sock"
	// DefaultDaemonCallTimeoutSeconds is the default timeout IPAM daemon calls
	DefaultDaemonCallTimeoutSeconds = 5
	// DefaultLogFile default log file path to be used for logging
	DefaultLogFile = "/var/log/nv-ipam-cni.log"
	// DefaultLogLevel is the default log level
	DefaultLogLevel = "info"
	// ConfFileName is the name of CNI configuration file found in conf dir
	ConfFileName = "nv-ipam.conf"
)

// ConfLoader loads CNI configuration
//
//go:generate mockery --name ConfLoader
type ConfLoader interface {
	// LoadConf loads configuration from CNI CmdArgs
	LoadConf(args *skel.CmdArgs) (*NetConf, error)
}

// IPAMConf is the configuration supported by our CNI plugin
type IPAMConf struct {
	types.IPAM

	// PoolName is the name of the pool to be used to allocate IP
	PoolName string `json:"poolName,omitempty"`
	// PoolType is the type of the pool which is referred by the PoolName,
	// supported values: ippool, cidrpool
	PoolType string `json:"poolType,omitempty"`
	// Address of the NVIDIA-ipam DaemonSocket
	DaemonSocket             string `json:"daemonSocket,omitempty"`
	DaemonCallTimeoutSeconds int    `json:"daemonCallTimeoutSeconds,omitempty"`
	ConfDir                  string `json:"confDir,omitempty"`
	LogFile                  string `json:"logFile,omitempty"`
	LogLevel                 string `json:"logLevel,omitempty"`

	// internal fields
	// holds processed data from poolName field
	Pools []string `json:"-"`
	// k8s metadata parsed from CNI_ARGS
	K8SMetadata struct {
		PodName      string
		PodNamespace string
		PodUID       string
	} `json:"-"`
	// requested IPs from CNI_ARGS, args and capabilities
	RequestedIPs []net.IP `json:"-"`
}

// NetConf is CNI network config
type NetConf struct {
	Name          string    `json:"name"`
	CNIVersion    string    `json:"cniVersion"`
	IPAM          *IPAMConf `json:"ipam"`
	DeviceID      string    `json:"deviceID"`
	RuntimeConfig struct {
		IPs []string `json:"ips,omitempty"`
	} `json:"runtimeConfig,omitempty"`
	Args *struct {
		ArgsCNI *IPAMArgs `json:"cni"`
	} `json:"args"`
}

// IPAMArgs holds arguments from stdin args["cni"]
type IPAMArgs struct {
	IPs       []string `json:"ips"`
	PoolNames []string `json:"poolNames"`
	PoolType  string   `json:"poolType"`
}

// IPAMEnvArgs holds arguments from CNI_ARGS env variable
type IPAMEnvArgs struct {
	types.CommonArgs
	IP                types.UnmarshallableString
	K8S_POD_NAME      types.UnmarshallableString //nolint
	K8S_POD_NAMESPACE types.UnmarshallableString //nolint
	K8S_POD_UID       types.UnmarshallableString //nolint
}

type confLoader struct{}

func NewConfLoader() ConfLoader {
	return &confLoader{}
}

// LoadConf Loads NetConf from json string provided as []byte
func (cl *confLoader) LoadConf(args *skel.CmdArgs) (*NetConf, error) {
	n := &NetConf{}

	if err := json.Unmarshal(args.StdinData, &n); err != nil {
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
	fileConf, err := cl.loadFromConfFile(confFilePath)
	if err == nil {
		cl.overlayConf(fileConf, n.IPAM)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read/parse config file(%s). %w", confFilePath, err)
	}

	// overlay config with defaults
	defaultConf := &IPAMConf{
		// use network name as pool name by default
		PoolName:                 n.Name,
		PoolType:                 common.PoolTypeIPPool,
		ConfDir:                  DefaultConfDir,
		LogFile:                  DefaultLogFile,
		DaemonSocket:             DefaultDaemonSocket,
		DaemonCallTimeoutSeconds: DefaultDaemonCallTimeoutSeconds,
		LogLevel:                 DefaultLogLevel,
	}
	cl.overlayConf(defaultConf, n.IPAM)

	// static IP address priority:
	// stdin runtimeConfig["ips"] > stdin args["cni"]["ips"] > IP argument from CNI_ARGS env variable
	requestedIPs := n.RuntimeConfig.IPs
	if len(requestedIPs) == 0 {
		if n.Args != nil && n.Args.ArgsCNI != nil {
			requestedIPs = n.Args.ArgsCNI.IPs
		}
	}
	for _, v := range requestedIPs {
		ip := parseIP(v)
		if ip == nil {
			return nil, fmt.Errorf("static IP request contains invalid IP address")
		}
		n.IPAM.RequestedIPs = append(n.IPAM.RequestedIPs, ip)
	}

	if err := cl.loadEnvCNIArgs(n, args); err != nil {
		return nil, err
	}

	n.IPAM.Pools, err = getPools(n)
	if err != nil {
		return nil, err
	}
	n.IPAM.PoolType, err = getPoolType(n)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// loads arguments from CNI_ARGS env variable
func (cl *confLoader) loadEnvCNIArgs(conf *NetConf, args *skel.CmdArgs) error {
	envArgs := &IPAMEnvArgs{}
	err := types.LoadArgs(args.Args, envArgs)
	if err != nil {
		return err
	}
	if envArgs.K8S_POD_NAME == "" {
		return fmt.Errorf("CNI_ARGS: K8S_POD_NAME is not provided by container runtime")
	}
	if envArgs.K8S_POD_NAMESPACE == "" {
		return fmt.Errorf("CNI_ARGS: K8S_POD_NAMESPACE is not provided by container runtime")
	}

	conf.IPAM.K8SMetadata.PodName = string(envArgs.K8S_POD_NAME)
	conf.IPAM.K8SMetadata.PodNamespace = string(envArgs.K8S_POD_NAMESPACE)
	conf.IPAM.K8SMetadata.PodUID = string(envArgs.K8S_POD_UID)

	// use IP argument from CNI_ARGS env only if IPs are not configured by other methods
	if len(conf.IPAM.RequestedIPs) > 0 || envArgs.IP == "" {
		return nil
	}
	parsedIP := parseIP(string(envArgs.IP))
	if parsedIP == nil {
		return fmt.Errorf("CNI_ARGS: IP argument contains invalid IP address")
	}
	conf.IPAM.RequestedIPs = append(conf.IPAM.RequestedIPs, parsedIP)
	return nil
}

func parseIP(s string) net.IP {
	s, _, _ = strings.Cut(s, "/")
	return net.ParseIP(s)
}

// returns list of pools that should be used by the plugin.
// STDIN field "args[cni][poolNames]" has the highest priority
func getPools(n *NetConf) ([]string, error) {
	var pools []string
	if n.Args != nil && len(n.Args.ArgsCNI.PoolNames) > 0 {
		pools = n.Args.ArgsCNI.PoolNames
	} else {
		pools = strings.Split(n.IPAM.PoolName, ",")
	}
	if len(pools) > 2 {
		return nil, fmt.Errorf("pool field can't contain more then two entries")
	}
	for _, p := range pools {
		if p == "" {
			return nil, fmt.Errorf("pool field has invalid format")
		}
	}
	return pools, nil
}

// returns poolType that should be used by the plugin.
// STDIN field "args[cni][poolType]" has the highest priority
func getPoolType(n *NetConf) (string, error) {
	var poolType string
	if n.Args != nil && n.Args.ArgsCNI.PoolType != "" {
		poolType = n.Args.ArgsCNI.PoolType
	} else {
		poolType = n.IPAM.PoolType
	}
	poolType = strings.ToLower(poolType)
	if poolType != common.PoolTypeIPPool && poolType != common.PoolTypeCIDRPool {
		return "", fmt.Errorf("unsupported poolType %s, supported values: %s, %s",
			poolType, common.PoolTypeIPPool, common.PoolTypeCIDRPool)
	}
	return poolType, nil
}

// loadFromConfFile returns *IPAMConf with values from config file located in filePath.
func (cl *confLoader) loadFromConfFile(filePath string) (*IPAMConf, error) {
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
func (cl *confLoader) overlayConf(from, to *IPAMConf) {
	if to.ConfDir == "" {
		to.ConfDir = from.ConfDir
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

	if to.PoolType == "" {
		to.PoolType = from.PoolType
	}

	if to.DaemonSocket == "" {
		to.DaemonSocket = from.DaemonSocket
	}

	if to.DaemonCallTimeoutSeconds == 0 {
		to.DaemonCallTimeoutSeconds = from.DaemonCallTimeoutSeconds
	}
}
