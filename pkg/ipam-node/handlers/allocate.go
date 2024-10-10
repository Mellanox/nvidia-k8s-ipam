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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	cniTypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
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
			routes := make([]*nodev1.Route, 0)
			for _, route := range r.Routes {
				routes = append(routes, &nodev1.Route{Dest: route.String()})
			}
			allocationInfo.Routes = routes
		}
		resp.Allocations = append(resp.Allocations, allocationInfo)
	}
	return resp, nil
}

// PoolAlloc container which store Pool name and allocation
type PoolAlloc struct {
	Pool string
	*current.IPConfig
	Routes []net.IPNet
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
	rangeSet, rangeStart, subnet, gateway, err := newRangeSetAndDependenciesForPoolCfg(poolCfg)
	if err != nil {
		return PoolAlloc{}, poolCfgError(poolLog, poolName, poolType, err.Error())
	}
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

	if params.Features != nil {
		if params.Features.AllocateDefaultGateway {
			if gateway == nil {
				return PoolAlloc{}, status.Errorf(codes.InvalidArgument,
					"pool without gateway can't be used with allocate_default_gateway feature,"+
						"pool \"%s\", poolType \"%s\"", poolName, poolType)
			}
			selectedStaticIP = gateway
		}

		if params.Features.AllocateIpWithIndex != nil {
			selectedIP := ip.NextIPWithOffset(rangeStart, int64(*params.Features.AllocateIpWithIndex))
			if selectedIP.Equal(gateway) {
				return PoolAlloc{}, status.Errorf(codes.InvalidArgument,
					"requested IP index \"%d\" is the actual gateway, "+
						"use allocate_default_gateway feature instead", *params.Features.AllocateIpWithIndex)
			}
			if !rangeSet.Contains(selectedIP) {
				return PoolAlloc{}, status.Errorf(codes.InvalidArgument,
					"requested IP index \"%d\" is outside of the given chunk", *params.Features.AllocateIpWithIndex)
			}
			selectedStaticIP = selectedIP
		}
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
	setAllocMetadata(&allocMeta, params.Metadata)
	result, err := alloc.Allocate(params.CniContainerid, params.CniIfname, allocMeta, selectedStaticIP)
	if err != nil {
		poolLog.Error(err, "failed to allocate IP address")
		return PoolAlloc{}, specificError(err, poolName, poolType)
	}
	if params.Features != nil && params.Features.AllocateDefaultGateway {
		// TODO (ykulazhenkov): do we want to keep gateway in this case?
		// if we will return gateway here, the container will have same IP as interface address and as gateway
		result.Gateway = nil
	}
	routes, err := getRoutes(result.Gateway, poolCfg.Routes, poolCfg.DefaultGateway)
	if err != nil {
		return PoolAlloc{}, err
	}
	poolLog.Info("IP address allocated", "allocation", result.String())
	return PoolAlloc{
		Pool:     poolName,
		IPConfig: result,
		Routes:   routes,
	}, nil
}

func newRangeSetAndDependenciesForPoolCfg(poolCfg *pool.Pool) (*allocator.RangeSet, net.IP, *net.IPNet, net.IP, error) {
	rangeStart := net.ParseIP(poolCfg.StartIP)
	if rangeStart == nil {
		return nil, nil, nil, nil, errors.New("invalid rangeStart")
	}
	rangeEnd := net.ParseIP(poolCfg.EndIP)
	if rangeEnd == nil {
		return nil, nil, nil, nil, errors.New("invalid rangeEnd")
	}
	_, subnet, err := net.ParseCIDR(poolCfg.Subnet)
	if err != nil || subnet == nil || subnet.IP == nil || subnet.Mask == nil {
		return nil, nil, nil, nil, errors.New("invalid subnet")
	}
	gateway := net.ParseIP(poolCfg.Gateway)
	rangeSet := &allocator.RangeSet{allocator.Range{
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
		Subnet:     cniTypes.IPNet(*subnet),
		Gateway:    gateway,
	}}
	return rangeSet, rangeStart, subnet, gateway, nil
}

func setAllocMetadata(allocMeta *types.ReservationMetadata, paramsMetadata *nodev1.IPAMMetadata) {
	if paramsMetadata != nil {
		allocMeta.PodUUID = paramsMetadata.K8SPodUid
		allocMeta.PodName = paramsMetadata.K8SPodName
		allocMeta.PodNamespace = paramsMetadata.K8SPodNamespace
		allocMeta.DeviceID = paramsMetadata.DeviceId
	}
}

func specificError(err error, poolName string, poolType string) error {
	if errors.Is(err, storePkg.ErrReservationAlreadyExist) {
		return status.Errorf(codes.AlreadyExists,
			"allocation already exist in the pool \"%s\", poolType \"%s\"", poolName, poolType)
	}
	if errors.Is(err, allocator.ErrNoFreeAddresses) {
		return status.Errorf(codes.ResourceExhausted,
			"no free addresses in the pool \"%s\", poolType \"%s\"",
			poolName, poolType)
	}
	if errors.Is(err, storePkg.ErrIPAlreadyReserved) {
		return status.Errorf(codes.ResourceExhausted,
			"requested IP is already reserved in the pool \"%s\", poolType \"%s\"",
			poolName, poolType)
	}
	return status.Errorf(codes.Internal,
		"failed to allocate IP address in pool \"%s\", poolType \"%s\"", poolName, poolType)
}

func poolCfgError(reqLog logr.Logger, pool, poolType, reason string) error {
	reqLog.Error(nil, "invalid pool config", "pool", pool, "poolType", poolType,
		"reason", reason)
	return status.Errorf(codes.Internal, "invalid config for pool \"%s\", poolType \"%s\"", pool, poolType)
}

func getRoutes(gateway net.IP, routesCfg []pool.Route, defaultGateway bool) ([]net.IPNet, error) {
	routes := make([]net.IPNet, 0)
	if gateway == nil {
		return routes, nil
	}
	for _, r := range routesCfg {
		_, ipNet, err := net.ParseCIDR(r.Dst)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument,
				"unexpected Route destination format, received value: %s, error: %s",
				r.Dst, err.Error())
		}
		routes = append(routes, *ipNet)
	}
	if defaultGateway {
		routes = append(routes, createDefaultRoute(gateway))
	}

	return sortAndDedupIPNets(routes), nil
}

func createDefaultRoute(gateway net.IP) net.IPNet {
	if gateway.To4() != nil {
		// IPv4 default route
		return net.IPNet{
			IP:   net.IPv4(0, 0, 0, 0),
			Mask: net.CIDRMask(0, 32), // /0 for IPv4
		}
	}
	return net.IPNet{
		IP:   net.IPv6zero,         // :: (IPv6 default)
		Mask: net.CIDRMask(0, 128), // /0 for IPv6
	}
}

// Function to sort and remove duplicates from a slice of net.IPNet
func sortAndDedupIPNets(slice []net.IPNet) []net.IPNet {
	if len(slice) == 0 {
		return slice
	}

	slices.SortFunc(slice, func(a, b net.IPNet) int {
		if !a.IP.Equal(b.IP) {
			return bytes.Compare(a.IP, b.IP) // Compare IP addresses
		}
		return bytes.Compare(a.Mask, b.Mask) // Compare subnet masks if IPs are equal
	})

	return slices.CompactFunc(slice, func(a, b net.IPNet) bool {
		return a.IP.Equal(b.IP) && bytes.Equal(a.Mask, b.Mask)
	})
}
