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

package plugin

import (
	"github.com/containernetworking/cni/pkg/skel"
	// "github.com/containernetworking/plugins/pkg/ipam"
	log "github.com/k8snetworkplumbingwg/cni-log"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/version"
)

const (
	CNIPluginName = "nv-ipam"
)

func NewPlugin() *Plugin {
	return &Plugin{
		Name:    CNIPluginName,
		Version: version.GetVersionString(),
	}
}

type Plugin struct {
	Name    string
	Version string
}

func (p *Plugin) CmdAdd(args *skel.CmdArgs) error {
	setupLog()
	log.Infof("CMD Add Called with args: %+v", args)
	log.Infof("CMD Add Stdin data: %s", string(args.StdinData))

	// TODO: Implement
	// err := os.Setenv("CNI_PATH", "path to host-local used by plugin")
	// if err != nil {
	// 	return err
	// }
	// res, err := ipam.ExecAdd("host-local", args.StdinData)
	// print results
	return nil
}

func (p *Plugin) CmdDel(args *skel.CmdArgs) error {
	setupLog()
	log.Infof("CMD Del Called with args: %+v", args)
	log.Infof("CMD Del Stdin data: %s", string(args.StdinData))
	// TODO: Implement
	// err := os.Setenv("CNI_PATH", "path to host-local used by plugin")
	// if err != nil {
	//	 return err
	// }
	// err = ipam.ExecDel("host-local", args.StdinData)
	return nil
}

func (p *Plugin) CmdCheck(args *skel.CmdArgs) error {
	setupLog()
	log.Infof("CMD Check Called with args: %+v", args)
	log.Infof("CMD Check Stdin data: %s", string(args.StdinData))
	// TODO: Implement
	// err := os.Setenv("CNI_PATH", "path to host-local used by plugin")
	// if err != nil {
	// 	return err
	// }
	// err = ipam.ExecCheck("host-local", args.StdinData)
	// return err
	return nil
}

func setupLog() {
	log.SetLogFile("/var/log/nv-ipam-cni.log")
	log.SetLogLevel(log.VerboseLevel)
	log.SetLogStderr(true)
}
