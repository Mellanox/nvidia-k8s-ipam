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
	"math"
	"math/big"
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ip"
)

// validateExclusions validate exclusions:
// - startIP and endIPs should be valid ip addresses from subnet,
// - endIP should be equal or greater than startIP
// there is no need to validate that ranges have no overlaps
func validateExclusions(subnet *net.IPNet, exclusions []ExcludeRange, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, e := range exclusions {
		startIP := net.ParseIP(e.StartIP)
		if len(startIP) == 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("exclusions").Index(i).Child("startIP"), e.StartIP,
				"is invalid IP address"))
		} else if !subnet.Contains(startIP) {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("exclusions").Index(i).Child("startIP"), e.StartIP,
				fmt.Sprintf("is not part of the %s subnet", subnet.String())))
		}
		endIP := net.ParseIP(e.EndIP)
		if len(endIP) == 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("exclusions").Index(i).Child("endIP"), e.EndIP,
				"is invalid IP address"))
		} else if !subnet.Contains(endIP) {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("exclusions").Index(i).Child("endIP"), e.EndIP,
				fmt.Sprintf("is not part of the %s subnet", subnet.String())))
		}
		if len(startIP) > 0 && len(endIP) > 0 && ip.Cmp(endIP, startIP) < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("exclusions").Index(i), e,
				"endIP should be equal or greater than startIP"))
		}
	}
	return allErrs
}

// validatePerNodeExclusions validate per-node index exclusions for CIDRPool:
// - startIndex and endIndex should be non-negative
// - endIndex should be equal or greater than startIndex
// - indices should be within the valid range for the per-node subnet
// there is no need to validate that ranges have no overlaps
func validatePerNodeExclusions(
	subnet *net.IPNet, perNodeNetworkPrefix int32,
	exclusions []ExcludeIndexRange, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// Calculate the maximum valid index for the per-node subnet
	// For a /prefix network, we have 2^(address_bits - prefix) IPs
	_, bitsTotal := subnet.Mask.Size()
	hostBits := bitsTotal - int(perNodeNetworkPrefix)

	// Use big.Int to calculate max index safely (avoids overflow)
	// then clamp to int32 range
	ipCount := big.NewInt(0).Exp(big.NewInt(2), big.NewInt(int64(hostBits)), nil)
	maxIndexBig := big.NewInt(0).Sub(ipCount, big.NewInt(1))
	maxIndex := int32(math.MaxInt32)
	if maxIndexBig.IsInt64() && maxIndexBig.Int64() < int64(math.MaxInt32) {
		maxIndex = int32(maxIndexBig.Int64()) //nolint:gosec // checked above
	}

	for i, e := range exclusions {
		if e.StartIndex < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i).Child("startIndex"), e.StartIndex,
				"must be non-negative"))
		}
		if e.EndIndex < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i).Child("endIndex"), e.EndIndex,
				"must be non-negative"))
		}
		if e.EndIndex < e.StartIndex {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i), e,
				"endIndex must be greater than or equal to startIndex"))
		}
		if e.StartIndex > maxIndex {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i).Child("startIndex"), e.StartIndex,
				fmt.Sprintf("index is outside of the per-node subnet range (max: %d)", maxIndex)))
		}
		if e.EndIndex > maxIndex {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i).Child("endIndex"), e.EndIndex,
				fmt.Sprintf("index is outside of the per-node subnet range (max: %d)", maxIndex)))
		}
	}
	return allErrs
}

// validatePerNodeExclusionsForBlockSize validate per-node index exclusions for IPPool:
// - startIndex and endIndex should be non-negative
// - endIndex should be equal or greater than startIndex
// - indices should be within the valid range based on perNodeBlockSize
// there is no need to validate that ranges have no overlaps
func validatePerNodeExclusionsForBlockSize(
	perNodeBlockSize int, exclusions []ExcludeIndexRange, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// For IPPool, the max index is perNodeBlockSize - 1
	// Clamp to int32 max to match API type
	maxIndex := int32(perNodeBlockSize - 1) //nolint:gosec // clamped below
	if perNodeBlockSize-1 > math.MaxInt32 {
		maxIndex = math.MaxInt32
	}

	for i, e := range exclusions {
		if e.StartIndex < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i).Child("startIndex"), e.StartIndex,
				"must be non-negative"))
		}
		if e.EndIndex < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i).Child("endIndex"), e.EndIndex,
				"must be non-negative"))
		}
		if e.EndIndex < e.StartIndex {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i), e,
				"endIndex must be greater than or equal to startIndex"))
		}
		if e.StartIndex > maxIndex {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i).Child("startIndex"), e.StartIndex,
				fmt.Sprintf("index is outside of the per-node block range (max: %d)", maxIndex)))
		}
		if e.EndIndex > maxIndex {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("perNodeExclusions").Index(i).Child("endIndex"), e.EndIndex,
				fmt.Sprintf("index is outside of the per-node block range (max: %d)", maxIndex)))
		}
	}
	return allErrs
}
