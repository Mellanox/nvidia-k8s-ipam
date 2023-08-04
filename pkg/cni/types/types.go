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
	// LoadConf loads CNI configuration from Json data
	LoadConf(bytes []byte) (*NetConf, error)
}

// IPAMConf is the configuration supported by our CNI plugin
type IPAMConf struct {
	types.IPAM

	// PoolName is the name of the pool to be used to allocate IP
	PoolName string `json:"poolName,omitempty"`
	// Address of the NVIDIA-ipam DaemonSocket
	DaemonSocket             string `json:"daemonSocket,omitempty"`
	DaemonCallTimeoutSeconds int    `json:"daemonCallTimeoutSeconds,omitempty"`
	ConfDir                  string `json:"confDir,omitempty"`
	LogFile                  string `json:"logFile,omitempty"`
	LogLevel                 string `json:"logLevel,omitempty"`

	// internal fields
	Pools []string `json:"-"`
}

// NetConf is CNI network config
type NetConf struct {
	Name       string    `json:"name"`
	CNIVersion string    `json:"cniVersion"`
	IPAM       *IPAMConf `json:"ipam"`
	DeviceID   string    `json:"deviceID"`
}

type confLoader struct{}

func NewConfLoader() ConfLoader {
	return &confLoader{}
}

// LoadConf Loads NetConf from json string provided as []byte
func (cl *confLoader) LoadConf(bytes []byte) (*NetConf, error) {
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
		ConfDir:                  DefaultConfDir,
		LogFile:                  DefaultLogFile,
		DaemonSocket:             DefaultDaemonSocket,
		DaemonCallTimeoutSeconds: DefaultDaemonCallTimeoutSeconds,
		LogLevel:                 DefaultLogLevel,
	}
	cl.overlayConf(defaultConf, n.IPAM)

	n.IPAM.Pools, err = parsePoolName(n.IPAM.PoolName)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func parsePoolName(poolName string) ([]string, error) {
	pools := strings.Split(poolName, ",")
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

	if to.DaemonSocket == "" {
		to.DaemonSocket = from.DaemonSocket
	}

	if to.DaemonCallTimeoutSeconds == 0 {
		to.DaemonCallTimeoutSeconds = from.DaemonCallTimeoutSeconds
	}
}
