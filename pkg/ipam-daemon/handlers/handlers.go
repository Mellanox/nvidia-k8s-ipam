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

	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	daemonv1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/daemon/v1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/allocator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-daemon/store"
	poolPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

type GetAllocatorFunc = func(s *allocator.RangeSet, poolName string, store storePkg.Store) allocator.IPAllocator

// New create and initialize new instance of grpc Handlers
func New(poolManager poolPkg.Manager, storeManager storePkg.Manager, getAllocFunc GetAllocatorFunc) *Handlers {
	return &Handlers{
		poolManager:  poolManager,
		storeManager: storeManager,
		getAllocFunc: getAllocFunc,
	}
}

// Handlers contains implementation of the GRPC endpoints handlers for ipam-daemon
type Handlers struct {
	poolManager  poolPkg.Manager
	storeManager storePkg.Manager
	getAllocFunc GetAllocatorFunc
	daemonv1.UnimplementedIPAMBackendServiceServer
}

func (h *Handlers) openStore(ctx context.Context) (storePkg.Store, error) {
	reqLog := klog.FromContext(ctx)
	store, err := h.storeManager.Open(ctx)
	if err != nil {
		reqLog.Error(err, "failed to open store")
		return nil, status.Errorf(codes.Internal, "failed to open store")
	}
	return store, nil
}

func (h *Handlers) closeStore(ctx context.Context, store storePkg.Store, err error) error {
	reqLog := klog.FromContext(ctx)
	if err == nil {
		if err := store.Commit(); err != nil {
			reqLog.Error(err, "failed to close store")
			return status.Errorf(codes.Internal, "failed to close store")
		}
		return nil
	}
	store.Cancel()
	return status.Errorf(codes.Internal, err.Error())
}

type paramsGetter interface {
	GetParameters() *daemonv1.IPAMParameters
}

func isRequiredError(field string) error {
	return status.Errorf(codes.InvalidArgument, "%s is required parameter", field)
}

func isInvalidError(field string) error {
	return status.Errorf(codes.InvalidArgument, "%s is invalid", field)
}

func validateReq(req paramsGetter) error {
	params := req.GetParameters()
	if params == nil {
		return isRequiredError("parameters")
	}
	if len(params.Pools) == 0 || len(params.Pools) > 2 {
		return isInvalidError("parameters.pools")
	}
	for i, p := range params.Pools {
		if p == "" {
			return isInvalidError(fmt.Sprintf("parameters.pools[%d]", i))
		}
	}
	if params.CniContainerid == "" {
		return isRequiredError("parameters.cni_containerid")
	}
	if params.CniIfname == "" {
		return isRequiredError("parameters.cni_ifname")
	}
	if params.Metadata == nil {
		return isRequiredError("parameters.metadata")
	}
	if params.Metadata.K8SPodName == "" {
		return isRequiredError("parameters.metadata.k8s_pod_name")
	}
	if params.Metadata.K8SPodNamespace == "" {
		return isRequiredError("parameters.metadata.k8s_pod_namespace")
	}
	return nil
}

func addFieldsToLogger(log logr.Logger, req paramsGetter) logr.Logger {
	params := req.GetParameters()
	if params == nil {
		return log
	}
	return log.WithValues("pools", params.Pools,
		"container_id", params.CniContainerid,
		"interface_name", params.CniIfname,
		"meta", params.Metadata.String(),
	)
}

func checkReqISCanceled(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return status.Error(codes.Canceled, "request has been canceled")
	default:
		return nil
	}
}
