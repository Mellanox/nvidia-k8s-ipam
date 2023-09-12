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

package options

import (
	goflag "flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	cliflag "k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cmdoptions"
	cniTypes "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
)

const (
	// DefaultStoreFile contains path of the default store file
	DefaultStoreFile   = "/var/lib/cni/nv-ipam/store"
	DefaultBindAddress = cniTypes.DefaultDaemonSocket
)

// New initialize and return new Options object
func New() *Options {
	return &Options{
		Options:        *cmdoptions.New(),
		MetricsAddr:    ":8080",
		ProbeAddr:      ":8081",
		NodeName:       "",
		BindAddress:    DefaultBindAddress,
		StoreFile:      DefaultStoreFile,
		PoolsNamespace: "kube-system",
		// shim CNI parameters
		CNIBinDir:                   "/opt/cni/bin",
		CNIBinFile:                  "/nv-ipam",
		CNISkipBinFileCopy:          false,
		CNISkipConfigCreation:       false,
		CNIDaemonSocket:             cniTypes.DefaultDaemonSocket,
		CNIDaemonCallTimeoutSeconds: 5,
		CNIConfDir:                  cniTypes.DefaultConfDir,
		CNILogLevel:                 cniTypes.DefaultLogLevel,
		CNILogFile:                  cniTypes.DefaultLogFile,
	}
}

// Options holds command line options for controller
type Options struct {
	cmdoptions.Options
	MetricsAddr    string
	ProbeAddr      string
	NodeName       string
	PoolsNamespace string
	BindAddress    string
	StoreFile      string
	// shim CNI parameters
	CNIBinDir                   string
	CNIBinFile                  string
	CNISkipBinFileCopy          bool
	CNISkipConfigCreation       bool
	CNIConfDir                  string
	CNIDaemonSocket             string
	CNIDaemonCallTimeoutSeconds int
	CNILogFile                  string
	CNILogLevel                 string
}

// AddNamedFlagSets register flags for common options in NamedFlagSets
func (o *Options) AddNamedFlagSets(sharedFS *cliflag.NamedFlagSets) {
	o.Options.AddNamedFlagSets(sharedFS)

	daemonFS := sharedFS.FlagSet("Node daemon")

	goFS := goflag.NewFlagSet("tmp", goflag.ContinueOnError)
	ctrl.RegisterFlags(goFS)
	daemonFS.AddGoFlagSet(goFS)

	daemonFS.StringVar(&o.MetricsAddr, "metrics-bind-address", o.MetricsAddr,
		"The address the metric endpoint binds to.")
	daemonFS.StringVar(&o.ProbeAddr, "health-probe-bind-address",
		o.ProbeAddr, "The address the probe endpoint binds to.")
	daemonFS.StringVar(&o.NodeName, "node-name",
		o.NodeName, "The name of the Node on which the daemon runs")
	daemonFS.StringVar(&o.PoolsNamespace, "ippools-namespace",
		o.PoolsNamespace, "The name of the namespace to watch for IPPools CRs")
	daemonFS.StringVar(&o.BindAddress, "bind-address", o.BindAddress,
		"GPRC server bind address. e.g.: tcp://127.0.0.1:9092, unix:///var/lib/foo")
	daemonFS.StringVar(&o.StoreFile, "store-file", o.StoreFile,
		"Path of the file which used to store allocations")

	cniFS := sharedFS.FlagSet("Shim CNI Configuration")
	cniFS.StringVar(&o.CNIBinDir,
		"cni-bin-dir", o.CNIBinDir, "CNI binary directory")
	cniFS.StringVar(&o.CNIBinFile,
		"cni-nv-ipam-bin-file", o.CNIBinFile, "nv-ipam binary file path")
	cniFS.BoolVar(&o.CNISkipBinFileCopy,
		"cni-skip-nv-ipam-binary-copy", o.CNISkipBinFileCopy, "skip nv-ipam binary file copy")
	cniFS.BoolVar(&o.CNISkipConfigCreation,
		"cni-skip-nv-ipam-config-creation", o.CNISkipConfigCreation, "skip config file creation for nv-ipam CNI")
	cniFS.StringVar(&o.CNIConfDir, "cni-conf-dir", o.CNIConfDir,
		"shim CNI config: path with config file")
	cniFS.StringVar(&o.CNIDaemonSocket, "cni-daemon-socket", o.CNIDaemonSocket,
		"shim CNI config: IPAM daemon socket path")
	cniFS.IntVar(&o.CNIDaemonCallTimeoutSeconds, "cni-daemon-call-timeout", o.CNIDaemonCallTimeoutSeconds,
		"shim CNI config: timeout for IPAM daemon calls")
	cniFS.StringVar(&o.CNILogFile, "cni-log-file", o.CNILogFile,
		"shim CNI config: path to log file for shim CNI")
	cniFS.StringVar(&o.CNILogLevel, "cni-log-level", o.CNILogLevel,
		"shim CNI config: log level for shim CNI")
}

// Validate registered options
func (o *Options) Validate() error {
	if len(o.NodeName) == 0 {
		return fmt.Errorf("node-name is required parameter")
	}
	if len(o.PoolsNamespace) == 0 {
		return fmt.Errorf("ippools-namespace is required parameter")
	}
	_, _, err := ParseBindAddress(o.BindAddress)
	if err != nil {
		return fmt.Errorf("bind-address is invalid: %v", err)
	}
	if err := o.verifyPaths(); err != nil {
		return err
	}
	return o.Options.Validate()
}

func (o *Options) verifyPaths() error {
	_, err := os.Stat(filepath.Dir(o.StoreFile))
	if err != nil {
		return fmt.Errorf("dir for store-file is not found: %v", err)
	}
	if !o.CNISkipBinFileCopy {
		// CNIBinFile
		if _, err := os.Stat(o.CNIBinFile); err != nil {
			return fmt.Errorf("cni-nv-ipam-bin-file is not found: %v", err)
		}
		// CNIBinDir
		if _, err := os.Stat(o.CNIBinDir); err != nil {
			return fmt.Errorf("cni-bin-dir is not found: %v", err)
		}
	}
	if !o.CNISkipConfigCreation {
		// CNIConfDir
		if _, err := os.Stat(o.CNIConfDir); err != nil {
			return fmt.Errorf("cni-conf-dir is not found: %v", err)
		}
	}
	// parent dir for CNI log file
	if _, err := os.Stat(filepath.Dir(o.CNILogFile)); err != nil {
		return fmt.Errorf("cni-log-file is not found: %v", err)
	}
	return nil
}

// ParseBindAddress parses bind-address and return network and address part separately,
// returns error if bind-address format is invalid
func ParseBindAddress(addr string) (string, string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", err
	}
	switch u.Scheme {
	case "tcp":
		return u.Scheme, u.Host, nil
	case "unix":
		return u.Scheme, u.Host + u.Path, nil
	default:
		return "", "", fmt.Errorf("unsupported scheme")
	}
}
