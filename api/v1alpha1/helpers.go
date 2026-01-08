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

package v1alpha1

import (
	"math/big"
	"net"
	"sort"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
)

// GetGatewayForSubnet returns computed gateway for subnet as string, index 0 means no gw
// Examples:
// - subnet 192.168.0.0/24, index = 0, gateway = "" (invalid config, can't use network address as gateway )
// - subnet 192.168.0.0/24, index 1, gateway = 192.168.1.1
// - subnet 192.168.0.0/24, index 10, gateway = 102.168.1.10
// - subnet 192.168.1.2/31, index 0, gateway = 192.168.1.2 (point to point network can use network IP as a gateway)
// - subnet 192.168.33.0/24, index 900, gateway = "" (invalid config, index too large)
func GetGatewayForSubnet(subnet *net.IPNet, index int32) string {
	setBits, bitsTotal := subnet.Mask.Size()
	maxIndex := GetPossibleIPCount(subnet)

	if setBits >= bitsTotal-1 {
		// point to point or single IP
		maxIndex.Sub(maxIndex, big.NewInt(1))
	} else if index == 0 {
		// index 0 can be used only for point to point or single IP networks
		return ""
	}
	if maxIndex.Cmp(big.NewInt(int64(index))) < 0 {
		// index too large
		return ""
	}
	gwIP := ip.NextIPWithOffset(subnet.IP, int64(index))
	if gwIP == nil {
		return ""
	}
	return gwIP.String()
}

// GetPossibleIPCount returns count of IP addresses for hosts in the provided subnet.
// for IPv4: returns amount of IPs - 2 (subnet address and broadcast address)
// for IPv6: returns amount of IPs - 1 (subnet address)
// for point to point subnets(/31 and /127) returns 2
func GetPossibleIPCount(subnet *net.IPNet) *big.Int {
	setBits, bitsTotal := subnet.Mask.Size()
	if setBits == bitsTotal {
		return big.NewInt(1)
	}
	if setBits == bitsTotal-1 {
		// point to point
		return big.NewInt(2)
	}
	ipCount := big.NewInt(0).Exp(big.NewInt(2), big.NewInt(int64(bitsTotal-setBits)), nil)
	if subnet.IP.To4() != nil {
		ipCount.Sub(ipCount, big.NewInt(2))
	} else {
		ipCount.Sub(ipCount, big.NewInt(1))
	}
	return ipCount
}

// ExcludeIndexRangeToExcludeRange converts index-based exclusions to IP-based exclusions.
// IPs are calculated as offset from the provided startIP.
func ExcludeIndexRangeToExcludeRange(excludeIndexRanges []ExcludeIndexRange, startIP string) []ExcludeRange {
	// startIP, endIP and ranges are assumed to be valid and are checked with the pool validate function.
	if len(excludeIndexRanges) == 0 {
		return nil
	}

	rangeStart := net.ParseIP(startIP)
	if rangeStart == nil {
		return nil
	}

	excludeRanges := make([]ExcludeRange, 0, len(excludeIndexRanges))
	for _, excludeIndexRange := range excludeIndexRanges {
		excludeRangeStart := ip.NextIPWithOffset(rangeStart, int64(excludeIndexRange.StartIndex))
		if excludeRangeStart == nil {
			continue
		}
		excludeRangeEnd := ip.NextIPWithOffset(rangeStart, int64(excludeIndexRange.EndIndex))
		if excludeRangeEnd == nil {
			continue
		}

		excludeRanges = append(excludeRanges,
			ExcludeRange{StartIP: excludeRangeStart.String(), EndIP: excludeRangeEnd.String()})
	}

	return excludeRanges
}

