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

package controllers

import (
	"context"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

// IPPoolReconciler reconciles IPPool objects
type IPPoolReconciler struct {
	PoolManager pool.Manager
	client.Client
	Scheme   *runtime.Scheme
	NodeName string
}

// Reconcile contains logic to sync IPPool objects
func (r *IPPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	ipPool := &ipamv1alpha1.IPPool{}
	poolKey := common.GetPoolKey(req.Name, common.PoolTypeIPPool)
	err := r.Client.Get(ctx, req.NamespacedName, ipPool)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			reqLog.Info("IPPool not found, removing from PoolManager")
			r.PoolManager.RemovePool(poolKey)
			return ctrl.Result{}, nil
		}
		reqLog.Error(err, "failed to get IPPool object from the cache")
		return ctrl.Result{}, err
	}
	reqLog.Info("Notification on IPPool", "name", ipPool.Name)
	found := false
	for _, alloc := range ipPool.Status.Allocations {
		if alloc.NodeName == r.NodeName {
			exclusions := make([]pool.ExclusionRange, 0, len(ipPool.Spec.Exclusions))
			for _, e := range ipPool.Spec.Exclusions {
				exclusions = append(exclusions, pool.ExclusionRange{StartIP: e.StartIP, EndIP: e.EndIP})
			}
			ipPool := &pool.Pool{
				Name:       ipPool.Name,
				Subnet:     ipPool.Spec.Subnet,
				Gateway:    ipPool.Spec.Gateway,
				StartIP:    alloc.StartIP,
				EndIP:      alloc.EndIP,
				Exclusions: exclusions,
			}
			r.PoolManager.UpdatePool(poolKey, ipPool)
			found = true
			break
		}
	}
	if !found {
		r.PoolManager.RemovePool(poolKey)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IPPool{}).
		Complete(r)
}
