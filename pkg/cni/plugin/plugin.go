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
	"context"
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// "github.com/containernetworking/plugins/pkg/ipam"
	log "github.com/k8snetworkplumbingwg/cni-log"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/pool"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
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
	conf, err := types.LoadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	setupLog(conf.IPAM.LogFile, conf.IPAM.LogLevel)

	log.Infof("CMD Add Called with args: %+v", args)
	log.Infof("CMD Add Stdin data: %s", string(args.StdinData))

	// get pool info from node
	node, err := conf.IPAM.K8sClient.CoreV1().Nodes().Get(context.TODO(), conf.IPAM.NodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s from k8s API. %w", conf.IPAM.NodeName, err)
	}

	pm, err := pool.NewManagerImpl(node)
	if err != nil {
		return fmt.Errorf("failed to get pools from node %s. %w", conf.IPAM.NodeName, err)
	}

	pool := pm.GetPoolByName(conf.IPAM.PoolName)
	if pool == nil {
		return fmt.Errorf("failed to get pools from node %s. pool %s not found", conf.IPAM.NodeName, conf.IPAM.PoolName)
	}

	// build host-local config
	hlc := types.HostLocalNetConfFromNetConfAndPool(conf, pool)
	data, err := json.Marshal(hlc)
	if err != nil {
		return fmt.Errorf("failed to marshal host-local net conf. %w", err)
	}

	log.Infof("host-local stdin data:%s", string(data))

	// call host-local cni with alternate path

	// print results

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
	conf, err := types.LoadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	setupLog(conf.IPAM.LogFile, conf.IPAM.LogLevel)

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
	conf, err := types.LoadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	setupLog(conf.IPAM.LogFile, conf.IPAM.LogLevel)

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

func setupLog(logFile, logLevel string) {
	if logLevel != "" {
		l := log.StringToLevel(logLevel)
		log.SetLogLevel(l)
	}

	if logFile != "" {
		log.SetLogFile(logFile)
	}
}