// ClampExcludeRange clamps the exclusion range to the provided startIP and endIP range.
// Entries outside the range between startIP and endIP (included) are omitted from the result.
// Entries within the range are clamped to the start and end ip if they end up partly outside the range.
func ClampExcludeRanges(excludeRanges []ExcludeRange, startIP string, endIP string) []ExcludeRange {
	// startIP, endIP and ranges are assumed to be valid and are checked with the pool validate function.
	if len(excludeRanges) == 0 {
		return nil
	}

	rangeStart := net.ParseIP(startIP)
	rangeEnd := net.ParseIP(endIP)
	if rangeStart == nil || rangeEnd == nil {
		return nil
	}

	clampedRanges := make([]ExcludeRange, 0)
	for _, excludeRange := range excludeRanges {
		excludeRangeStart := net.ParseIP(excludeRange.StartIP)
		excludeRangeEnd := net.ParseIP(excludeRange.EndIP)

		if excludeRangeStart == nil || excludeRangeEnd == nil {
			continue
		}

		// if exclude range start ip is after range end ip or exclude range end ip is before range start ip, skip
		if ip.Cmp(excludeRangeStart, rangeEnd) > 0 || ip.Cmp(excludeRangeEnd, rangeStart) < 0 {
			continue
		}

		// clamp to the start and end ip
		if ip.Cmp(excludeRangeStart, rangeStart) < 0 {
			excludeRangeStart = rangeStart
		}
		if ip.Cmp(excludeRangeEnd, rangeEnd) > 0 {
			excludeRangeEnd = rangeEnd
		}

		clampedRanges = append(clampedRanges,
			ExcludeRange{StartIP: excludeRangeStart.String(), EndIP: excludeRangeEnd.String()})
	}

	return clampedRanges
}

// MergeExcludeRanges merges overlapping exclude ranges into a single range.
// it returns a slice of non-overlapping exclude ranges.
func MergeExcludeRanges(excludeRanges []ExcludeRange) []ExcludeRange {
	// excludeRanges are assumed to be valid and are checked with the pool validate function.
	if len(excludeRanges) == 0 {
		return nil
	}

	// sort by start IP
	type excludeRangeIP struct {
		startIP net.IP
		endIP   net.IP
	}

	excludeRangeIPs := make([]excludeRangeIP, 0, len(excludeRanges))
	for _, excludeRange := range excludeRanges {
		start := net.ParseIP(excludeRange.StartIP)
		end := net.ParseIP(excludeRange.EndIP)
		if start == nil || end == nil {
			continue
		}

		excludeRangeIPs = append(excludeRangeIPs, excludeRangeIP{startIP: start, endIP: end})
	}

	if len(excludeRangeIPs) == 0 {
		return nil
	}

	sort.Slice(excludeRangeIPs, func(i, j int) bool {
		return ip.Cmp(excludeRangeIPs[i].startIP, excludeRangeIPs[j].startIP) < 0
	})

	// merge ranges
	current := excludeRangeIPs[0]
	mergedRangesIPs := make([]excludeRangeIP, 0)
	for i := 1; i < len(excludeRangeIPs); i++ {
		next := excludeRangeIPs[i]

		// case1: current range overlaps with next range, merge them
		// case2: next range starts immediately after current range ends, merge them
		if ip.Cmp(current.endIP, next.startIP) >= 0 ||
			ip.Cmp(ip.NextIP(current.endIP), next.startIP) == 0 {
			if ip.Cmp(current.endIP, next.endIP) < 0 {
				current.endIP = next.endIP
			}
		} else {
			mergedRangesIPs = append(mergedRangesIPs, excludeRangeIP{startIP: current.startIP, endIP: current.endIP})
			current = next
		}
	}
	// add the last merged element
	mergedRangesIPs = append(mergedRangesIPs, excludeRangeIP{startIP: current.startIP, endIP: current.endIP})

	// convert back to ExcludeRange
	mergedRanges := make([]ExcludeRange, 0, len(mergedRangesIPs))
	for _, mergedRangeIP := range mergedRangesIPs {
		mergedRanges = append(mergedRanges,
			ExcludeRange{StartIP: mergedRangeIP.startIP.String(), EndIP: mergedRangeIP.endIP.String()})
	}
	return mergedRanges
}

// Contains checks if the given IP is contained in exclude range.
func (er *ExcludeRange) Contains(ipp net.IP) bool {
	if ipp == nil {
		return false
	}

	start := net.ParseIP(er.StartIP)
	end := net.ParseIP(er.EndIP)
	if start == nil || end == nil {
		return false
	}

	return ip.Cmp(start, ipp) <= 0 && ip.Cmp(end, ipp) >= 0
}
