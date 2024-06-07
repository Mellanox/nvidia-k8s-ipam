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
	"fmt"
	"net"

	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/allocator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	poolPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

type GetAllocatorFunc = func(s *allocator.RangeSet, exclusions *allocator.RangeSet,
	poolKey string, session storePkg.Session) allocator.IPAllocator

// New create and initialize new instance of grpc Handlers
func New(poolConfReader poolPkg.ConfigReader, store storePkg.Store, getAllocFunc GetAllocatorFunc) *Handlers {
	return &Handlers{
		poolConfReader: poolConfReader,
		store:          store,
		getAllocFunc:   getAllocFunc,
	}
}

// Handlers contains implementation of the GRPC endpoints handlers for ipam-daemon
type Handlers struct {
	poolConfReader poolPkg.ConfigReader
	store          storePkg.Store
	getAllocFunc   GetAllocatorFunc
	nodev1.UnsafeIPAMServiceServer
}

func (h *Handlers) openStore(ctx context.Context) (storePkg.Session, error) {
	reqLog := logr.FromContextOrDiscard(ctx)
	store, err := h.store.Open(ctx)
	if err != nil {
		reqLog.Error(err, "failed to open store")
		return nil, status.Errorf(codes.Internal, "failed to open store")
	}
	return store, nil
}

func (h *Handlers) closeSession(ctx context.Context, session storePkg.Session, err error) error {
	reqLog := logr.FromContextOrDiscard(ctx)
	if err == nil {
		if err := session.Commit(); err != nil {
			reqLog.Error(err, "failed to close session")
			return status.Errorf(codes.Internal, "failed to close session")
		}
		reqLog.Info("all changes are committed to the store")
		return nil
	}
	reqLog.Info("all store modifications are canceled")
	session.Cancel()
	return err
}

type paramsGetter interface {
	GetParameters() *nodev1.IPAMParameters
}

func fieldIsRequiredError(field string) error {
	return status.Errorf(codes.InvalidArgument, "%s is required field", field)
}

func fieldsIsInvalidError(field string) error {
	return status.Errorf(codes.InvalidArgument, "%s is invalid", field)
}

func validateReq(req paramsGetter) error {
	params := req.GetParameters()
	if params == nil {
		return fieldIsRequiredError("parameters")
	}
	if len(params.Pools) == 0 || len(params.Pools) > 2 {
		return fieldsIsInvalidError("parameters.pools")
	}
	for i, p := range params.Pools {
		if p == "" {
			return fieldsIsInvalidError(fmt.Sprintf("parameters.pools[%d]", i))
		}
	}
	if params.CniContainerid == "" {
		return fieldIsRequiredError("parameters.cni_containerid")
	}
	if params.CniIfname == "" {
		return fieldIsRequiredError("parameters.cni_ifname")
	}
	if params.Metadata == nil {
		return fieldIsRequiredError("parameters.metadata")
	}
	if params.Metadata.K8SPodName == "" {
		return fieldIsRequiredError("parameters.metadata.k8s_pod_name")
	}
	if params.Metadata.K8SPodNamespace == "" {
		return fieldIsRequiredError("parameters.metadata.k8s_pod_namespace")
	}
	for i, v := range params.RequestedIps {
		ipAddr := net.ParseIP(v)
		if ipAddr == nil {
			return fieldsIsInvalidError(fmt.Sprintf("parameters.requested_ips[%d]", i))
		}
	}
	return nil
}

func setDefaultsToParams(params *nodev1.IPAMParameters) *nodev1.IPAMParameters {
	if params.PoolType == nodev1.PoolType_POOL_TYPE_UNSPECIFIED {
		params.PoolType = nodev1.PoolType_POOL_TYPE_IPPOOL
	}
	return params
}

func addFieldsToLogger(log logr.Logger, req paramsGetter) logr.Logger {
	params := req.GetParameters()
	if params == nil {
		return log
	}
	return log.WithValues("pools", params.Pools,
		"pool_type", params.PoolType,
		"container_id", params.CniContainerid,
		"interface_name", params.CniIfname,
		"meta", params.Metadata.String(),
	)
}

func checkReqIsCanceled(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return status.Error(codes.Canceled, "request has been canceled")
	default:
		return nil
	}
}

// converts poolType from grpc request to string representations,
// default/fallback value is IPPool
func poolTypeAsString(poolType nodev1.PoolType) string {
	if poolType == nodev1.PoolType_POOL_TYPE_CIDRPOOL {
		return common.PoolTypeCIDRPool
	}
	return common.PoolTypeIPPool
}
