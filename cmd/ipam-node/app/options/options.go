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
	DefaultBindAddress = "unix://" + cniTypes.DefaultDaemonSocket
)

// New initialize and return new Options object
func New() *Options {
	return &Options{
		Options:     *cmdoptions.New(),
		MetricsAddr: ":8080",
		ProbeAddr:   ":8081",
		NodeName:    "",
		BindAddress: DefaultBindAddress,
		StoreFile:   DefaultStoreFile,
	}
}

// Options holds command line options for controller
type Options struct {
	cmdoptions.Options
	MetricsAddr string
	ProbeAddr   string
	NodeName    string
	BindAddress string
	StoreFile   string
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
	daemonFS.StringVar(&o.BindAddress, "bind-address", o.BindAddress,
		"GPRC server bind address. e.g.: tcp://127.0.0.1:9092, unix:///var/lib/foo")
	daemonFS.StringVar(&o.StoreFile, "store-file", o.StoreFile,
		"Path of the file which used to store allocations")
}

// Validate registered options
func (o *Options) Validate() error {
	if len(o.NodeName) == 0 {
		return fmt.Errorf("node-name is required parameter")
	}
	_, _, err := ParseBindAddress(o.BindAddress)
	if err != nil {
		return fmt.Errorf("bind-address is invalid: %v", err)
	}
	if len(o.StoreFile) == 0 {
		return fmt.Errorf("store-file can't be empty")
	}
	_, err = os.Stat(filepath.Dir(o.StoreFile))
	if err != nil {
		return fmt.Errorf("store-file is invalid: %v", err)
	}
	return o.Options.Validate()
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
