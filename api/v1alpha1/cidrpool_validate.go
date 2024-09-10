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
	"fmt"
	"net"

	cniUtils "github.com/containernetworking/cni/pkg/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate implements validation for the object fields
func (r *CIDRPool) Validate() field.ErrorList {
	errList := field.ErrorList{}
	if err := cniUtils.ValidateNetworkName(r.Name); err != nil {
		errList = append(errList, field.Invalid(
			field.NewPath("metadata", "name"), r.Name,
			"invalid CIDR pool name, should be compatible with CNI network name"))
	}
	errList = append(errList, r.validateCIDR()...)
	if r.Spec.NodeSelector != nil {
		errList = append(errList, validateNodeSelector(r.Spec.NodeSelector, field.NewPath("spec"))...)
	}
	if r.Spec.GatewayIndex == nil {
		if r.Spec.DefaultGateway {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "defaultGateway"), r.Spec.DefaultGateway,
				"cannot be true if spec.gatewayIndex is not set"))
		}
		if len(r.Spec.Routes) > 0 {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "routes"), r.Spec.Routes,
				"cannot be set if spec.gatewayIndex is not set"))
		}
	}
	_, network, _ := net.ParseCIDR(r.Spec.CIDR)
	errList = append(errList, validateRoutes(r.Spec.Routes, network, r.Spec.DefaultGateway, field.NewPath("spec"))...)
	return errList
}

// validate IP configuration of the CIDR pool
func (r *CIDRPool) validateCIDR() field.ErrorList {
	netIP, network, err := net.ParseCIDR(r.Spec.CIDR)
	if err != nil {
		return field.ErrorList{field.Invalid(field.NewPath("spec", "cidr"), r.Spec.CIDR, "is invalid cidr")}
	}
	if !netIP.Equal(network.IP) {
		return field.ErrorList{field.Invalid(field.NewPath("spec", "cidr"), r.Spec.CIDR, "network prefix has host bits set")}
	}

	setBits, bitsTotal := network.Mask.Size()
	if setBits == bitsTotal {
		return field.ErrorList{field.Invalid(
			field.NewPath("spec", "cidr"), r.Spec.CIDR, "single IP prefixes are not supported")}
	}

	if r.Spec.GatewayIndex != nil && *r.Spec.GatewayIndex < 0 {
		return field.ErrorList{field.Invalid(
			field.NewPath("spec", "gatewayIndex"), r.Spec.GatewayIndex, "must not be negative")}
	}

	if r.Spec.PerNodeNetworkPrefix < 0 {
		return field.ErrorList{field.Invalid(
			field.NewPath("spec", "perNodeNetworkPrefix"), r.Spec.PerNodeNetworkPrefix, "must not be negative")}
	}

	if r.Spec.PerNodeNetworkPrefix == 0 ||
		r.Spec.PerNodeNetworkPrefix >= int32(bitsTotal) ||
		r.Spec.PerNodeNetworkPrefix < int32(setBits) {
		return field.ErrorList{field.Invalid(
			field.NewPath("spec", "perNodeNetworkPrefix"),
			r.Spec.PerNodeNetworkPrefix, "must be less or equal than network prefix size in the \"cidr\" field")}
	}

	errList := field.ErrorList{}
	firstNodePrefix := &net.IPNet{IP: network.IP, Mask: net.CIDRMask(int(r.Spec.PerNodeNetworkPrefix), bitsTotal)}
	if r.Spec.GatewayIndex != nil && GetGatewayForSubnet(firstNodePrefix, *r.Spec.GatewayIndex) == "" {
		errList = append(errList, field.Invalid(
			field.NewPath("spec", "gatewayIndex"),
			r.Spec.GatewayIndex, "gateway index is outside of the node prefix"))
	}
	errList = append(errList, validateExclusions(network, r.Spec.Exclusions, field.NewPath("spec"))...)
	errList = append(errList, r.validateStaticAllocations(network)...)
	return errList
}

