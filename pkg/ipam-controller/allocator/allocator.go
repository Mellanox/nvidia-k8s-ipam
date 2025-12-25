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

package allocator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"

	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
)

var ErrNoFreeRanges = errors.New("no free IP ranges available")

// AllocatedRange contains range of IPs allocated for the node
type AllocatedRange struct {
	StartIP net.IP
	EndIP   net.IP
}

func newPoolAllocator(cfg AllocationConfig) *PoolAllocator {
	return &PoolAllocator{
		cfg:         cfg,
		allocations: map[string]AllocatedRange{},
		startIps:    sets.New[string]()}
}

// PoolAllocator contains pool settings and related allocations
type PoolAllocator struct {
	cfg AllocationConfig
	// allocations for nodes, key is the node name, value is allocated range
	allocations map[string]AllocatedRange
	// startIps map, key is StartIp as string. It is used to find overlaps
	startIps sets.Set[string]
}

func (pa *PoolAllocator) getLog(ctx context.Context, cfg AllocationConfig) logr.Logger {
	return logr.FromContextOrDiscard(ctx).WithName(fmt.Sprintf("allocator/pool=%s", cfg.PoolName))
}

// AllocateFromPool allocates a new range in the poolAllocator or
// return existing one,
// returns ErrNoFreeRanges if no free ranges available
func (pa *PoolAllocator) AllocateFromPool(ctx context.Context, node string) (*AllocatedRange, error) {
	log := pa.getLog(ctx, pa.cfg).WithValues("node", node)
	existingAlloc, exist := pa.allocations[node]
	if exist {
		log.V(1).Info("allocation for the node already exist",
			"start", existingAlloc.StartIP, "end", existingAlloc.EndIP)
		return &existingAlloc, nil
	}

	// determine the first possible range for the subnet
	var startIP net.IP
	var endIP net.IP
	if pa.canUseNetworkAddress() {
		startIP = pa.cfg.Subnet.IP
	} else {
		startIP = ip.NextIP(pa.cfg.Subnet.IP)
	}
	endIP = ip.NextIPWithOffset(startIP, int64(pa.cfg.PerNodeBlockSize)-1)

	for pa.allocationValid(startIP, endIP) {
		// check if the entire range is excluded
		if pa.isEntireRangeExcluded(AllocatedRange{StartIP: startIP, EndIP: endIP}) {
			startIP, endIP = pa.getNextNonExcludedCandidates(startIP, endIP)
			continue
		}

		// entire range is not excluded, check if the range is allocated i.e its present in startIPs
		if pa.startIps.Has(startIP.String()) {
			startIP = ip.NextIP(endIP)
			endIP = ip.NextIPWithOffset(startIP, int64(pa.cfg.PerNodeBlockSize)-1)
			continue
		}

		// if perNodeBlockSize is 1, we should not allocate the gateway
		if pa.cfg.PerNodeBlockSize == 1 && pa.cfg.Gateway != nil && startIP.Equal(pa.cfg.Gateway) {
			startIP = ip.NextIP(endIP)
			endIP = ip.NextIPWithOffset(startIP, int64(pa.cfg.PerNodeBlockSize)-1)
			continue
		}

		// found free range or allocation is invalid break from the loop
		break
	}

	if !pa.allocationValid(startIP, endIP) {
		// out of range
		log.Info("can't allocate: pool has no free ranges")
		return &AllocatedRange{}, ErrNoFreeRanges
	}

	log.Info("range allocated",
		"start", startIP, "end", endIP)
	a := AllocatedRange{
		StartIP: startIP,
		EndIP:   endIP,
	}
	pa.allocations[node] = a
	pa.startIps.Insert(startIP.String())
	return &a, nil
}

// getNextNonExcludedCandidates returns the next non excluded start and end IP candidates
func (pa *PoolAllocator) getNextNonExcludedCandidates(currentStartIP net.IP, currentEndIP net.IP) (net.IP, net.IP) {
	if currentStartIP == nil || currentEndIP == nil {
		return nil, nil
	}

	// the next start and end IP candidates start from currentEndIP +1
	nextStartIP := ip.NextIP(currentEndIP)
	nextEndIP := ip.NextIPWithOffset(nextStartIP, int64(pa.cfg.PerNodeBlockSize)-1)
	if nextStartIP == nil || nextEndIP == nil {
		return nil, nil
	}

	// merge exclude ranges
	mergedRanges := ipamv1alpha1.MergeExcludeRanges(pa.cfg.ExcludeRanges)

	for _, excludeRange := range mergedRanges {
		// find the range that contains startIP, endIP
		if !excludeRange.Contains(nextStartIP) || !excludeRange.Contains(nextEndIP) {
			continue
		}

		// exclude range contains the start and end IP, the next free IP is the next IP after the end of the exclude range
		// that IP is for sure not excluded because if it was, it would have been included in the merged exclude ranges.
		nextFreeIP := ip.NextIP(net.ParseIP(excludeRange.EndIP))
		if nextFreeIP == nil {
			return nil, nil
		}

		// align it to the nearest block size boundary and return the next candidate start and end IPs
		distance := ip.Distance(nextStartIP, nextFreeIP)
		ipsToSkip := (distance / int64(pa.cfg.PerNodeBlockSize)) * int64(pa.cfg.PerNodeBlockSize)

		nextStartIP = ip.NextIPWithOffset(nextStartIP, ipsToSkip)
		nextEndIP = ip.NextIPWithOffset(nextStartIP, int64(pa.cfg.PerNodeBlockSize)-1)
		return nextStartIP, nextEndIP
	}

	// if we reach this point then startIP and endIP are not fully contained in any of the exclude ranges
	// so they can be used as candidates for allocation
	return nextStartIP, nextEndIP
}

