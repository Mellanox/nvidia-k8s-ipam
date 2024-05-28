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
