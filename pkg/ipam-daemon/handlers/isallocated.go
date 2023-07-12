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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	daemonv1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/daemon/v1"
)

// IsAllocated is the handler for IsAllocated GRPC endpoint
func (h *Handlers) IsAllocated(
	ctx context.Context, req *daemonv1.IsAllocatedRequest) (*daemonv1.IsAllocatedResponse, error) {
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

	for _, p := range params.Pools {
		poolLog := reqLog.WithValues("pool", p)
		res := store.GetReservationByID(p, params.CniContainerid, params.CniIfname)
		if res == nil {
			poolLog.Info("reservation not found")
			err = status.Errorf(codes.NotFound, "reservation for pool %s not found", p)
			break
		}
		reqLog.Info("reservation exist")
		reqLog.V(1).Info("reservation data", "data", res.String())
	}
	if err := h.closeStore(ctx, store, err); err != nil {
		return nil, err
	}

	return &daemonv1.IsAllocatedResponse{}, nil
}
