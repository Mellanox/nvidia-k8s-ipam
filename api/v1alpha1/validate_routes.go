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
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// validateRoutes validate routes:
// - dst is a valid CIDR
func validateRoutes(routes []Route, network *net.IPNet, defaultGateway bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, r := range routes {
		_, routeNet, err := net.ParseCIDR(r.Dst)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("routes").Index(i).Child("dst"), r.Dst,
				"is invalid subnet"))
		}
		if routeNet != nil && network != nil {
			if (routeNet.IP.To4() != nil) != (network.IP.To4() != nil) {
				allErrs = append(allErrs, field.Invalid(
					fldPath.Child("routes").Index(i).Child("dst"), r.Dst,
					"is not same address family IPv4/IPv6"))
			}
		}
		if routeNet != nil && defaultGateway {
			if isDefaultRoute(routeNet) {
				allErrs = append(allErrs, field.Invalid(
					fldPath.Child("routes").Index(i).Child("dst"), r.Dst,
					"default route is not allowed if 'defaultGateway' is true"))
			}
		}
	}
	return allErrs
}

func isDefaultRoute(ipNet *net.IPNet) bool {
	// Check if it's IPv4 and matches 0.0.0.0/0
	if ipNet.IP.To4() != nil && ipNet.String() == "0.0.0.0/0" {
		return true
	}
	// Check if it's IPv6 and matches ::/0
	if ipNet.IP.To4() == nil && ipNet.String() == "::/0" {
		return true
	}
	return false
}
