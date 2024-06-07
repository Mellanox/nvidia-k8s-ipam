/*
 Copyright 2023, NVIDIA CORPORATION & AFFILIATES
 Copyright 2015 CNI authors
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
	"errors"
	"fmt"
	"net"

	current "github.com/containernetworking/cni/pkg/types/100"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

var (
	// ErrNoFreeAddresses is returned is there is no free IP addresses left in the pool
	ErrNoFreeAddresses = errors.New("no free addresses in the allocated range")
)

// IPAllocator is the interface of the allocate package
//
//go:generate mockery --name IPAllocator
type IPAllocator interface {
	// Allocate allocates IP address from the range for the container identified by ID and ifName
	Allocate(id string, ifName string, meta types.ReservationMetadata, staticIP net.IP) (*current.IPConfig, error)
}

type allocator struct {
	rangeSet   *RangeSet
	session    storePkg.Session
	poolKey    string
	exclusions *RangeSet
}

// NewIPAllocator create and initialize a new instance of IP allocator
func NewIPAllocator(s *RangeSet, exclusions *RangeSet,
	poolKey string, session storePkg.Session) IPAllocator {
	return &allocator{
		rangeSet:   s,
		session:    session,
		poolKey:    poolKey,
		exclusions: exclusions,
	}
}

// Allocate allocates an IP
func (a *allocator) Allocate(id string, ifName string,
	meta types.ReservationMetadata, staticIP net.IP) (*current.IPConfig, error) {
	var reservedIP *net.IPNet
	var gw net.IP

	if staticIP != nil {
		return a.allocateStaticIP(id, ifName, meta, staticIP)
	}

	iter := a.getIter()
	for {
		reservedIP, gw = iter.Next()
		if reservedIP == nil {
			return nil, ErrNoFreeAddresses
		}
		if a.exclusions != nil && a.exclusions.Contains(reservedIP.IP) {
			continue
		}
		err := a.session.Reserve(a.poolKey, id, ifName, meta, reservedIP.IP)
		if err == nil {
			break
		}
		if !errors.Is(err, storePkg.ErrIPAlreadyReserved) {
			return nil, err
		}
	}

	return &current.IPConfig{
		Address: *reservedIP,
		Gateway: gw,
	}, nil
}

func (a *allocator) allocateStaticIP(id string, ifName string,
	meta types.ReservationMetadata, staticIP net.IP) (*current.IPConfig, error) {
	for _, r := range *a.rangeSet {
		rangeSubnet := net.IPNet(r.Subnet)
		if !rangeSubnet.Contains(staticIP) {
			continue
		}
		lastReservedIP := a.session.GetLastReservedIP(a.poolKey)
		err := a.session.Reserve(a.poolKey, id, ifName, meta, staticIP)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate staticIP %s: %w", staticIP.String(), err)
		}
		// for static allocations we don't want to change lastReservedIP
		a.session.SetLastReservedIP(a.poolKey, lastReservedIP)
		return &current.IPConfig{
			Address: net.IPNet{IP: staticIP, Mask: r.Subnet.Mask},
			Gateway: r.Gateway,
		}, nil
	}
	return nil, fmt.Errorf("can't find IP range in the allocator for static IP: %s", staticIP.String())
}

// RangeIter implements iterator over the RangeSet
type RangeIter struct {
	rangeSet *RangeSet
	// The current range id
	rangeIdx int
	// Our current position
	cur net.IP
	// The IP where we started iterating; if we hit this again, we're done.
	startIP net.IP
}

// getIter encapsulates the strategy for this allocator.
// We use a round-robin strategy, attempting to evenly use the whole set.
// More specifically, a crash-looping container will not see the same IP until
// the entire range has been run through.
// We may wish to consider avoiding recently-released IPs in the future.
func (a *allocator) getIter() *RangeIter {
	iter := RangeIter{
		rangeSet: a.rangeSet,
	}

	// Round-robin by trying to allocate from the last reserved IP + 1
	startFromLastReservedIP := false

	// We might get a last reserved IP that is wrong if the range indexes changed.
	// This is not critical, we just lose round-robin this one time.
	lastReservedIP := a.session.GetLastReservedIP(a.poolKey)
	if lastReservedIP != nil {
		startFromLastReservedIP = a.rangeSet.Contains(lastReservedIP)
	}

	// Find the range in the set with this IP
	if startFromLastReservedIP {
		for i, r := range *a.rangeSet {
			if r.Contains(lastReservedIP) {
				iter.rangeIdx = i

				// We advance the cursor on every Next(), so the first call
				// to next() will return lastReservedIP + 1
				iter.cur = lastReservedIP
				break
			}
		}
	} else {
		iter.rangeIdx = 0
		iter.startIP = (*a.rangeSet)[0].RangeStart
	}
	return &iter
}

// Next returns the next IP, its mask, and its gateway. Returns nil
// if the iterator has been exhausted
func (i *RangeIter) Next() (*net.IPNet, net.IP) {
	r := (*i.rangeSet)[i.rangeIdx]

	// If this is the first time iterating, and we're not starting in the middle
	// of the range, then start at rangeStart, which is inclusive
	if i.cur == nil {
		i.cur = r.RangeStart
		i.startIP = i.cur
		if r.Gateway != nil && i.cur.Equal(r.Gateway) {
			return i.Next()
		}
		return &net.IPNet{IP: i.cur, Mask: r.Subnet.Mask}, r.Gateway
	}

	nextIP := ip.NextIP(i.cur)
	// If we've reached the end of this range, we need to advance the range
	// RangeEnd is inclusive as well
	if i.cur.Equal(r.RangeEnd) || nextIP == nil {
		i.rangeIdx++
		i.rangeIdx %= len(*i.rangeSet)
		r = (*i.rangeSet)[i.rangeIdx]

		i.cur = r.RangeStart
	} else {
		i.cur = nextIP
	}

	if i.startIP == nil {
		i.startIP = i.cur
	} else if i.cur.Equal(i.startIP) {
		// IF we've looped back to where we started, give up
		return nil, nil
	}

	if r.Gateway != nil && i.cur.Equal(r.Gateway) {
		return i.Next()
	}

	return &net.IPNet{IP: i.cur, Mask: r.Subnet.Mask}, r.Gateway
}

// StartIP returns start IP of the current range
func (i *RangeIter) StartIP() net.IP {
	return i.startIP
}
