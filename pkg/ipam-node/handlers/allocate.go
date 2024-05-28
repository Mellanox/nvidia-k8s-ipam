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
	params := req.Parameters
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
			Pool: r.Pool,
			Ip:   r.Address.String(),
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
	for _, pool := range params.Pools {
		var alloc PoolAlloc
		alloc, err = h.allocateInPool(pool, reqLog, session, params)
		if err != nil {
			break
		}
		result = append(result, alloc)
	}
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (h *Handlers) allocateInPool(pool string, reqLog logr.Logger,
	session storePkg.Session, params *nodev1.IPAMParameters) (PoolAlloc, error) {
	poolLog := reqLog.WithValues("pool", pool)

	poolCfg := h.poolConfReader.GetPoolByName(pool)
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
	rangeSet := &allocator.RangeSet{allocator.Range{
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
		Subnet:     cniTypes.IPNet(*subnet),
		Gateway:    net.ParseIP(poolCfg.Gateway),
	}}
	if err := rangeSet.Canonicalize(); err != nil {
		return PoolAlloc{}, poolCfgError(poolLog, pool,
			fmt.Sprintf("invalid range config: %s", err.Error()))
	}
	alloc := h.getAllocFunc(rangeSet, &allocator.RangeSet{}, pool, session)
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
	result, err := alloc.Allocate(params.CniContainerid, params.CniIfname, allocMeta)
	if err != nil {
		poolLog.Error(err, "failed to allocate IP address")
		if errors.Is(err, storePkg.ErrReservationAlreadyExist) {
			return PoolAlloc{}, status.Errorf(codes.AlreadyExists,
				"allocation already exist in the pool %s", pool)
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
