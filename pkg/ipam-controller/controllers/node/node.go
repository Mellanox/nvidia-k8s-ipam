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
	"errors"
	"reflect"
	"time"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/allocator"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/selector"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

// NodeReconciler reconciles Node objects
type NodeReconciler struct {
	Allocator *allocator.Allocator
	Selector  *selector.Selector

	ConfigEventCh chan event.GenericEvent
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile contains logic to sync Node objects
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	if !r.Allocator.IsConfigured() {
		reqLog.V(1).Info("allocator is not yet configured, requeue")
		return ctrl.Result{RequeueAfter: time.Second, Requeue: true}, nil
	}

	node := &corev1.Node{}
	err := r.Client.Get(ctx, req.NamespacedName, node)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			reqLog.Info("node object removed, deallocate ranges")
			r.Allocator.Deallocate(ctx, req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !r.Selector.Match(node) {
		reqLog.Info("node doesn't match selector, ensure range is not allocated")
		r.Allocator.Deallocate(ctx, node.Name)
		return r.cleanAnnotation(ctx, node)
	}

	var existingNodeAlloc map[string]*pool.IPPool
	nodeAllocManager, err := pool.NewManagerImpl(node)
	if err == nil {
		existingNodeAlloc = nodeAllocManager.GetPools()
	}

	expectedAlloc, err := r.Allocator.Allocate(ctx, node.Name)
	if err != nil {
		if errors.Is(allocator.ErrNoFreeRanges, err) {
			return r.cleanAnnotation(ctx, node)
		}
		return ctrl.Result{}, err
	}
	if reflect.DeepEqual(existingNodeAlloc, expectedAlloc) {
		reqLog.Info("node ranges are up-to-date")
		return ctrl.Result{}, nil
	}

	if err := pool.SetIPBlockAnnotation(node, expectedAlloc); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Client.Update(ctx, node); err != nil {
		reqLog.Info("failed to set annotation on the node object, deallocate ranges and retry",
			"reason", err.Error())
		r.Allocator.Deallocate(ctx, node.Name)
		return ctrl.Result{}, err
	}
	reqLog.Info("node object updated")

	return ctrl.Result{}, nil
}

// remove annotation from the node object in the API
func (r *NodeReconciler) cleanAnnotation(ctx context.Context, node *corev1.Node) (ctrl.Result, error) {
	if !pool.IPBlockAnnotationExists(node) {
		return ctrl.Result{}, nil
	}
	pool.RemoveIPBlockAnnotation(node)
	if err := r.Client.Update(ctx, node); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		// catch notifications received through chan from ConfigMap controller
		Watches(&source.Channel{Source: r.ConfigEventCh}, handler.Funcs{
			GenericFunc: func(e event.GenericEvent, q workqueue.RateLimitingInterface) {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: e.Object.GetNamespace(),
					Name:      e.Object.GetName(),
				}})
			}}).
		Complete(r)
}
