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
	"fmt"
	"net"

	"github.com/containernetworking/cni/pkg/types"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
)

// Range contains IP range configuration
type Range struct {
	RangeStart net.IP      `json:"rangeStart,omitempty"` // The first ip, inclusive
	RangeEnd   net.IP      `json:"rangeEnd,omitempty"`   // The last ip, inclusive
	Subnet     types.IPNet `json:"subnet"`
	Gateway    net.IP      `json:"gateway,omitempty"`
}

// Canonicalize takes a given range and ensures that all information is consistent,
// filling out Start, End, and Gateway with sane values if missing
func (r *Range) Canonicalize() error {
	if err := CanonicalizeIP(&r.Subnet.IP); err != nil {
		return err
	}

	// Can't create an allocator for /32 or /128 networks (single IP)
	ones, masklen := r.Subnet.Mask.Size()
	if ones > masklen-1 {
		return fmt.Errorf("network %s too small to allocate from", (*net.IPNet)(&r.Subnet).String())
	}

	if len(r.Subnet.IP) != len(r.Subnet.Mask) {
		return fmt.Errorf("IPNet IP and Mask version mismatch")
	}

	// Ensure Subnet IP is the network address, not some other address
	networkIP := r.Subnet.IP.Mask(r.Subnet.Mask)
	if !r.Subnet.IP.Equal(networkIP) {
		return fmt.Errorf("network has host bits set. "+
			"For a subnet mask of length %d the network address is %s", ones, networkIP.String())
	}

	// validate Gateway only if set
	if r.Gateway != nil {
		if err := CanonicalizeIP(&r.Gateway); err != nil {
			return err
		}
	}

	// RangeStart: If specified, make sure it's sane (inside the subnet),
	// otherwise use the first free IP (i.e. .1) - this will conflict with the
	// gateway but we skip it in the iterator
	if r.RangeStart != nil {
		if err := CanonicalizeIP(&r.RangeStart); err != nil {
			return err
		}

		if !r.Contains(r.RangeStart) {
			return fmt.Errorf("RangeStart %s not in network %s", r.RangeStart.String(), (*net.IPNet)(&r.Subnet).String())
		}
	} else {
		if isPointToPoint(r.Subnet) {
			r.RangeStart = r.Subnet.IP
		} else {
			r.RangeStart = ip.NextIP(r.Subnet.IP)
			if r.RangeStart == nil {
				return fmt.Errorf("computed RangeStart is not a valid IP")
			}
		}
	}

	// RangeEnd: If specified, verify sanity. Otherwise, add a sensible default
	// (e.g. for a /24: .254 if IPv4, ::255 if IPv6)
	if r.RangeEnd != nil {
		if err := CanonicalizeIP(&r.RangeEnd); err != nil {
			return err
		}

		if !r.Contains(r.RangeEnd) {
			return fmt.Errorf("RangeEnd %s not in network %s", r.RangeEnd.String(), (*net.IPNet)(&r.Subnet).String())
		}
	} else {
		r.RangeEnd = lastIP(r.Subnet)
	}

	return nil
}

// Contains checks if a given ip is a valid, allocatable address in a given Range
func (r *Range) Contains(addr net.IP) bool {
	if err := CanonicalizeIP(&addr); err != nil {
		return false
	}

	subnet := (net.IPNet)(r.Subnet)

	// Not the same address family
	if len(addr) != len(r.Subnet.IP) {
		return false
	}

	// Not in network
	if !subnet.Contains(addr) {
		return false
	}

	if ip.Cmp(addr, r.RangeStart) < 0 {
		// Before the range start
		return false
	}

	if ip.Cmp(addr, r.RangeEnd) > 0 {
		// After the  range end
		return false
	}

	return true
}

// Overlaps returns true if there is any overlap between ranges
func (r *Range) Overlaps(r1 *Range) bool {
	// different families
	if len(r.RangeStart) != len(r1.RangeStart) {
		return false
	}

	return r.Contains(r1.RangeStart) ||
		r.Contains(r1.RangeEnd) ||
		r1.Contains(r.RangeStart) ||
		r1.Contains(r.RangeEnd)
}

// String returns string representation of the Range
func (r *Range) String() string {
	return fmt.Sprintf("%s-%s", r.RangeStart.String(), r.RangeEnd.String())
}

// CanonicalizeIP makes sure a provided ip is in standard form
func CanonicalizeIP(addr *net.IP) error {
	if addr == nil {
		return fmt.Errorf("IP can't be nil")
	}
	normalizedIP := ip.NormalizeIP(*addr)
	if normalizedIP == nil {
		return fmt.Errorf("IP %s not v4 nor v6", *addr)
	}
	*addr = normalizedIP
	return nil
}

// Returns true if IPNet is point to point (/31 or /127)
func isPointToPoint(subnet types.IPNet) bool {
	ones, maskLen := subnet.Mask.Size()
	return ones == maskLen-1
}

// Determine the last IP of a subnet, excluding the broadcast if IPv4 (if not /31 net)
func lastIP(subnet types.IPNet) net.IP {
	var end net.IP
	for i := 0; i < len(subnet.IP); i++ {
		end = append(end, subnet.IP[i]|^subnet.Mask[i])
	}
	if subnet.IP.To4() != nil && !isPointToPoint(subnet) {
		end[3]--
	}

	return end
}
