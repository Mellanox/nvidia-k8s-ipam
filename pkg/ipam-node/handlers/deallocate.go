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

	"github.com/go-logr/logr"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
)

// Deallocate is the handler for Deallocate GRPC endpoint
func (h *Handlers) Deallocate(
	ctx context.Context, req *nodev1.DeallocateRequest) (*nodev1.DeallocateResponse, error) {
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
	for _, p := range params.Pools {
		store.ReleaseReservationByID(p, params.CniContainerid, params.CniIfname)
	}
	if err := h.closeSession(ctx, store, nil); err != nil {
		return nil, err
	}
	reqLog.Info("reservation released")
	return &nodev1.DeallocateResponse{}, nil
}