// allocationValid checks if the allocation is valid
// 1. startIP and endIP are valid IPs
// 2. endIP is within the subnet (which means startIP is also within the subnet)
// 3. endIP is not a broadcast IP
func (pa *PoolAllocator) allocationValid(startIP net.IP, endIP net.IP) bool {
	return (startIP != nil &&
		endIP != nil &&
		pa.cfg.Subnet.Contains(endIP) &&
		!ip.IsBroadcast(endIP, pa.cfg.Subnet))
}

// Deallocate remove info about allocation for the node from the poolAllocator
func (pa *PoolAllocator) Deallocate(ctx context.Context, node string) {
	log := pa.getLog(ctx, pa.cfg)
	log.Info("deallocate range for node", "node", node)
	a, ok := pa.allocations[node]
	if ok {
		pa.startIps.Delete(a.StartIP.String())
		delete(pa.allocations, node)
	}
}

// canUseNetworkAddress returns true if it is allowed to use network address in the node range
// it is allowed to use network address if the subnet is point to point of a single IP subnet
func (pa *PoolAllocator) canUseNetworkAddress() bool {
	return ip.IsPointToPointSubnet(pa.cfg.Subnet) || ip.IsSingleIPSubnet(pa.cfg.Subnet)
}

// load loads range to the pool allocator with validation for conflicts
func (pa *PoolAllocator) load(ctx context.Context, nodeName string, allocRange AllocatedRange) error {
	log := pa.getLog(ctx, pa.cfg).WithValues("node", nodeName)
	if err := pa.checkAllocation(allocRange); err != nil {
		log.Info("range check failed", "reason", err.Error())
		return err
	}
	// range size is always the same, then an overlap means the blocks are necessarily equal.
	// it's enough to just check StartIP which can technically act as an absolute "block index" in the subnet
	if pa.startIps.Has(allocRange.StartIP.String()) {
		err := fmt.Errorf("range overlaps")
		log.Info("skip loading range", "reason", err.Error())
		return err
	}

	log.Info("data loaded", "startIP", allocRange.StartIP, "endIP", allocRange.EndIP)
	pa.allocations[nodeName] = allocRange
	pa.startIps.Insert(allocRange.StartIP.String())
	return nil
}

func (pa *PoolAllocator) checkAllocation(allocRange AllocatedRange) error {
	if !pa.cfg.Subnet.Contains(allocRange.StartIP) || !pa.cfg.Subnet.Contains(allocRange.EndIP) {
		return fmt.Errorf("invalid allocation allocators: start or end IP is out of the subnet")
	}
	if ip.Cmp(allocRange.EndIP, allocRange.StartIP) < 0 {
		return fmt.Errorf("invalid allocation allocators: start IP must be less or equal to end IP")
	}
	distanceFromNetworkStart := ip.Distance(pa.cfg.Subnet.IP, allocRange.StartIP)
	// check that StartIP of the range has valid offset.
	// all ranges have same size, so we can simply check that (StartIP offset) % pa.cfg.PerNodeBlockSize == 0
	if pa.canUseNetworkAddress() {
		if math.Mod(float64(distanceFromNetworkStart), float64(pa.cfg.PerNodeBlockSize)) != 0 {
			return fmt.Errorf("invalid start IP offset")
		}
	} else {
		if distanceFromNetworkStart < 1 ||
			// -1 required because we skip network address (e.g. in 192.168.0.0/24, first allocation will be 192.168.0.1)
			math.Mod(float64(distanceFromNetworkStart)-1, float64(pa.cfg.PerNodeBlockSize)) != 0 {
			return fmt.Errorf("invalid start IP offset")
		}
	}
	if ip.Distance(allocRange.StartIP, allocRange.EndIP) != int64(pa.cfg.PerNodeBlockSize)-1 {
		return fmt.Errorf("ip count mismatch")
	}
	// for single IP ranges we need to discard allocation if it matches the gateway
	if pa.cfg.PerNodeBlockSize == 1 && pa.cfg.Gateway != nil && allocRange.StartIP.Equal(pa.cfg.Gateway) {
		return fmt.Errorf("gw can't be allocated when perNodeBlockSize is 1")
	}

	// check if the entire range is excluded
	if pa.isEntireRangeExcluded(allocRange) {
		return fmt.Errorf("entire range is excluded")
	}

	return nil
}

