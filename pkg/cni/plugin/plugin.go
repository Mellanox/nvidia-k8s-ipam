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
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ipam"
	log "github.com/k8snetworkplumbingwg/cni-log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/pool"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/version"
)

const (
	CNIPluginName = "nv-ipam"

	delegateIPAMPluginName = "host-local"
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
		return fmt.Errorf("failed to load config. %w", err)
	}
	setupLog(conf.IPAM.LogFile, conf.IPAM.LogLevel)
	logCall("ADD", args, conf.IPAM)

	// build host-local config
	pool, err := getPoolbyName(conf.IPAM.K8sClient, conf.IPAM.NodeName, conf.IPAM.PoolName)
	if err != nil {
		return fmt.Errorf("failed to get pool by name. %w", err)
	}

	hlc := types.HostLocalNetConfFromNetConfAndPool(conf, pool)
	data, err := json.Marshal(hlc)
	if err != nil {
		return fmt.Errorf("failed to marshal host-local net conf. %w", err)
	}
	log.Debugf("host-local stdin data: %q", string(data))

	// call host-local cni with alternate path
	err = os.Setenv("CNI_PATH", filepath.Join(conf.IPAM.DataDir, "bin"))
	if err != nil {
		return err
	}
	res, err := ipam.ExecAdd(delegateIPAMPluginName, data)
	if err != nil {
		return fmt.Errorf("failed to exec ADD host-local CNI plugin. %w", err)
	}

	return cnitypes.PrintResult(res, conf.CNIVersion)
}

func (p *Plugin) CmdDel(args *skel.CmdArgs) error {
	conf, err := types.LoadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to load config. %w", err)
	}
	setupLog(conf.IPAM.LogFile, conf.IPAM.LogLevel)
	logCall("DEL", args, conf.IPAM)

	// build host-local config
	pool, err := getPoolbyName(conf.IPAM.K8sClient, conf.IPAM.NodeName, conf.IPAM.PoolName)
	if err != nil {
		return fmt.Errorf("failed to get pool by name. %w", err)
	}

	hlc := types.HostLocalNetConfFromNetConfAndPool(conf, pool)
	data, err := json.Marshal(hlc)
	if err != nil {
		return fmt.Errorf("failed to marshal host-local net conf. %w", err)
	}
	log.Debugf("host-local stdin data: %q", string(data))

	// call host-local cni with alternate path
	err = os.Setenv("CNI_PATH", filepath.Join(conf.IPAM.DataDir, "bin"))
	if err != nil {
		return err
	}
	err = ipam.ExecDel(delegateIPAMPluginName, data)
	if err != nil {
		return fmt.Errorf("failed to exec DEL host-local CNI plugin. %w", err)
	}

	return nil
}

func (p *Plugin) CmdCheck(args *skel.CmdArgs) error {
	conf, err := types.LoadConf(args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to load config. %w", err)
	}
	setupLog(conf.IPAM.LogFile, conf.IPAM.LogLevel)
	logCall("CHECK", args, conf.IPAM)

	// build host-local config
	pool, err := getPoolbyName(conf.IPAM.K8sClient, conf.IPAM.NodeName, conf.IPAM.PoolName)
	if err != nil {
		return fmt.Errorf("failed to get pool by name. %w", err)
	}

	hlc := types.HostLocalNetConfFromNetConfAndPool(conf, pool)
	data, err := json.Marshal(hlc)
	if err != nil {
		return fmt.Errorf("failed to marshal host-local net conf. %w", err)
	}
	log.Debugf("host-local stdin data: %q", string(data))

	// call host-local cni with alternate path
	err = os.Setenv("CNI_PATH", filepath.Join(conf.IPAM.DataDir, "bin"))
	if err != nil {
		return err
	}
	err = ipam.ExecCheck(delegateIPAMPluginName, data)
	if err != nil {
		return fmt.Errorf("failed to exec CHECK host-local CNI plugin. %w", err)
	}

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

func logCall(cmd string, args *skel.CmdArgs, conf *types.IPAMConf) {
	log.Infof("CMD %s Call: ContainerID: %s Netns: %s IfName: %s", cmd, args.ContainerID, args.Netns, args.IfName)
	log.Debugf("CMD %s: Args: %s StdinData: %q", cmd, args.Args, string(args.StdinData))
	log.Debugf("CMD %s: Parsed IPAM conf: %+v", cmd, conf)
}

func getPoolbyName(kclient *kubernetes.Clientset, nodeName, poolName string) (*pool.IPPool, error) {
	// get pool info from node
	node, err := kclient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s from k8s API. %w", nodeName, err)
	}

	pm, err := pool.NewManagerImpl(node)
	if err != nil {
		return nil, fmt.Errorf("failed to get pools from node %s. %w", nodeName, err)
	}

	pool := pm.GetPoolByName(poolName)
	if pool == nil {
		return nil, fmt.Errorf("failed to get pools from node %s. pool %s not found", nodeName, poolName)
	}
	return pool, nil
}
