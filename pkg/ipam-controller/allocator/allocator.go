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
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"reflect"
	"sort"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

var ErrNoFreeRanges = errors.New("no free IP ranges available")

// contains allocation information for the node
type nodeAllocationInfo struct {
	Node    string
	Subnet  *net.IPNet
	Gateway net.IP
	allocatedRange
}

// allocatedRange contains range of IPs allocated for the node
type allocatedRange struct {
	StartIP net.IP
	EndIP   net.IP
}

func newPoolAllocator(cfg AllocationConfig) *poolAllocator {
	return &poolAllocator{
		cfg:         cfg,
		allocations: map[string]allocatedRange{}}
}

// poolAllocator contains pool settings and related allocations
type poolAllocator struct {
	cfg AllocationConfig
	// allocations for nodes, key is the node name, value is allocated range
	allocations map[string]allocatedRange
}

func (pa *poolAllocator) getLog(ctx context.Context, cfg AllocationConfig) logr.Logger {
	return logr.FromContextOrDiscard(ctx).WithName(fmt.Sprintf("allocator/pool=%s", cfg.PoolName))
}

// Configure update configuration for pool allocator, resets allocations if required
func (pa *poolAllocator) Configure(ctx context.Context, cfg AllocationConfig) {
	log := pa.getLog(ctx, cfg)
	log.V(1).Info("pool configuration update")
	if pa.cfg.Equal(&cfg) {
		log.V(1).Info("pool configuration is the same, keep allocations")
		return
	}
	pa.cfg = cfg
	pa.allocations = map[string]allocatedRange{}
	log.Info("pool configuration updated, reset allocations")
}

// Allocate allocates a new range in the poolAllocator or
// return existing one,
// returns ErrNoFreeRanges if no free ranges available
func (pa *poolAllocator) Allocate(ctx context.Context, node string) (nodeAllocationInfo, error) {
	log := pa.getLog(ctx, pa.cfg).WithValues("node", node)
	existingAlloc, exist := pa.allocations[node]
	if exist {
		log.V(1).Info("allocation for the node already exist",
			"start", existingAlloc.StartIP, "end", existingAlloc.EndIP)
		return pa.getNodeAllocationInfo(node, existingAlloc), nil
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
		return nodeAllocationInfo{}, ErrNoFreeRanges
	}

	log.Info("range allocated",
		"start", startIP, "end", endIP)
	pa.allocations[node] = allocatedRange{
		StartIP: startIP,
		EndIP:   endIP,
	}
	return pa.getNodeAllocationInfo(node, pa.allocations[node]), nil
}

// Deallocate remove info about allocation for the node from the poolAllocator
func (pa *poolAllocator) Deallocate(ctx context.Context, node string) {
	log := pa.getLog(ctx, pa.cfg)
	log.Info("deallocate range for node", "node", node)
	delete(pa.allocations, node)
}

// Load loads range to the pool allocator with validation for conflicts
func (pa *poolAllocator) Load(ctx context.Context, allocData nodeAllocationInfo) error {
	log := pa.getLog(ctx, pa.cfg)
	if err := pa.checkAllocation(allocData); err != nil {
		log.Info("range check failed", "reason", err.Error())
		return err
	}
	allocations := pa.getAllocationsAsSlice()
	for _, a := range allocations {
		// range size is always the same, then an overlap means the blocks are necessarily equal.
		// it's enough to just compare StartIP which can technically act as an absolute "block index" in the subnet
		if allocData.allocatedRange.StartIP.Equal(a.StartIP) {
			err := fmt.Errorf("range overlaps with: %v", a)
			log.Info("skip loading range", "reason", err.Error())
			return err
		}
	}
	pa.allocations[allocData.Node] = allocData.allocatedRange
	return nil
}

