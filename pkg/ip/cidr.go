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
	"math/big"
	"net"
)

// NextIP returns IP incremented by 1, if IP is invalid, return nil
func NextIP(ip net.IP) net.IP {
	normalizedIP := normalizeIP(ip)
	if normalizedIP == nil {
		return nil
	}

	i := ipToInt(normalizedIP)
	return intToIP(i.Add(i, big.NewInt(1)), len(normalizedIP) == net.IPv6len)
}

// NextIPWithOffset returns IP incremented by offset, if IP is invalid, return nil
func NextIPWithOffset(ip net.IP, offset int64) net.IP {
	if offset < 0 {
		return nil
	}
	normalizedIP := normalizeIP(ip)
	if normalizedIP == nil {
		return nil
	}

	i := ipToInt(normalizedIP)
	return intToIP(i.Add(i, big.NewInt(offset)), len(normalizedIP) == net.IPv6len)
}

// PrevIP returns IP decremented by 1, if IP is invalid, return nil
func PrevIP(ip net.IP) net.IP {
	normalizedIP := normalizeIP(ip)
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
	normalizedA := normalizeIP(a)
	normalizedB := normalizeIP(b)

	if len(normalizedA) == len(normalizedB) && len(normalizedA) != 0 {
		return ipToInt(normalizedA).Cmp(ipToInt(normalizedB))
	}

	return -2
}

// Distance returns amount of IPs between a and b
// returns -1 if result is negative
// returns -2 if result is too large or IPs are not valid addresses
func Distance(a, b net.IP) int64 {
	normalizedA := normalizeIP(a)
	normalizedB := normalizeIP(b)

	if len(normalizedA) == len(normalizedB) && len(normalizedA) != 0 {
		count := big.NewInt(0).Sub(ipToInt(normalizedB), ipToInt(normalizedA))
		if !count.IsInt64() {
			return -2
		}
		if count.Sign() < 0 {
			return -1
		}
		return count.Int64()
	}
	return -2
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

// normalizeIP will normalize IP by family,
// IPv4 : 4-byte form
// IPv6 : 16-byte form
// others : nil
func normalizeIP(ip net.IP) net.IP {
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
	if !network.Contains(ip) {
		return false
	}
	masked := make(net.IP, len(ip.To4()))
	binary.BigEndian.PutUint32(masked,
		binary.BigEndian.Uint32(network.IP.To4())|^binary.BigEndian.Uint32(net.IP(network.Mask).To4()))
	return ip.Equal(masked)
}
