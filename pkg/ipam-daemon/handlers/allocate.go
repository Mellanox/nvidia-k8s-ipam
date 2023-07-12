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
	"net"
	"time"

	cniTypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	daemonv1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/daemon/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/allocator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/types"
)

// Allocate is the handler for Allocate GRPC endpoint
func (h *Handlers) Allocate(ctx context.Context, req *daemonv1.AllocateRequest) (*daemonv1.AllocateResponse, error) {
	reqLog := addFieldsToLogger(klog.FromContext(ctx), req)
	ctx = klog.NewContext(ctx, reqLog)

	if err := validateReq(req); err != nil {
		return nil, err
	}
	params := req.Parameters
	store, err := h.openStore(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkReqISCanceled(ctx); err != nil {
		return nil, h.closeStore(ctx, store, err)
	}
	result, err := h.allocate(reqLog, store, params)
	if err := h.closeStore(ctx, store, err); err != nil {
		return nil, err
	}
	resp := &daemonv1.AllocateResponse{}
	for _, r := range result {
		resp.Allocations = append(resp.Allocations, &daemonv1.AllocationInfo{
			Pool:    r.Pool,
			Ip:      r.Address.String(),
			Gateway: r.Gateway.String(),
		})
	}
	return resp, nil
}

// PoolAlloc container which store Pool name and allocation
type PoolAlloc struct {
	Pool string
	*current.IPConfig
}

func (h *Handlers) allocate(reqLog logr.Logger,
	store storePkg.Store, params *daemonv1.IPAMParameters) ([]PoolAlloc, error) {
	var err error
	result := make([]PoolAlloc, 0, len(params.Pools))
	for _, pool := range params.Pools {
		var alloc PoolAlloc
		alloc, err = h.allocateInPool(pool, reqLog, store, params)
		if err != nil {
			break
		}
		result = append(result, alloc)
	}
	if err != nil {
		for _, pool := range params.Pools {
			store.ReleaseReservationByID(pool, params.CniContainerid, params.CniIfname)
		}
		return nil, err
	}

	return result, nil
}

func (h *Handlers) allocateInPool(pool string, reqLog logr.Logger,
	store storePkg.Store, params *daemonv1.IPAMParameters) (PoolAlloc, error) {
	poolLog := reqLog.WithValues("pool", pool)

	poolCfg := h.poolManager.GetPoolByName(pool)
	if poolCfg == nil {
		return PoolAlloc{}, status.Errorf(codes.NotFound, "configuration for pool %s not found", pool)
	}
	rangeStart := net.ParseIP(poolCfg.StartIP)
	if rangeStart == nil {
		return PoolAlloc{}, poolCfgError(poolLog, pool, "invalid rangeStart")
	}
	rangeEnd := net.ParseIP(poolCfg.EndIP)
	if rangeEnd == nil {
		return PoolAlloc{}, poolCfgError(poolLog, pool, "invalid rangeEnd")
	}
	_, subnet, err := net.ParseCIDR(poolCfg.Subnet)
	if err != nil || subnet == nil || subnet.IP == nil || subnet.Mask == nil {
		return PoolAlloc{}, poolCfgError(poolLog, pool, "invalid subnet")
	}
	gateway := net.ParseIP(poolCfg.Gateway)
	if gateway == nil {
		return PoolAlloc{}, poolCfgError(poolLog, pool, "invalid gateway")
	}
	alloc := h.getAllocFunc(&allocator.RangeSet{allocator.Range{
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
		Subnet:     cniTypes.IPNet(*subnet),
		Gateway:    gateway,
	}}, pool, store)
	allocMeta := types.ReservationMetadata{
		CreateTime:         time.Now(),
		PoolConfigSnapshot: poolCfg.String(),
	}
	if params.Metadata != nil {
		allocMeta.PodUUID = params.Metadata.K8SPodUid
		allocMeta.PodName = params.Metadata.K8SPodName
		allocMeta.PodNamespace = params.Metadata.K8SPodNamespace
		allocMeta.DeviceID = params.Metadata.DeviceId
	}
	result, err := alloc.Allocate(params.CniContainerid, params.CniIfname, allocMeta)
	if err != nil {
		poolLog.Error(err, "failed to allocate IP address")
		if errors.Is(err, storePkg.ErrReservationAlreadyExist) {
			return PoolAlloc{}, status.Errorf(codes.AlreadyExists, "IP address in pool %s is already allocated", pool)
		}
		if errors.Is(err, allocator.ErrNoFreeAddresses) {
			return PoolAlloc{}, status.Errorf(codes.ResourceExhausted, "no free addresses in the pool %s", pool)
		}
		return PoolAlloc{}, status.Errorf(codes.Internal, "failed to allocate IP address in pool %s", pool)
	}
	poolLog.Info("IP address allocated", "allocation", result.String())

	return PoolAlloc{
		Pool:     pool,
		IPConfig: result,
	}, nil
}

func poolCfgError(reqLog logr.Logger, pool, reason string) error {
	reqLog.Error(nil, "invalid pool config", "pool", pool,
		"reason", reason)
	return status.Errorf(codes.Internal, "invalid config for pool %s", pool)
}