func (pa *poolAllocator) checkAllocation(allocData nodeAllocationInfo) error {
	if allocData.Subnet.String() != pa.cfg.Subnet.String() {
		return fmt.Errorf("subnet mismatch")
	}
	if !allocData.Gateway.Equal(pa.cfg.Gateway) {
		return fmt.Errorf("gateway mismatch")
	}
	// check that StartIP of the range has valid offset.
	// all ranges have same size, so we can simply check that (StartIP offset - 1) % pa.cfg.PerNodeBlockSize == 0
	// -1 required because we skip network addressee (e.g. in 192.168.0.0/24, first allocation will be 192.168.0.1)
	distanceFromNetworkStart := ip.Distance(pa.cfg.Subnet.IP, allocData.StartIP)
	if distanceFromNetworkStart < 1 ||
		math.Mod(float64(distanceFromNetworkStart)-1, float64(pa.cfg.PerNodeBlockSize)) != 0 {
		return fmt.Errorf("invalid start IP offset")
	}
	if ip.Distance(allocData.StartIP, allocData.EndIP) != int64(pa.cfg.PerNodeBlockSize)-1 {
		return fmt.Errorf("ip count mismatch")
	}
	return nil
}

func (pa *poolAllocator) getNodeAllocationInfo(node string, allocRange allocatedRange) nodeAllocationInfo {
	return nodeAllocationInfo{allocatedRange: allocRange, Subnet: pa.cfg.Subnet, Gateway: pa.cfg.Gateway, Node: node}
}

// return slice with allocated ranges.
// ranges are not overlap and are sorted, but there can be "holes" between ranges
func (pa *poolAllocator) getAllocationsAsSlice() []allocatedRange {
	allocatedRanges := make([]allocatedRange, 0, len(pa.allocations))
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
}

func (pc *AllocationConfig) Equal(other *AllocationConfig) bool {
	return reflect.DeepEqual(pc, other)
}

// New create and initialize new allocator
func New() *Allocator {
	return &Allocator{allocators: map[string]*poolAllocator{}}
}

type Allocator struct {
	lock       sync.Mutex
	allocators map[string]*poolAllocator
	configured bool
}

// IsConfigured returns true if allocator is configured
func (a *Allocator) IsConfigured() bool {
	a.lock.Lock()
	defer a.lock.Unlock()
	return a.configured
}

// Configure update allocator configuration
func (a *Allocator) Configure(ctx context.Context, configs []AllocationConfig) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.configure(ctx, configs)
}

// ConfigureAndLoadAllocations configures allocator and load data from the node objects
func (a *Allocator) ConfigureAndLoadAllocations(ctx context.Context, configs []AllocationConfig, nodes []corev1.Node) {
	log := logr.FromContextOrDiscard(ctx)
	a.lock.Lock()
	defer a.lock.Unlock()
	a.configure(ctx, configs)
	for i := range nodes {
		node := nodes[i]
		nodePoolMgr, err := pool.NewManagerImpl(&node)
		if err != nil {
			log.Info("skip loading data from the node", "reason", err.Error())
			continue
		}
		// load allocators only for know pools (pools which are defined in the config)
		for poolName, poolData := range a.allocators {
			nodeIPPoolConfig := nodePoolMgr.GetPoolByName(poolName)
			allocInfo, err := ipPoolConfigToNodeAllocationInfo(node.Name, nodeIPPoolConfig)
			if err != nil {
				log.Info("ignore allocation info from node", "reason", err.Error())
				continue
			}

			if err := poolData.Load(ctx, allocInfo); err != nil {
				continue
			}
			a.allocators[poolName] = poolData
		}
	}
	a.configured = true
}

// Allocate allocates ranges for node from all pools
func (a *Allocator) Allocate(ctx context.Context, nodeName string) (map[string]*pool.IPPool, error) {
	log := logr.FromContextOrDiscard(ctx).WithValues("node", nodeName)
	a.lock.Lock()
	defer a.lock.Unlock()

	nodeAllocations := make(map[string]*pool.IPPool, len(a.allocators))
	for poolName, allocator := range a.allocators {
		allocation, err := allocator.Allocate(ctx, nodeName)
		if err != nil {
			a.deallocate(ctx, nodeName)
			return nil, err
		}
		nodeAllocations[poolName] = nodeAllocationInfoToIPPoolConfig(poolName, allocation)
	}

	if log.V(1).Enabled() {
		//nolint:errchkjson
		dump, _ := json.Marshal(nodeAllocations)
		log.V(1).Info("allocated ranges", "ranges", dump)
	}
	return nodeAllocations, nil
}

