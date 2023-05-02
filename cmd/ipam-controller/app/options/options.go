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

	cliflag "k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cmdoptions"
)

// New initialize and return new Options object
func New() *Options {
	return &Options{
		Options:              *cmdoptions.New(),
		MetricsAddr:          ":8080",
		ProbeAddr:            ":8081",
		EnableLeaderElection: false,
		ConfigMapName:        "nvidia-k8s-ipam-config",
		ConfigMapNamespace:   "kube-system",
	}
}

// Options holds command line options for controller
type Options struct {
	cmdoptions.Options
	MetricsAddr          string
	ProbeAddr            string
	EnableLeaderElection bool
	ConfigMapName        string
	ConfigMapNamespace   string
}

// AddNamedFlagSets register flags for common options in NamedFlagSets
func (o *Options) AddNamedFlagSets(sharedFS *cliflag.NamedFlagSets) {
	o.Options.AddNamedFlagSets(sharedFS)

	controllerFS := sharedFS.FlagSet("Controller")

	goFS := goflag.NewFlagSet("tmp", goflag.ContinueOnError)
	ctrl.RegisterFlags(goFS)
	controllerFS.AddGoFlagSet(goFS)

	controllerFS.StringVar(&o.MetricsAddr, "metrics-bind-address", o.MetricsAddr,
		"The address the metric endpoint binds to.")
	controllerFS.StringVar(&o.ProbeAddr, "health-probe-bind-address",
		o.ProbeAddr, "The address the probe endpoint binds to.")
	controllerFS.BoolVar(&o.EnableLeaderElection, "leader-elect", o.EnableLeaderElection,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	controllerFS.StringVar(&o.ConfigMapName, "config-name",
		o.ConfigMapName, "The name of the ConfigMap which holds controller configuration")
	controllerFS.StringVar(&o.ConfigMapNamespace, "config-namespace",
		o.ConfigMapNamespace, "The name of the namespace where ConfigMap with controller configuration exist")
}

// Validate registered options
func (o *Options) Validate() error {
	return o.Options.Validate()
}
