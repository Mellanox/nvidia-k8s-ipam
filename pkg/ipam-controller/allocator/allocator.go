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
	"sort"

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
	allocations := pa.getAllocationsAsSlice()
	var startIP net.IP
	if len(allocations) == 0 || ip.Distance(pa.cfg.Subnet.IP, allocations[0].StartIP) > 2 {
		// start allocations from the network address if there are no allocations or if the "hole" exist before
		// the firs allocation
		startIP = ip.NextIP(pa.cfg.Subnet.IP)
	} else {
		for i := 0; i < len(allocations); i++ {
			nextI := i + 1
			// if last allocation in the list
			if nextI > len(allocations)-1 ||
				// or we found a "hole" in allocations. the "hole" can't be less than required for
				// the allocation by design. because we reset all allocations when PerNodeBlockSize size changes
				ip.Distance(allocations[i].EndIP, allocations[nextI].StartIP) > 1 {
				startIP = ip.NextIP(allocations[i].EndIP)
				break
			}
		}
	}
	endIP := ip.NextIPWithOffset(startIP, int64(pa.cfg.PerNodeBlockSize)-1)

	if startIP == nil ||
		endIP == nil ||
		!pa.cfg.Subnet.Contains(endIP) ||
		ip.IsBroadcast(endIP, pa.cfg.Subnet) {
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

	if ip.Cmp(allocRange.EndIP, allocRange.StartIP) <= 0 {
		return fmt.Errorf("invalid allocation allocators: start IP must be less then end IP")
	}

	// check that StartIP of the range has valid offset.
	// all ranges have same size, so we can simply check that (StartIP offset - 1) % pa.cfg.PerNodeBlockSize == 0
	// -1 required because we skip network addressee (e.g. in 192.168.0.0/24, first allocation will be 192.168.0.1)
	distanceFromNetworkStart := ip.Distance(pa.cfg.Subnet.IP, allocRange.StartIP)
	if distanceFromNetworkStart < 1 ||
		math.Mod(float64(distanceFromNetworkStart)-1, float64(pa.cfg.PerNodeBlockSize)) != 0 {
		return fmt.Errorf("invalid start IP offset")
	}
	if ip.Distance(allocRange.StartIP, allocRange.EndIP) != int64(pa.cfg.PerNodeBlockSize)-1 {
		return fmt.Errorf("ip count mismatch")
	}
	return nil
}

// return slice with allocated ranges.
// ranges are not overlap and are sorted, but there can be "holes" between ranges
func (pa *PoolAllocator) getAllocationsAsSlice() []AllocatedRange {
	allocatedRanges := make([]AllocatedRange, 0, len(pa.allocations))
	for _, a := range pa.allocations {
		allocatedRanges = append(allocatedRanges, a)
	}
	sort.Slice(allocatedRanges, func(i, j int) bool {
		return ip.Cmp(allocatedRanges[i].StartIP, allocatedRanges[j].StartIP) < 0
	})
	return allocatedRanges
}

// AllocationConfig contains configuration of the IP pool
type AllocationConfig struct {
	PoolName         string
	Subnet           *net.IPNet
	Gateway          net.IP
	PerNodeBlockSize int
	NodeSelector     *corev1.NodeSelector
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
		PoolName:         p.Name,
		Subnet:           subnet,
		Gateway:          gateway,
		PerNodeBlockSize: p.Spec.PerNodeBlockSize,
		NodeSelector:     p.Spec.NodeSelector,
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