// validateStatic allocations:
// - entries should be uniq (nodeName, prefix)
// - prefix should have the right size
// - prefix should be part of the pool cidr
// - gateway should be part of the prefix
func (r *CIDRPool) validateStaticAllocations(cidr *net.IPNet) field.ErrorList {
	errList := field.ErrorList{}

	nodes := map[string]uint{}
	prefixes := map[string]uint{}

	_, parentCIDRTotalBits := cidr.Mask.Size()

	for i, alloc := range r.Spec.StaticAllocations {
		if alloc.NodeName != "" {
			nodes[alloc.NodeName]++
		}
		netIP, nodePrefix, err := net.ParseCIDR(alloc.Prefix)
		if err != nil {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "staticAllocations").Index(i).Child("prefix"), alloc.Prefix,
				"is not a valid network prefix"))
			continue
		}
		if !netIP.Equal(nodePrefix.IP) {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "staticAllocations").Index(i).Child("prefix"), alloc.Prefix,
				"network prefix has host bits set"))
			continue
		}

		prefixes[nodePrefix.String()]++

		if !cidr.Contains(nodePrefix.IP) {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "staticAllocations").Index(i).Child("prefix"), alloc.Prefix,
				"prefix is not part of the pool cidr"))
			continue
		}

		nodePrefixOnes, nodePrefixTotalBits := nodePrefix.Mask.Size()
		if parentCIDRTotalBits != nodePrefixTotalBits {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "staticAllocations").Index(i).Child("prefix"), alloc.Prefix,
				"ip family doesn't match the pool cidr"))
			continue
		}
		if nodePrefixOnes != int(r.Spec.PerNodeNetworkPrefix) {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "staticAllocations").Index(i).Child("prefix"), alloc.Prefix,
				"prefix size doesn't match spec.perNodeNetworkPrefix"))
			continue
		}

		if alloc.Gateway != "" {
			gwIP := net.ParseIP(alloc.Gateway)
			if len(gwIP) == 0 {
				errList = append(errList, field.Invalid(
					field.NewPath("spec", "staticAllocations").Index(i).Child("gateway"), alloc.Gateway,
					"is not a valid IP"))
				continue
			}
			if !nodePrefix.Contains(gwIP) {
				errList = append(errList, field.Invalid(
					field.NewPath("spec", "staticAllocations").Index(i).Child("gateway"), alloc.Gateway,
					"is outside of the node prefix"))
				continue
			}
		}
	}
	for k, v := range nodes {
		if v > 1 {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "staticAllocations"), r.Spec.StaticAllocations,
				fmt.Sprintf("contains multiple entries for node %s", k)))
		}
	}
	for k, v := range prefixes {
		if v > 1 {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "staticAllocations"), r.Spec.StaticAllocations,
				fmt.Sprintf("contains multiple entries for prefix %s", k)))
		}
	}
	return errList
}

