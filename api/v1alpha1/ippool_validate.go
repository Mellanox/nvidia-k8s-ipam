/*
 Copyright 2023, NVIDIA CORPORATION & AFFILIATES
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

	cniUtils "github.com/containernetworking/cni/pkg/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate contains validation for the object fields
func (r *IPPool) Validate() field.ErrorList {
	errList := field.ErrorList{}
	if err := cniUtils.ValidateNetworkName(r.Name); err != nil {
		errList = append(errList, field.Invalid(
			field.NewPath("metadata", "name"), r.Name,
			"invalid IP pool name, should be compatible with CNI network name"))
	}
	_, network, err := net.ParseCIDR(r.Spec.Subnet)
	if err != nil {
		errList = append(errList, field.Invalid(
			field.NewPath("spec", "subnet"), r.Spec.Subnet, "is invalid subnet"))
	}

	if r.Spec.PerNodeBlockSize < 2 {
		errList = append(errList, field.Invalid(
			field.NewPath("spec", "perNodeBlockSize"),
			r.Spec.PerNodeBlockSize, "must be at least 2"))
	}

	if network != nil && r.Spec.PerNodeBlockSize >= 2 {
		if GetPossibleIPCount(network).Cmp(big.NewInt(int64(r.Spec.PerNodeBlockSize))) < 0 {
			// config is not valid even if only one node exist in the cluster
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "perNodeBlockSize"), r.Spec.PerNodeBlockSize,
				"is larger then amount of IPs available in the subnet"))
		}
	}
	if network != nil {
		errList = append(errList, validateExclusions(network, r.Spec.Exclusions, field.NewPath("spec"))...)
	}
	var parsedGW net.IP
	if r.Spec.Gateway != "" {
		parsedGW = net.ParseIP(r.Spec.Gateway)
		if len(parsedGW) == 0 {
			errList = append(errList, field.Invalid(
				field.NewPath("spec", "gateway"), r.Spec.Gateway,
				"is invalid IP address"))
		}
	}

	if network != nil && len(parsedGW) != 0 && !network.Contains(parsedGW) {
		errList = append(errList, field.Invalid(
			field.NewPath("spec", "gateway"), r.Spec.Gateway,
			"is not part of the subnet"))
	}

	if r.Spec.NodeSelector != nil {
		errList = append(errList, validateNodeSelector(r.Spec.NodeSelector, field.NewPath("spec"))...)
	}
	return errList
}
