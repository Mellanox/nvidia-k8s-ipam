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
	"fmt"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

// NodeReconciler reconciles Node objects
type NodeReconciler struct {
	PoolsNamespace string
	NodeEventCh    chan event.GenericEvent
	MigrationCh    chan struct{}
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile contains logic to sync Node objects
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	select {
	case <-r.MigrationCh:
	case <-ctx.Done():
		return ctrl.Result{}, fmt.Errorf("canceled")
	}
	reqLog := log.FromContext(ctx)
	reqLog.Info("Notification on Node", "name", req.Name)
	node := &corev1.Node{}
	err := r.Client.Get(ctx, req.NamespacedName, node)
	if err != nil {
		if !apiErrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		reqLog.Info("node object removed")
	}
	// node updated, trigger sync for all pools
	poolList := &ipamv1alpha1.IPPoolList{}
	if err := r.Client.List(ctx, poolList, client.InNamespace(r.PoolsNamespace)); err != nil {
		return ctrl.Result{}, err
	}
	for _, p := range poolList.Items {
		r.NodeEventCh <- event.GenericEvent{
			Object: &ipamv1alpha1.IPPool{
				ObjectMeta: metav1.ObjectMeta{Namespace: r.PoolsNamespace, Name: p.Name},
			}}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
