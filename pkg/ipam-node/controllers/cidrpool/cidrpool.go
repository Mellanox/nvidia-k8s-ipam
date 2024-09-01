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
	"net"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/common"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

// CIDRPoolReconciler reconciles CIDRPool objects
type CIDRPoolReconciler struct {
	PoolManager pool.Manager
	client.Client
	Scheme   *runtime.Scheme
	NodeName string
}

// Reconcile contains logic to sync CIDRPool objects
func (r *CIDRPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	cidrPool := &ipamv1alpha1.CIDRPool{}
	poolKey := common.GetPoolKey(req.Name, common.PoolTypeCIDRPool)
	err := r.Client.Get(ctx, req.NamespacedName, cidrPool)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			reqLog.Info("CIDRPool not found, removing from PoolManager")
			r.PoolManager.RemovePool(poolKey)
			return ctrl.Result{}, nil
		}
		reqLog.Error(err, "failed to get CIDRPool object from the cache")
		return ctrl.Result{}, err
	}
	reqLog.Info("Notification on CIDRPool", "name", cidrPool.Name)

	if errList := cidrPool.Validate(); len(errList) > 0 {
		reqLog.Info("CIDRPool has invalid config, ignore the pool", "name",
			cidrPool.Name, "reason", errList.ToAggregate().Error())
		r.PoolManager.RemovePool(poolKey)
		return ctrl.Result{}, nil
	}

	found := false
	for _, alloc := range cidrPool.Status.Allocations {
		if alloc.NodeName == r.NodeName {
			if errList := alloc.Validate(cidrPool); len(errList) > 0 {
				reqLog.Info("CIDRPool has invalid allocation for the node, ignore the pool",
					"name", cidrPool.Name, "reason", errList.ToAggregate().Error())
				r.PoolManager.RemovePool(poolKey)
				return ctrl.Result{}, nil
			}
			_, nodeSubnet, _ := net.ParseCIDR(alloc.Prefix)
			startIP := ip.NextIP(nodeSubnet.IP)
			if ip.IsPointToPointSubnet(nodeSubnet) {
				startIP = nodeSubnet.IP
			}
			endIP := ip.LastIP(nodeSubnet)
			routes := make([]pool.Route, 0, len(cidrPool.Spec.Routes))
			for _, r := range cidrPool.Spec.Routes {
				routes = append(routes, pool.Route{Dst: r.Dst})
			}
			pool := &pool.Pool{
				Name:       cidrPool.Name,
				Subnet:     alloc.Prefix,
				Gateway:    alloc.Gateway,
				StartIP:    startIP.String(),
				EndIP:      endIP.String(),
				Exclusions: buildExclusions(cidrPool.Spec.Exclusions, nodeSubnet, startIP, endIP),
				Routes:     routes,
			}
			pool.DefaultGateway = cidrPool.Spec.DefaultGateway
			reqLog.Info("CIDRPool config updated", "name", cidrPool.Name)
			r.PoolManager.UpdatePool(poolKey, pool)
			found = true
			break
		}
	}
	if !found {
		reqLog.Info("CIDRPool config removed", "name", cidrPool.Name, "reason", "allocation not found")
		r.PoolManager.RemovePool(poolKey)
	}
	return ctrl.Result{}, nil
}

func buildExclusions(ranges []ipamv1alpha1.ExcludeRange,
	nodeSubnet *net.IPNet, firstIP net.IP, lastIP net.IP) []pool.ExclusionRange {
	exclusions := make([]pool.ExclusionRange, 0, len(ranges))
	for _, r := range ranges {
		rangeStartIP := net.ParseIP(r.StartIP)
		rangeEndIP := net.ParseIP(r.EndIP)
		if rangeEndIP == nil || rangeStartIP == nil {
			continue
		}
		containsStart := nodeSubnet.Contains(rangeStartIP)
		containsEnd := nodeSubnet.Contains(rangeEndIP)
		if !containsStart && !containsEnd {
			// the range is not related to the node's subnet,
			continue
		}
		exlRange := pool.ExclusionRange{StartIP: r.StartIP, EndIP: r.EndIP}
		if !containsStart {
			exlRange.StartIP = firstIP.String()
		}
		if !containsEnd {
			exlRange.EndIP = lastIP.String()
		}
		exclusions = append(exclusions, exlRange)
	}
	return exclusions
}

// SetupWithManager sets up the controller with the Manager.
func (r *CIDRPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.CIDRPool{}).
		Complete(r)
}
