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
	"fmt"
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	v1helper "k8s.io/component-helpers/scheduling/corev1"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/allocator"
)

// IPPoolReconciler reconciles Pool objects
type IPPoolReconciler struct {
	PoolsNamespace string
	NodeEventCh    chan event.GenericEvent
	MigrationCh    chan struct{}
	client.Client
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile contains logic to sync IPPool objects
func (r *IPPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	select {
	case <-r.MigrationCh:
	case <-ctx.Done():
		return ctrl.Result{}, fmt.Errorf("canceled")
	}
	reqLog := log.FromContext(ctx)
	if req.Namespace != r.PoolsNamespace {
		// this should never happen because of the watcher configuration of the manager from controller-runtime pkg
		reqLog.Info("Ignoring notification on IPPool from different namespace", "ns", req.Namespace)
		return ctrl.Result{}, nil
	}

	pool := &ipamv1alpha1.IPPool{}
	err := r.Client.Get(ctx, req.NamespacedName, pool)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			reqLog.Info("IPPool not found")
			return ctrl.Result{}, nil
		}
		reqLog.Error(err, "failed to get IPPool object from the cache")
		return ctrl.Result{}, err
	}
	reqLog.Info("Notification on IPPool", "name", pool.Name)

	errList := pool.Validate()
	if len(errList) != 0 {
		return r.handleInvalidSpec(ctx, errList.ToAggregate(), pool)
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		reqLog.Error(err, "failed to list Nodes")
		return ctrl.Result{}, err
	}
	nodeNames := make([]string, 0)
	for i := range nodeList.Items {
		node := nodeList.Items[i]
		if pool.Spec.NodeSelector != nil {
			match, err := v1helper.MatchNodeSelectorTerms(&node, pool.Spec.NodeSelector)
			if err != nil {
				reqLog.Error(err, "failed to match Node", "node", node.Name)
				continue
			}
			if match {
				nodeNames = append(nodeNames, node.Name)
			}
		} else {
			nodeNames = append(nodeNames, node.Name)
		}
	}

	sort.Slice(nodeNames, func(i, j int) bool {
		return nodeNames[i] < nodeNames[j]
	})

	pa := allocator.CreatePoolAllocatorFromIPPool(ctx, pool, sets.New[string]().Insert(nodeNames...))

	allocations := make([]ipamv1alpha1.Allocation, 0)
	for _, name := range nodeNames {
		// AllocateFromPool will return same allocation if it was already allocated
		a, err := pa.AllocateFromPool(ctx, name)
		if err != nil {
			if errors.Is(err, allocator.ErrNoFreeRanges) {
				msg := fmt.Sprintf("failed to allocate IPs on Node: %s", name)
				reqLog.Error(err, msg)
				r.recorder.Event(pool, "Warning", "NoFreeRanges", msg)
				continue
			}
			return ctrl.Result{}, err
		}
		alloc := ipamv1alpha1.Allocation{
			NodeName: name,
			StartIP:  a.StartIP.String(),
			EndIP:    a.EndIP.String(),
		}
		allocations = append(allocations, alloc)
	}

	if !reflect.DeepEqual(pool.Status.Allocations, allocations) {
		pool.Status.Allocations = allocations
		if err := r.Status().Update(ctx, pool); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *IPPoolReconciler) handleInvalidSpec(ctx context.Context,
	err error, pool *ipamv1alpha1.IPPool) (reconcile.Result, error) {
	reqLog := log.FromContext(ctx)
	reqLog.Error(err, "invalid IPPool Spec, clearing Status")
	pool.Status.Allocations = make([]ipamv1alpha1.Allocation, 0)
	if err2 := r.Status().Update(ctx, pool); err2 != nil {
		return ctrl.Result{}, err2
	}
	r.recorder.Event(pool, "Warning", "InvalidSpec", err.Error())
	return ctrl.Result{Requeue: false}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("IPPoolController")
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IPPool{}).
		// catch notifications received through chan from Node controller
		WatchesRawSource(source.Channel(r.NodeEventCh, handler.Funcs{
			GenericFunc: func(ctx context.Context, e event.TypedGenericEvent[client.Object],
				q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: e.Object.GetNamespace(),
					Name:      e.Object.GetName(),
				}})
			}})).
		Complete(r)
}
