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