// Validate checks that CIDRPoolAllocation is a valid allocation for provided pool,
// it is expected that provided CIDRPool is already validated
//
//nolint:gocyclo
func (a *CIDRPoolAllocation) Validate(pool *CIDRPool) field.ErrorList {
	errList := field.ErrorList{}
	if a.NodeName == "" {
		errList = append(errList, field.Invalid(
			field.NewPath("nodeName"), a.NodeName, "can't be empty"))
	}
	if a.Prefix == "" {
		errList = append(errList, field.Invalid(
			field.NewPath("prefix"), a.Prefix, "can't be empty"))
	}
	if len(errList) > 0 {
		return errList
	}

	netIP, prefixNetwork, err := net.ParseCIDR(a.Prefix)
	if err != nil {
		return field.ErrorList{field.Invalid(
			field.NewPath("prefix"), a.Prefix, "is not a valid network prefix")}
	}
	if !netIP.Equal(prefixNetwork.IP) {
		return field.ErrorList{field.Invalid(field.NewPath("prefix"), a.Prefix, "network prefix has host bits set")}
	}

	computedGW := ""
	if pool.Spec.GatewayIndex != nil {
		computedGW = GetGatewayForSubnet(prefixNetwork, *pool.Spec.GatewayIndex)
	}

	// check static allocations first
	for _, staticAlloc := range pool.Spec.StaticAllocations {
		errList := field.ErrorList{}
		if a.NodeName == staticAlloc.NodeName {
			if a.Prefix != staticAlloc.Prefix {
				errList = append(errList, field.Invalid(
					field.NewPath("prefix"), a.Prefix,
					fmt.Sprintf("doesn't match prefix from static allocation %s", staticAlloc.Prefix)))
			}
			if staticAlloc.Gateway != "" && a.Gateway != staticAlloc.Gateway {
				errList = append(errList, field.Invalid(
					field.NewPath("gateway"), a.Gateway,
					fmt.Sprintf("doesn't match gateway from static allocation %s", staticAlloc.Gateway)))
			}
			if staticAlloc.Gateway == "" {
				if computedGW != "" && a.Gateway != computedGW {
					errList = append(errList, field.Invalid(
						field.NewPath("gateway"), a.Gateway,
						fmt.Sprintf("doesn't match computed gateway %s", computedGW)))
				}
				if computedGW == "" && a.Gateway != "" {
					errList = append(errList, field.Invalid(
						field.NewPath("gateway"), a.Gateway, "gateway expected to be empty"))
				}
			}
			if len(errList) != 0 {
				return errList
			}
			// allocation match the static allocation, no need for extra validation, because it is
			// expected that the pool.staticAllocations were already validated.
			return nil
		}
		if a.Prefix == staticAlloc.Prefix {
			return field.ErrorList{field.Invalid(
				field.NewPath("prefix"), a.Prefix,
				fmt.Sprintf("is statically allocated for different node: %s", staticAlloc.NodeName))}
		}
	}

	_, cidr, _ := net.ParseCIDR(pool.Spec.CIDR)
	_, parentCIDRTotalBits := cidr.Mask.Size()

	if !cidr.Contains(prefixNetwork.IP) {
		return field.ErrorList{field.Invalid(
			field.NewPath("prefix"), a.Prefix,
			"is not part of the pool cidr")}
	}
	nodePrefixOnes, nodePrefixTotalBits := prefixNetwork.Mask.Size()
	if parentCIDRTotalBits != nodePrefixTotalBits {
		return field.ErrorList{field.Invalid(
			field.NewPath("prefix"), a.Prefix,
			"ip family is not match with the pool cidr")}
	}
	if nodePrefixOnes != int(pool.Spec.PerNodeNetworkPrefix) {
		return field.ErrorList{field.Invalid(
			field.NewPath("prefix"), a.Prefix,
			"prefix size doesn't match spec.perNodeNetworkPrefix")}
	}

	if computedGW != "" && a.Gateway != computedGW {
		return field.ErrorList{field.Invalid(
			field.NewPath("gateway"), a.Gateway,
			fmt.Sprintf("doesn't match computed gateway %s", computedGW))}
	}
	if computedGW == "" && a.Gateway != "" {
		return field.ErrorList{field.Invalid(
			field.NewPath("gateway"), a.Gateway, "gateway expected to be empty")}
	}

	// check for conflicting entries (all field should be uniq)
	alreadyFound := false
	for _, e := range pool.Status.Allocations {
		if (a.Gateway != "" && a.Gateway == e.Gateway) || a.Prefix == e.Prefix || a.NodeName == e.NodeName {
			if alreadyFound {
				return field.ErrorList{field.Invalid(
					field.NewPath("status"), a, fmt.Sprintf("conflicting allocation found in the pool: %v", e))}
			}
			alreadyFound = true
		}
	}
	return nil
}
