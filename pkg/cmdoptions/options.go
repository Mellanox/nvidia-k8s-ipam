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

package cmdoptions

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
)

func New() *Options {
	return &Options{
		FeatureGate: featuregate.NewFeatureGate(),
		LogConfig:   logsapi.NewLoggingConfiguration(),
	}
}

// Options holds shared command line options
type Options struct {
	FeatureGate featuregate.MutableFeatureGate
	LogConfig   *logsapi.LoggingConfiguration
}

// AddNamedFlagSets register flags for common options in NamedFlagSets
func (o *Options) AddNamedFlagSets(sharedFS *cliflag.NamedFlagSets) {
	logFS := sharedFS.FlagSet("Logging")
	logsapi.AddFlags(o.LogConfig, logFS)
	logs.AddFlags(logFS, logs.SkipLoggingConfigurationFlags())
	utilruntime.Must(logsapi.AddFeatureGates(o.FeatureGate))

	commonFS := sharedFS.FlagSet("Common")
	o.FeatureGate.AddFlag(commonFS)
	commonFS.Bool("version", false, "print binary version and exit")
}

// Validate registered options
func (o *Options) Validate() error {
	return logsapi.ValidateAndApply(o.LogConfig, o.FeatureGate)
}
