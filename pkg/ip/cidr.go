// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ip

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
)

// NextIP returns IP incremented by 1, if IP is invalid, return nil
func NextIP(ip net.IP) net.IP {
	normalizedIP := NormalizeIP(ip)
	if normalizedIP == nil {
		return nil
	}

	i := ipToInt(normalizedIP)
	return intToIP(i.Add(i, big.NewInt(1)), len(normalizedIP) == net.IPv6len)
}

// NextIPWithOffset returns IP incremented by offset.
func NextIPWithOffset(ip net.IP, offset *big.Int) (net.IP, error) {
	if offset == nil || offset.Sign() < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}
	normalizedIP := NormalizeIP(ip)
	if normalizedIP == nil {
		return nil, fmt.Errorf("invalid IP address")
	}

	i := ipToInt(normalizedIP)
	nextIP := intToIP(i.Add(i, offset), len(normalizedIP) == net.IPv6len)
	if nextIP == nil {
		return nil, fmt.Errorf("IP address overflow")
	}
	return nextIP, nil
}

// PrevIP returns IP decremented by 1, if IP is invalid, return nil
func PrevIP(ip net.IP) net.IP {
	normalizedIP := NormalizeIP(ip)
	if normalizedIP == nil {
		return nil
	}

	i := ipToInt(normalizedIP)
	return intToIP(i.Sub(i, big.NewInt(1)), len(normalizedIP) == net.IPv6len)
}

// Cmp compares two IPs, returning the usual ordering:
// a < b : -1
// a == b : 0
// a > b : 1
// incomparable : -2
func Cmp(a, b net.IP) int {
	normalizedA := NormalizeIP(a)
	normalizedB := NormalizeIP(b)

	if len(normalizedA) == len(normalizedB) && len(normalizedA) != 0 {
		return ipToInt(normalizedA).Cmp(ipToInt(normalizedB))
	}

	return -2
}

// Distance returns the non-negative distance between two addresses of the same family.
func Distance(a, b net.IP) (*big.Int, error) {
	normalizedA := NormalizeIP(a)
	normalizedB := NormalizeIP(b)
	if len(normalizedA) == 0 || len(normalizedB) == 0 {
		return nil, fmt.Errorf("invalid IP address")
	}
	if len(normalizedA) != len(normalizedB) {
		return nil, fmt.Errorf("IP address families do not match")
	}

	distance := new(big.Int).Sub(ipToInt(normalizedB), ipToInt(normalizedA))
	if distance.Sign() < 0 {
		return nil, fmt.Errorf("second IP address must not precede first IP address")
	}
	return distance, nil
}

func ipToInt(ip net.IP) *big.Int {
	return big.NewInt(0).SetBytes(ip)
}

func intToIP(i *big.Int, isIPv6 bool) net.IP {
	if i.Sign() < 0 {
		return nil
	}
	intBytes := i.Bytes()
	ipLen := net.IPv4len
	if isIPv6 {
		ipLen = net.IPv6len
	}
	if len(intBytes) == ipLen {
		return intBytes
	}
	if len(intBytes) > ipLen {
		return nil
	}
	zeroes := ipLen - len(intBytes)
	return append(make([]byte, zeroes), intBytes...)
}

// NormalizeIP will normalize IP by family,
// IPv4 : 4-byte form
// IPv6 : 16-byte form
// others : nil
func NormalizeIP(ip net.IP) net.IP {
	if ipTo4 := ip.To4(); ipTo4 != nil {
		return ipTo4
	}
	return ip.To16()
}

// IsBroadcast returns true if provided IP is IPv4 Broadcast ip of the network
func IsBroadcast(ip net.IP, network *net.IPNet) bool {
	if network == nil {
		return false
	}
	if ip.To4() == nil {
		// no broadcast IPs in ipv6
		return false
	}
	if network.IP.To4() == nil {
		return false
	}
	if IsPointToPointSubnet(network) || IsSingleIPSubnet(network) {
		return false
	}
	if !network.Contains(ip) {
		return false
	}
	masked := make(net.IP, len(ip.To4()))
	binary.BigEndian.PutUint32(masked,
		binary.BigEndian.Uint32(network.IP.To4())|^binary.BigEndian.Uint32(net.IP(network.Mask).To4()))
	return ip.Equal(masked)
}

// IsPointToPointSubnet returns true if the network is point to point (/31 or /127)
func IsPointToPointSubnet(network *net.IPNet) bool {
	ones, maskLen := network.Mask.Size()
	return ones == maskLen-1
}

// IsSingleIPSubnet returns true if the network is a single IP subnet (/32 or /128)
func IsSingleIPSubnet(network *net.IPNet) bool {
	ones, maskLen := network.Mask.Size()
	return ones == maskLen
}