// isEntireRangeExcluded checks if the entire allocated range is excluded
func (pa *PoolAllocator) isEntireRangeExcluded(allocRange AllocatedRange) bool {
	// create a merged list of exclude ranges clamped to the allocated range start and end IP
	excludeRangesFromPerNodeExclusions := ipamv1alpha1.ExcludeIndexRangeToExcludeRange(
		pa.cfg.PerNodeExclusions, allocRange.StartIP.String())
	allExcludeRanges := append(pa.cfg.ExcludeRanges, excludeRangesFromPerNodeExclusions...)
	// Clamp the ranges to the allocated range start and end IP
	allExcludeRanges = ipamv1alpha1.ClampExcludeRanges(
		allExcludeRanges, allocRange.StartIP.String(), allocRange.EndIP.String())
	// Merge the excluded ranges
	allExcludeRanges = ipamv1alpha1.MergeExcludeRanges(allExcludeRanges)

	// if the merged list is empty, then there are no excluded ranges.
	if len(allExcludeRanges) == 0 {
		return false
	}

	// if the merged list contains more than one element, then there are non excluded "holes" in the range. which means
	// some IPs are potentially still available for allocation.
	if len(allExcludeRanges) > 1 {
		return false
	}

	// merged list contains exactly one element.
	// check if the element covers the entire allocated range.
	excludeStart := net.ParseIP(allExcludeRanges[0].StartIP)
	excludeEnd := net.ParseIP(allExcludeRanges[0].EndIP)
	if excludeStart == nil || excludeEnd == nil {
		return false
	}

	if ip.Cmp(excludeStart, allocRange.StartIP) <= 0 && ip.Cmp(excludeEnd, allocRange.EndIP) >= 0 {
		return true
	}

	return false
}

// AllocationConfig contains configuration of the IP pool
type AllocationConfig struct {
	PoolName         string
	Subnet           *net.IPNet
	Gateway          net.IP
	PerNodeBlockSize int
	NodeSelector     *corev1.NodeSelector
	// list of ranges to exclude from allocation
	ExcludeRanges []ipamv1alpha1.ExcludeRange
	// list of index ranges to exclude from allocation
	PerNodeExclusions []ipamv1alpha1.ExcludeIndexRange
}

func (pc *AllocationConfig) Equal(other *AllocationConfig) bool {
	return reflect.DeepEqual(pc, other)
}

// CreatePoolAllocatorFromIPPool creates a PoolAllocator and load data from the IPPool CR
// the nodes Set contains the nodes that match the current NodeSelector of the IPPool
// it is used to filter out Allocations that are not relevant anymore
func CreatePoolAllocatorFromIPPool(ctx context.Context,
	p *ipamv1alpha1.IPPool, nodes sets.Set[string]) *PoolAllocator {
	log := logr.FromContextOrDiscard(ctx)
	_, subnet, _ := net.ParseCIDR(p.Spec.Subnet)
	gateway := net.ParseIP(p.Spec.Gateway)

	allocatorConfig := AllocationConfig{
		PoolName:          p.Name,
		Subnet:            subnet,
		Gateway:           gateway,
		PerNodeBlockSize:  p.Spec.PerNodeBlockSize,
		NodeSelector:      p.Spec.NodeSelector,
		ExcludeRanges:     p.Spec.Exclusions,
		PerNodeExclusions: p.Spec.PerNodeExclusions,
	}
	pa := newPoolAllocator(allocatorConfig)
	for i := range p.Status.Allocations {
		alloc := p.Status.Allocations[i]
		if !nodes.Has(alloc.NodeName) {
			continue
		}
		var err error
		nodeAllocStart := net.ParseIP(alloc.StartIP)
		if nodeAllocStart == nil {
			err = fmt.Errorf("startIP is incorrect ip")
		}
		nodeAllocEnd := net.ParseIP(alloc.EndIP)
		if nodeAllocEnd == nil {
			err = fmt.Errorf("endIP is incorrect ip")
		}
		logInfo := func(err error) {
			log.Info("ignore allocation info from node", "node", alloc.NodeName,
				"pool", p.Name, "reason", err.Error())
		}
		if err != nil {
			logInfo(err)
			continue
		}
		allocRange := AllocatedRange{StartIP: nodeAllocStart, EndIP: nodeAllocEnd}
		if err := pa.load(ctx, alloc.NodeName, allocRange); err != nil {
			logInfo(err)
			continue
		}
	}
	return pa
}
