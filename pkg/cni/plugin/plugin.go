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
	"fmt"
	"net"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	log "github.com/k8snetworkplumbingwg/cni-log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/version"
)

const (
	CNIPluginName = "nv-ipam"
)

// GRPCClient is an interface for the client which is used to communicate with NVIDIA IPAM Node Daemon
//
//go:generate mockery --name GRPCClient

type GRPCClient interface {
	nodev1.IPAMServiceClient
}

type NewGRPCClientFunc func(daemonSocket string) (GRPCClient, error)

func NewPlugin() *Plugin {
	return &Plugin{
		Name:              CNIPluginName,
		Version:           version.GetVersionString(),
		ConfLoader:        types.NewConfLoader(),
		NewGRPCClientFunc: defaultNewGRPCClientFunc,
	}
}

type Plugin struct {
	Name              string
	Version           string
	ConfLoader        types.ConfLoader
	NewGRPCClientFunc NewGRPCClientFunc
}

func (p *Plugin) CmdAdd(args *skel.CmdArgs) error {
	cmd, err := p.prepareCMD(args)
	if err != nil {
		return log.Errorf("command preparation failed: %v", err)
	}
	logCall("ADD", args, cmd.Config.IPAM)
	ctx, cFunc := context.WithTimeout(context.Background(),
		time.Second*time.Duration(cmd.Config.IPAM.DaemonCallTimeoutSeconds))
	defer cFunc()
	resp, err := cmd.Client.Allocate(ctx, &nodev1.AllocateRequest{Parameters: cmd.ReqParams})
	if err != nil {
		return log.Errorf("grpc call failed: %v", err)
	}
	result, err := grpcRespToResult(resp)
	if err != nil {
		return err
	}
	log.Infof("CmdAdd succeed")
	return cnitypes.PrintResult(result, cmd.Config.CNIVersion)
}

func (p *Plugin) CmdDel(args *skel.CmdArgs) error {
	cmd, err := p.prepareCMD(args)
	if err != nil {
		return err
	}
	logCall("DEL", args, cmd.Config.IPAM)
	ctx, cFunc := context.WithTimeout(context.Background(),
		time.Second*time.Duration(cmd.Config.IPAM.DaemonCallTimeoutSeconds))
	defer cFunc()
	if _, err := cmd.Client.Deallocate(ctx, &nodev1.DeallocateRequest{Parameters: cmd.ReqParams}); err != nil {
		return log.Errorf("grpc call failed: %v", err)
	}
	log.Infof("CmdDel succeed")
	return nil
}

func (p *Plugin) CmdCheck(args *skel.CmdArgs) error {
	cmd, err := p.prepareCMD(args)
	if err != nil {
		return err
	}
	logCall("CHECK", args, cmd.Config.IPAM)
	ctx, cFunc := context.WithTimeout(context.Background(),
		time.Second*time.Duration(cmd.Config.IPAM.DaemonCallTimeoutSeconds))
	defer cFunc()
	if _, err := cmd.Client.IsAllocated(ctx, &nodev1.IsAllocatedRequest{Parameters: cmd.ReqParams}); err != nil {
		return log.Errorf("grpc call failed: %v", err)
	}
	log.Infof("CmdCheck succeed")
	return nil
}

type cmdContext struct {
	Client    GRPCClient
	Config    *types.NetConf
	ReqParams *nodev1.IPAMParameters
}

func (p *Plugin) prepareCMD(args *skel.CmdArgs) (cmdContext, error) {
	var (
		c   cmdContext
		err error
	)
	c.Config, err = p.ConfLoader.LoadConf(args.StdinData)
	if err != nil {
		return cmdContext{}, fmt.Errorf("failed to load config. %v", err)
	}
	setupLog(c.Config.IPAM.LogFile, c.Config.IPAM.LogLevel)

	c.Client, err = p.NewGRPCClientFunc(c.Config.IPAM.DaemonSocket)
	if err != nil {
		return cmdContext{}, fmt.Errorf("failed to connect to IPAM daemon: %v", err)
	}
	c.ReqParams, err = cniConfToGRPCReq(c.Config, args)
	if err != nil {
		return cmdContext{}, fmt.Errorf("failed to convert CNI parameters to GRPC request: %v", err)
	}
	return c, nil
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

func grpcRespToResult(resp *nodev1.AllocateResponse) (*current.Result, error) {
	result := &current.Result{CNIVersion: current.ImplementedSpecVersion}
	logErr := func(msg string) error {
		return log.Errorf("unexpected response from IPAM daemon: %s", msg)
	}
	for _, alloc := range resp.Allocations {
		if alloc.Ip == "" {
			return nil, logErr("IP can't be empty")
		}
		if alloc.Gateway == "" {
			return nil, logErr("Gateway can't be empty")
		}
		ipAddr, netAddr, err := net.ParseCIDR(alloc.Ip)
		if err != nil {
			return nil, logErr(fmt.Sprintf("unexpected IP address format, received value: %s", alloc.Ip))
		}
		gwIP := net.ParseIP(alloc.Gateway)
		if gwIP == nil {
			return nil, logErr(fmt.Sprintf("unexpected Gateway address format, received value: %s", alloc.Gateway))
		}
		result.IPs = append(result.IPs, &current.IPConfig{
			Address: net.IPNet{IP: ipAddr, Mask: netAddr.Mask},
			Gateway: gwIP,
		})
	}

	return result, nil
}

func cniConfToGRPCReq(conf *types.NetConf, args *skel.CmdArgs) (*nodev1.IPAMParameters, error) {
	cniExtraArgs := &kubernetesCNIArgs{}
	err := cnitypes.LoadArgs(args.Args, cniExtraArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to load extra CNI args: %v", err)
	}
	req := &nodev1.IPAMParameters{
		Pools:          conf.IPAM.Pools,
		CniIfname:      args.IfName,
		CniContainerid: args.ContainerID,
		Metadata: &nodev1.IPAMMetadata{
			K8SPodName:      string(cniExtraArgs.K8S_POD_NAME),
			K8SPodNamespace: string(cniExtraArgs.K8S_POD_NAMESPACE),
			K8SPodUid:       string(cniExtraArgs.K8S_POD_UID),
			DeviceId:        conf.DeviceID,
		},
	}

	if req.Metadata.K8SPodName == "" {
		return nil, log.Errorf("CNI_ARGS: K8S_POD_NAME is not provided by container runtime")
	}
	if req.Metadata.K8SPodNamespace == "" {
		return nil, log.Errorf("CNI_ARGS: K8S_POD_NAMESPACE is not provided by container runtime")
	}
	if req.Metadata.K8SPodUid == "" {
		log.Warningf("CNI_ARGS: K8S_POD_UID is not provided by container runtime")
	}
	return req, nil
}

// default NewGRPCClientFunc, initializes insecure GRPC connection to provided daemon socket
func defaultNewGRPCClientFunc(daemonSocket string) (GRPCClient, error) {
	conn, err := grpc.Dial(daemonSocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return nodev1.NewIPAMServiceClient(conn), nil
}

// kubernetesCNIArgs is the container for extra CNI Args which set by container runtimes
// in Kubernetes
type kubernetesCNIArgs struct {
	cnitypes.CommonArgs
	K8S_POD_NAME      cnitypes.UnmarshallableString //nolint
	K8S_POD_NAMESPACE cnitypes.UnmarshallableString //nolint
	K8S_POD_UID       cnitypes.UnmarshallableString //nolint
}
