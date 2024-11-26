/*
 Copyright 2024, NVIDIA CORPORATION & AFFILIATES
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
	"net"
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
)

// CIDRPoolReconciler reconciles CIDRPool objects
type CIDRPoolReconciler struct {
	PoolsNamespace string
	NodeEventCh    chan event.GenericEvent
	client.Client
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile contains logic to sync CIDRPool objects
func (r *CIDRPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	if req.Namespace != r.PoolsNamespace {
		// this should never happen because of the watcher configuration of the manager from controller-runtime pkg
		reqLog.Info("Ignoring notification on CIDRPool from different namespace", "ns", req.Namespace)
		return ctrl.Result{}, nil
	}

	pool := &ipamv1alpha1.CIDRPool{}
	err := r.Client.Get(ctx, req.NamespacedName, pool)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			reqLog.Info("CIDRPool not found")
			return ctrl.Result{}, nil
		}
		reqLog.Error(err, "failed to get CIDRPool object from the cache")
		return ctrl.Result{}, err
	}
	reqLog.Info("Notification on CIDRPool", "name", pool.Name)

	errList := pool.Validate()
	if len(errList) != 0 {
		return r.handleInvalidSpec(ctx, errList.ToAggregate(), pool)
	}

	_, cidrNetwork, _ := net.ParseCIDR(pool.Spec.CIDR)

	nodes, err := r.getMatchingNodes(ctx, pool.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, err
	}

	allocationsByNodeName, allocationsByPrefix := r.getValidExistingAllocationsMaps(ctx, pool, nodes)
	staticAllocationsByNodeName, staticAllocationsByPrefix := r.getStaticAllocationsMaps(pool)

	subnetGenFunc := ip.GetSubnetGen(cidrNetwork, pool.Spec.PerNodeNetworkPrefix)
	for _, node := range nodes {
		if _, allocationExist := allocationsByNodeName[node]; allocationExist {
			continue
		}
		nodeAlloc := r.getStaticAllocationForNodeIfExist(node, pool, staticAllocationsByNodeName)
		if nodeAlloc == nil {
			nodeAlloc = r.getNewAllocationForNode(subnetGenFunc, node, pool, allocationsByPrefix, staticAllocationsByPrefix)
		}
		if nodeAlloc == nil {
			msg := fmt.Sprintf("failed to allocated prefix for Node: %s, no free prefixes", node)
			reqLog.Error(err, msg)
			r.recorder.Event(pool, "Warning", "NoFreePrefixes", msg)
			continue
		}
		allocationsByNodeName[node] = nodeAlloc
		allocationsByPrefix[nodeAlloc.Prefix] = nodeAlloc
	}
	// prepare new status entry, keep the list sorted by the node name to avoid unnecessary updates caused by
	// different elements order
	allocations := make([]ipamv1alpha1.CIDRPoolAllocation, 0, len(allocationsByNodeName))
	for _, a := range allocationsByNodeName {
		reqLog.Info("prefix allocated", "node", a.NodeName, "prefix", a.Prefix, "gateway", a.Gateway)
		allocations = append(allocations, *a)
	}
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].NodeName < allocations[j].NodeName
	})

	if !reflect.DeepEqual(pool.Status.Allocations, allocations) {
		pool.Status.Allocations = allocations
		if err := r.Status().Update(ctx, pool); err != nil {
			return ctrl.Result{}, err
		}
		reqLog.Info("pool status updated")
	}
	return ctrl.Result{}, nil
}

// create new allocation for the node, returns nil if can't generate a subnet because lack of free prefixes
func (r *CIDRPoolReconciler) getNewAllocationForNode(subnetGen func() *net.IPNet, node string,
	pool *ipamv1alpha1.CIDRPool,
	allocByPrefix map[string]*ipamv1alpha1.CIDRPoolAllocation,
	staticAllocByPrefix map[string]*ipamv1alpha1.CIDRPoolStaticAllocation) *ipamv1alpha1.CIDRPoolAllocation {
	var nodePrefix *net.IPNet
	for {
		nodePrefix = subnetGen()
		if nodePrefix == nil {
			break
		}
		if _, alreadyAllocated := allocByPrefix[nodePrefix.String()]; alreadyAllocated {
			// subnet is already allocated, try next subnet
			continue
		}
		if _, alreadyAllocated := staticAllocByPrefix[nodePrefix.String()]; alreadyAllocated {
			// static allocation for the subnet exist (not for the current node), try next subnet
			continue
		}
		break
	}
	if nodePrefix == nil {
		return nil
	}
	gateway := ""
	if pool.Spec.GatewayIndex != nil {
		gateway = ipamv1alpha1.GetGatewayForSubnet(nodePrefix, *pool.Spec.GatewayIndex)
	}
	return &ipamv1alpha1.CIDRPoolAllocation{
		NodeName: node,
		Prefix:   nodePrefix.String(),
		Gateway:  gateway,
	}
}

// builds CIDRPoolAllocation from the CIDRPoolStaticAllocation if it exist for the node
func (r *CIDRPoolReconciler) getStaticAllocationForNodeIfExist(node string, pool *ipamv1alpha1.CIDRPool,
	staticAllocationsByNodeName map[string]*ipamv1alpha1.CIDRPoolStaticAllocation) *ipamv1alpha1.CIDRPoolAllocation {
	staticAlloc, staticAllocationFound := staticAllocationsByNodeName[node]
	if !staticAllocationFound {
		return nil
	}
	gateway := staticAlloc.Gateway
	if gateway == "" && pool.Spec.GatewayIndex != nil {
		// prefix is already validated
		_, nodeSubnet, _ := net.ParseCIDR(staticAlloc.Prefix)
		gateway = ipamv1alpha1.GetGatewayForSubnet(nodeSubnet, *pool.Spec.GatewayIndex)
	}
	return &ipamv1alpha1.CIDRPoolAllocation{
		NodeName: staticAlloc.NodeName,
		Prefix:   staticAlloc.Prefix,
		Gateway:  gateway,
	}
}

// returns two maps with valid existing allocations,
// first map contains nodeName to allocations mapping,
// second map contains prefix to allocation mapping
func (r *CIDRPoolReconciler) getValidExistingAllocationsMaps(ctx context.Context,
	pool *ipamv1alpha1.CIDRPool, nodes []string) (
	map[string]*ipamv1alpha1.CIDRPoolAllocation, map[string]*ipamv1alpha1.CIDRPoolAllocation) {
	reqLog := log.FromContext(ctx)
	nodeMap := make(map[string]*ipamv1alpha1.CIDRPoolAllocation, len(pool.Status.Allocations))
	prefixMap := make(map[string]*ipamv1alpha1.CIDRPoolAllocation, len(pool.Status.Allocations))

	matchingNodes := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		matchingNodes[n] = struct{}{}
	}

	for i := range pool.Status.Allocations {
		alloc := &pool.Status.Allocations[i]
		if _, nodeExist := matchingNodes[alloc.NodeName]; !nodeExist {
			reqLog.Info("node doesn't match the pool, discard the allocation", "node",
				alloc.NodeName, "prefix", alloc.Prefix, "gateway", alloc.Gateway)
			continue
		}
		errList := alloc.Validate(pool)
		if len(errList) > 0 {
			reqLog.Info("discard invalid allocation for the node", "node",
				alloc.NodeName, "prefix", alloc.Prefix, "gateway", alloc.Gateway, "reason", errList.ToAggregate().Error())
			continue
		}
		nodeMap[alloc.NodeName] = alloc
		prefixMap[alloc.Prefix] = alloc

		reqLog.Info("keep allocation for node", "node", alloc.NodeName, "prefix", alloc.Prefix, "gateway", alloc.Gateway)
	}
	return nodeMap, prefixMap
}

// returns two maps with static allocations,
// firs map contains nodeName to static allocations mapping
// second map contains prefix to static allocations mapping
func (r *CIDRPoolReconciler) getStaticAllocationsMaps(pool *ipamv1alpha1.CIDRPool) (
	map[string]*ipamv1alpha1.CIDRPoolStaticAllocation, map[string]*ipamv1alpha1.CIDRPoolStaticAllocation) {
	nodeMap := make(map[string]*ipamv1alpha1.CIDRPoolStaticAllocation, len(pool.Status.Allocations))
	prefixMap := make(map[string]*ipamv1alpha1.CIDRPoolStaticAllocation, len(pool.Status.Allocations))
	// static allocations are already validated by pool.Validate() call
	for i := range pool.Spec.StaticAllocations {
		alloc := &pool.Spec.StaticAllocations[i]
		nodeMap[alloc.NodeName] = alloc
		prefixMap[alloc.Prefix] = alloc
	}
	return nodeMap, prefixMap
}

// returns list with name of the nodes that match the pool selector
func (r *CIDRPoolReconciler) getMatchingNodes(
	ctx context.Context, nodeSelector *corev1.NodeSelector) ([]string, error) {
	reqLog := log.FromContext(ctx)
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		reqLog.Error(err, "failed to list Nodes")
		return nil, err
	}
	nodeNames := make([]string, 0)
	for i := range nodeList.Items {
		node := nodeList.Items[i]
		if nodeSelector == nil {
			nodeNames = append(nodeNames, node.Name)
			continue
		}
		match, err := v1helper.MatchNodeSelectorTerms(&node, nodeSelector)
		if err != nil {
			reqLog.Error(err, "failed to match Node", "node", node.Name)
			continue
		}
		if match {
			nodeNames = append(nodeNames, node.Name)
		}
	}

	sort.Slice(nodeNames, func(i, j int) bool {
		return nodeNames[i] < nodeNames[j]
	})
	return nodeNames, nil
}

// reset all allocations if the pool has invalid configuration
func (r *CIDRPoolReconciler) handleInvalidSpec(ctx context.Context,
	err error, pool *ipamv1alpha1.CIDRPool) (reconcile.Result, error) {
	reqLog := log.FromContext(ctx)
	reqLog.Error(err, "invalid CIDRPool Spec, clearing Status")
	pool.Status.Allocations = make([]ipamv1alpha1.CIDRPoolAllocation, 0)
	if err2 := r.Status().Update(ctx, pool); err2 != nil {
		return ctrl.Result{}, err2
	}
	r.recorder.Event(pool, "Warning", "InvalidSpec", err.Error())
	return ctrl.Result{Requeue: false}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CIDRPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recorder = mgr.GetEventRecorderFor("CIDRPoolController")
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.CIDRPool{}).
		// catch notifications received through chan from Node controller
		WatchesRawSource(source.Channel(r.NodeEventCh, handler.Funcs{
			GenericFunc: func(ctx context.Context, e event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: e.Object.GetNamespace(),
					Name:      e.Object.GetName(),
				}})
			}})).
		Complete(r)
}