// LastIP returns the last IP of a subnet, excluding the broadcast if IPv4 (if not /31 net)
func LastIP(network *net.IPNet) net.IP {
	if IsSingleIPSubnet(network) {
		return network.IP
	}
	var end net.IP
	for i := 0; i < len(network.IP); i++ {
		end = append(end, network.IP[i]|^network.Mask[i])
	}
	if network.IP.To4() != nil && !IsPointToPointSubnet(network) {
		end[3]--
	}
	return end
}

// SubnetIterator iterates over fixed-size prefixes in a network and can skip large address intervals in constant time.
type SubnetIterator struct {
	isIPv6         bool
	netBitsTotal   int
	prefixSize     int32
	networkIPAsInt *big.Int
	subnetIPCount  *big.Int
	subnetCount    *big.Int
	nextIndex      *big.Int
}

// NewSubnetIterator creates an iterator over prefixSize subnets in network.
func NewSubnetIterator(network *net.IPNet, prefixSize int32) (*SubnetIterator, error) {
	if network == nil {
		return nil, fmt.Errorf("network must not be nil")
	}
	networkOnes, netBitsTotal := network.Mask.Size()
	if netBitsTotal != net.IPv4len*8 && netBitsTotal != net.IPv6len*8 {
		return nil, fmt.Errorf("network mask must be a valid IPv4 or IPv6 mask")
	}
	//nolint: gosec
	if prefixSize < int32(networkOnes) || prefixSize > int32(netBitsTotal) {
		return nil, fmt.Errorf("prefix size must be between %d and %d", networkOnes, netBitsTotal)
	}

	var normalizedNetworkIP net.IP
	if netBitsTotal == net.IPv4len*8 {
		normalizedNetworkIP = network.IP.To4()
	} else if network.IP.To4() == nil {
		normalizedNetworkIP = network.IP.To16()
	}
	if normalizedNetworkIP == nil {
		return nil, fmt.Errorf("network IP address does not match the mask")
	}
	if !normalizedNetworkIP.Equal(normalizedNetworkIP.Mask(network.Mask)) {
		return nil, fmt.Errorf("network IP address has host bits set")
	}

	return &SubnetIterator{
		isIPv6:         netBitsTotal == net.IPv6len*8,
		netBitsTotal:   netBitsTotal,
		prefixSize:     prefixSize,
		networkIPAsInt: new(big.Int).SetBytes(normalizedNetworkIP),
		subnetIPCount:  new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(netBitsTotal)-int64(prefixSize)), nil),
		subnetCount:    new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(prefixSize)-int64(networkOnes)), nil),
		nextIndex:      big.NewInt(0),
	}, nil
}

// Next returns the next prefix, or nil after the iterator is exhausted.
func (i *SubnetIterator) Next() *net.IPNet {
	if i == nil || i.nextIndex.Cmp(i.subnetCount) >= 0 {
		return nil
	}
	subnetOffset := new(big.Int).Mul(i.subnetIPCount, i.nextIndex)
	subnetIPAsInt := new(big.Int).Add(i.networkIPAsInt, subnetOffset)
	i.nextIndex.Add(i.nextIndex, big.NewInt(1))
	subnetIP := intToIP(subnetIPAsInt, i.isIPv6)
	if subnetIP == nil {
		return nil
	}
	return &net.IPNet{IP: subnetIP, Mask: net.CIDRMask(int(i.prefixSize), i.netBitsTotal)}
}

// AdvancePastIP moves the iterator to the prefix containing the address immediately after end.
// It never moves the iterator backwards. If end is the last address or beyond the iterator network, the iterator is
// exhausted.
func (i *SubnetIterator) AdvancePastIP(end net.IP) error {
	if i == nil {
		return fmt.Errorf("invalid subnet iterator")
	}
	var normalizedEnd net.IP
	if i.isIPv6 {
		if end.To4() == nil {
			normalizedEnd = end.To16()
		}
	} else {
		normalizedEnd = end.To4()
	}
	if normalizedEnd == nil {
		return fmt.Errorf("IP address family does not match iterator network")
	}
	nextIP := NextIP(normalizedEnd)
	if nextIP == nil {
		i.nextIndex.Set(i.subnetCount)
		return nil
	}
	distance := new(big.Int).Sub(ipToInt(nextIP), i.networkIPAsInt)
	if distance.Sign() < 0 {
		return nil
	}
	targetIndex := new(big.Int).Quo(distance, i.subnetIPCount)
	if targetIndex.Cmp(i.nextIndex) > 0 {
		i.nextIndex.Set(targetIndex)
	}
	if i.nextIndex.Cmp(i.subnetCount) > 0 {
		i.nextIndex.Set(i.subnetCount)
	}
	return nil
}
