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
			if ip.IsPointToPointSubnet(nodeSubnet) || ip.IsSingleIPSubnet(nodeSubnet) {
				startIP = nodeSubnet.IP
			}
			endIP := ip.LastIP(nodeSubnet)
			routes := make([]pool.Route, 0, len(cidrPool.Spec.Routes))
			for _, r := range cidrPool.Spec.Routes {
				routes = append(routes, pool.Route{Dst: r.Dst})
			}
			// Combine IP-based exclusions and index-based per-node exclusions
			exclusions := buildExclusions(cidrPool.Spec.Exclusions, nodeSubnet, startIP, endIP)
			// For perNodeExclusions, count indexes from subnet start (consistent with gatewayIndex)
			exclusions = append(exclusions, buildPerNodeExclusions(cidrPool.Spec.PerNodeExclusions, nodeSubnet.IP, endIP)...)
			p := &pool.Pool{
				Name:       cidrPool.Name,
				Subnet:     alloc.Prefix,
				Gateway:    alloc.Gateway,
				StartIP:    startIP.String(),
				EndIP:      endIP.String(),
				Exclusions: exclusions,
				Routes:     routes,
			}
			p.DefaultGateway = cidrPool.Spec.DefaultGateway
			reqLog.Info("CIDRPool config updated", "name", cidrPool.Name)
			r.PoolManager.UpdatePool(poolKey, p)
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

// buildPerNodeExclusions converts index-based exclusions to IP-based exclusions
// for the node's allocated prefix. Indexes are counted from the subnet start
// (network address), consistent with gatewayIndex behavior.
func buildPerNodeExclusions(
	ranges []ipamv1alpha1.ExcludeIndexRange, firstIP net.IP, lastIP net.IP) []pool.ExclusionRange {
	if len(ranges) == 0 {
		return nil
	}

	exclusions := make([]pool.ExclusionRange, 0, len(ranges))
	for _, r := range ranges {
		// Convert start index to IP
		excludeStartIP := ip.NextIPWithOffset(firstIP, int64(r.StartIndex))
		if excludeStartIP == nil {
			continue
		}

		// Convert end index to IP
		excludeEndIP := ip.NextIPWithOffset(firstIP, int64(r.EndIndex))
		if excludeEndIP == nil {
			continue
		}

		// Check if the exclusion range is within the node's allocation
		// Skip if both IPs are outside the range
		if ip.Cmp(excludeEndIP, firstIP) < 0 || ip.Cmp(excludeStartIP, lastIP) > 0 {
			continue
		}

		// Clamp to the node's range
		if ip.Cmp(excludeStartIP, firstIP) < 0 {
			excludeStartIP = firstIP
		}
		if ip.Cmp(excludeEndIP, lastIP) > 0 {
			excludeEndIP = lastIP
		}

		exclusions = append(exclusions, pool.ExclusionRange{
			StartIP: excludeStartIP.String(),
			EndIP:   excludeEndIP.String(),
		})
	}
	return exclusions
}

// SetupWithManager sets up the controller with the Manager.
func (r *CIDRPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.CIDRPool{}).
		Complete(r)
}
