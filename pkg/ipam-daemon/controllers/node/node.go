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

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

// NodeReconciler reconciles Node objects
type NodeReconciler struct {
	PoolManager pool.Manager
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile contains logic to sync Node objects
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	node := &corev1.Node{}
	err := r.Client.Get(ctx, req.NamespacedName, node)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if err := r.PoolManager.Update(node); err != nil {
		reqLog.Info("pool config from the node object is not updated, reset pool config",
			"reason", err.Error())
		r.PoolManager.Reset()
	} else {
		reqLog.Info("pools configuration updated", "data", r.PoolManager.GetPools())
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.ObjectOld == nil || e.ObjectNew == nil {
					return true
				}
				return e.ObjectOld.GetAnnotations()[pool.IPBlocksAnnotation] !=
					e.ObjectNew.GetAnnotations()[pool.IPBlocksAnnotation]
			},
		}).
		Complete(r)
}
