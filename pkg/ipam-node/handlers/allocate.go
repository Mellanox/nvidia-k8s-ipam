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

package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	cniTypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

// Allocate is the handler for Allocate GRPC endpoint
func (h *Handlers) Allocate(ctx context.Context, req *nodev1.AllocateRequest) (*nodev1.AllocateResponse, error) {
	reqLog := addFieldsToLogger(logr.FromContextOrDiscard(ctx), req)
	ctx = logr.NewContext(ctx, reqLog)

	if err := validateReq(req); err != nil {
		return nil, err
	}
	params := setDefaultsToParams(req.Parameters)
	store, err := h.openStore(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkReqIsCanceled(ctx); err != nil {
		return nil, h.closeSession(ctx, store, err)
	}
	result, err := h.allocate(reqLog, store, params)
	if err := h.closeSession(ctx, store, err); err != nil {
		return nil, err
	}
	resp := &nodev1.AllocateResponse{}
	for _, r := range result {
		allocationInfo := &nodev1.AllocationInfo{
			Pool:     r.Pool,
			Ip:       r.Address.String(),
			PoolType: params.PoolType,
		}
		if r.Gateway != nil {
			allocationInfo.Gateway = r.Gateway.String()
		}
		resp.Allocations = append(resp.Allocations, allocationInfo)
	}
	return resp, nil
}

// PoolAlloc container which store Pool name and allocation
type PoolAlloc struct {
	Pool string
	*current.IPConfig
}

func (h *Handlers) allocate(reqLog logr.Logger,
	session storePkg.Session, params *nodev1.IPAMParameters) ([]PoolAlloc, error) {
	var err error
	result := make([]PoolAlloc, 0, len(params.Pools))
	requestedIPs := make([]net.IP, 0, len(params.RequestedIps))
	for _, ipAddr := range params.RequestedIps {
		// already validated by validateReq func
		requestedIPs = append(requestedIPs, net.ParseIP(ipAddr))
	}
	for _, poolName := range params.Pools {
		var alloc PoolAlloc
		alloc, err = h.allocateInPool(poolName, reqLog, session, params, requestedIPs)
		if err != nil {
			break
		}
		result = append(result, alloc)
	}
	if err != nil {
		return nil, err
	}
	// check that all requested static IPs were allocated
	for _, ipAddr := range requestedIPs {
		found := false
		for _, r := range result {
			if r.Address.IP.Equal(ipAddr) {
				found = true
				break
			}
		}
		if !found {
			return nil, status.Errorf(codes.InvalidArgument, "not all requested static IPs can be allocated"+
				" from the ranges available on the node, ip %s has no matching Pool", ipAddr.String())
		}
	}

	return result, nil
}

func (h *Handlers) allocateInPool(poolName string, reqLog logr.Logger,
	session storePkg.Session, params *nodev1.IPAMParameters, requestedIPs []net.IP) (PoolAlloc, error) {
	poolType := poolTypeAsString(params.PoolType)
	poolLog := reqLog.WithValues("pool", poolName, "poolType", poolType)
	poolKey := common.GetPoolKey(poolName, poolType)

	poolCfg := h.poolConfReader.GetPoolByKey(poolKey)
	if poolCfg == nil {
		return PoolAlloc{}, status.Errorf(codes.NotFound,
			"configuration for pool \"%s\", poolType \"%s\" not found", poolName, poolType)
	}
	rangeStart := net.ParseIP(poolCfg.StartIP)
	if rangeStart == nil {
		return PoolAlloc{}, poolCfgError(poolLog, poolName, poolType, "invalid rangeStart")
	}
	rangeEnd := net.ParseIP(poolCfg.EndIP)
	if rangeEnd == nil {
		return PoolAlloc{}, poolCfgError(poolLog, poolName, poolType, "invalid rangeEnd")
	}
	_, subnet, err := net.ParseCIDR(poolCfg.Subnet)
	if err != nil || subnet == nil || subnet.IP == nil || subnet.Mask == nil {
		return PoolAlloc{}, poolCfgError(poolLog, poolName, poolType, "invalid subnet")
	}
	gateway := net.ParseIP(poolCfg.Gateway)
	rangeSet := &allocator.RangeSet{allocator.Range{
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
		Subnet:     cniTypes.IPNet(*subnet),
		Gateway:    gateway,
	}}
	if err := rangeSet.Canonicalize(); err != nil {
		return PoolAlloc{}, poolCfgError(poolLog, poolName, poolType,
			fmt.Sprintf("invalid range config: %s", err.Error()))
	}
	var selectedStaticIP net.IP
	for _, ipAddr := range requestedIPs {
		if subnet.Contains(ipAddr) {
			selectedStaticIP = ipAddr
			break
		}
	}

	if params.Features != nil && params.Features.AllocateDefaultGateway {
		if gateway == nil {
			return PoolAlloc{}, status.Errorf(codes.InvalidArgument,
				"pool without gateway can't be used with allocate_default_gateway feature,"+
					"pool \"%s\", poolType \"%s\"", poolName, poolType)
		}
		selectedStaticIP = gateway
	}

	exclusionRangeSet := make(allocator.RangeSet, 0, len(poolCfg.Exclusions))
	for _, e := range poolCfg.Exclusions {
		exclusionRangeSet = append(exclusionRangeSet, allocator.Range{
			Subnet:     cniTypes.IPNet(*subnet),
			RangeStart: net.ParseIP(e.StartIP),
			RangeEnd:   net.ParseIP(e.EndIP),
		})
	}
	if len(exclusionRangeSet) > 0 {
		if err := exclusionRangeSet.Canonicalize(); err != nil {
			return PoolAlloc{}, poolCfgError(poolLog, poolName, poolType,
				fmt.Sprintf("invalid exclusion range config: %s", err.Error()))
		}
	}
	alloc := h.getAllocFunc(rangeSet, &exclusionRangeSet, poolKey, session)
	allocMeta := types.ReservationMetadata{
		CreateTime:         time.Now().Format(time.RFC3339Nano),
		PoolConfigSnapshot: poolCfg.String(),
	}
	if params.Metadata != nil {
		allocMeta.PodUUID = params.Metadata.K8SPodUid
		allocMeta.PodName = params.Metadata.K8SPodName
		allocMeta.PodNamespace = params.Metadata.K8SPodNamespace
		allocMeta.DeviceID = params.Metadata.DeviceId
	}
	result, err := alloc.Allocate(params.CniContainerid, params.CniIfname, allocMeta, selectedStaticIP)
	if err != nil {
		poolLog.Error(err, "failed to allocate IP address")
		if errors.Is(err, storePkg.ErrReservationAlreadyExist) {
			return PoolAlloc{}, status.Errorf(codes.AlreadyExists,
				"allocation already exist in the pool \"%s\", poolType \"%s\"", poolName, poolType)
		}
		if errors.Is(err, allocator.ErrNoFreeAddresses) {
			return PoolAlloc{}, status.Errorf(codes.ResourceExhausted,
				"no free addresses in the pool \"%s\", poolType \"%s\"",
				poolName, poolType)
		}
		if errors.Is(err, storePkg.ErrIPAlreadyReserved) {
			return PoolAlloc{}, status.Errorf(codes.ResourceExhausted,
				"requested IP is already reserved in the pool \"%s\", poolType \"%s\"",
				poolName, poolType)
		}
		return PoolAlloc{}, status.Errorf(codes.Internal,
			"failed to allocate IP address in pool \"%s\", poolType \"%s\"", poolName, poolType)
	}
	if params.Features != nil && params.Features.AllocateDefaultGateway {
		// TODO (ykulazhenkov): do we want to keep gateway in this case?
		// if we will return gateway here, the container will have same IP as interface address and as gateway
		result.Gateway = nil
	}

	poolLog.Info("IP address allocated", "allocation", result.String())
	return PoolAlloc{
		Pool:     poolName,
		IPConfig: result,
	}, nil
}

func poolCfgError(reqLog logr.Logger, pool, poolType, reason string) error {
	reqLog.Error(nil, "invalid pool config", "pool", pool, "poolType", poolType,
		"reason", reason)
	return status.Errorf(codes.Internal, "invalid config for pool \"%s\", poolType \"%s\"", pool, poolType)
}