func (a *Allocator) deallocate(ctx context.Context, nodeName string) {
	for _, allocator := range a.allocators {
		allocator.Deallocate(ctx, nodeName)
	}
}

// Deallocate release all ranges allocated for node
func (a *Allocator) Deallocate(ctx context.Context, nodeName string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.deallocate(ctx, nodeName)
}

func (a *Allocator) configure(ctx context.Context, configs []AllocationConfig) {
	log := logr.FromContextOrDiscard(ctx)
	for _, cfg := range configs {
		poolLog := log.WithValues("pool", cfg.PoolName,
			"gateway", cfg.Gateway.String(), "subnet", cfg.Subnet.String(), "perNodeBlockSize", cfg.PerNodeBlockSize)
		pAlloc, exist := a.allocators[cfg.PoolName]
		if exist {
			poolLog.Info("update IP pool allocator config")
			pAlloc.Configure(ctx, cfg)
		} else {
			poolLog.Info("initialize IP pool allocator")
			a.allocators[cfg.PoolName] = newPoolAllocator(cfg)
		}
	}
	// remove outdated pools from controller state
	for poolName := range a.allocators {
		found := false
		for _, cfg := range configs {
			if poolName == cfg.PoolName {
				found = true
				break
			}
		}
		if !found {
			delete(a.allocators, poolName)
		}
	}
}

func nodeAllocationInfoToIPPoolConfig(poolName string, alloc nodeAllocationInfo) *pool.IPPool {
	return &pool.IPPool{
		Name:    poolName,
		Subnet:  alloc.Subnet.String(),
		StartIP: alloc.StartIP.String(),
		EndIP:   alloc.EndIP.String(),
		Gateway: alloc.Gateway.String(),
	}
}

func ipPoolConfigToNodeAllocationInfo(node string, alloc *pool.IPPool) (nodeAllocationInfo, error) {
	if alloc == nil {
		return nodeAllocationInfo{}, fmt.Errorf("node allocation is nil")
	}
	_, subnet, err := net.ParseCIDR(alloc.Subnet)
	if subnet == nil || err != nil {
		return nodeAllocationInfo{}, fmt.Errorf("subnet is incorrect network")
	}
	gateway := net.ParseIP(alloc.Gateway)
	if gateway == nil {
		return nodeAllocationInfo{}, fmt.Errorf("gateway is incorrect ip")
	}
	nodeAllocStart := net.ParseIP(alloc.StartIP)
	if nodeAllocStart == nil {
		return nodeAllocationInfo{}, fmt.Errorf("startIP is incorrect ip")
	}
	nodeAllocEnd := net.ParseIP(alloc.EndIP)
	if nodeAllocEnd == nil {
		return nodeAllocationInfo{}, fmt.Errorf("endIP is incorrect ip")
	}

	if !subnet.Contains(gateway) {
		return nodeAllocationInfo{}, fmt.Errorf("gateway is outside of the subnet")
	}

	if !subnet.Contains(nodeAllocStart) || !subnet.Contains(nodeAllocEnd) {
		return nodeAllocationInfo{}, fmt.Errorf("invalid allocation allocators: start or end IP is out of the subnet")
	}

	if ip.Cmp(nodeAllocEnd, nodeAllocStart) <= 0 {
		return nodeAllocationInfo{}, fmt.Errorf("invalid allocation allocators: start IP must be less then end IP")
	}

	ipCount := ip.Distance(nodeAllocStart, nodeAllocEnd)
	if ipCount < 1 {
		return nodeAllocationInfo{}, fmt.Errorf("invalid allocation allocators: can't compute count of allocated IPs")
	}
	return nodeAllocationInfo{
		Node:           node,
		Subnet:         subnet,
		Gateway:        gateway,
		allocatedRange: allocatedRange{StartIP: nodeAllocStart, EndIP: nodeAllocEnd},
	}, nil
}
