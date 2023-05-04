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

package config

import (
	"fmt"
	"math"
	"net"

	cniUtils "github.com/containernetworking/cni/pkg/utils"
	metaValidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	validationField "k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// ConfigMapKey is the name of the key in ConfigMap which store
	// configuration
	ConfigMapKey = "config"
)

// PoolConfig contains configuration for IP pool
type PoolConfig struct {
	// subnet of the pool
	Subnet string `json:"subnet"`
	// amount of IPs to allocate for each node,
	// must be less than amount of available IPs in the subnet
	PerNodeBlockSize int `json:"perNodeBlockSize"`
	// gateway for the pool, defaults to the first IP of the Subnet if not set
	Gateway string `json:"gateway"`
}

// Config contains configuration for IPAM controller
type Config struct {
	// configuration for IP pools
	Pools map[string]PoolConfig `json:"pools"`
	// selector for nodes, if empty match all nodes
	NodeSelector map[string]string `json:"nodeSelector"`
}

// Validate validates IPAM controller config
func (c *Config) Validate() error {
	if len(c.Pools) == 0 {
		return fmt.Errorf("no IP pools in the config")
	}
	if errList := metaValidation.ValidateLabels(c.NodeSelector,
		validationField.NewPath("config", "nodeSelector")); len(errList) > 0 {
		return errList.ToAggregate()
	}
	for poolName, pool := range c.Pools {
		if err := cniUtils.ValidateNetworkName(poolName); err != nil {
			return fmt.Errorf("invalid IP pool name %s, should be compatible with CNI network name", poolName)
		}
		_, network, err := net.ParseCIDR(pool.Subnet)
		if err != nil {
			return fmt.Errorf("IP pool %s contains invalid subnet: %v", poolName, err)
		}

		if pool.PerNodeBlockSize < 2 {
			return fmt.Errorf("perNodeBlockSize should be at least 2")
		}

		setBits, bitsTotal := network.Mask.Size()
		// possibleIPs = net size - network address - broadcast
		possibleIPs := int(math.Pow(2, float64(bitsTotal-setBits))) - 2
		if possibleIPs < pool.PerNodeBlockSize {
			// config is not valid even if only one node exist in the cluster
			return fmt.Errorf("IP pool subnet contains less available IPs then " +
				"requested by perNodeBlockSize parameter")
		}
		if pool.Gateway != "" {
			parsedGW := net.ParseIP(pool.Gateway)
			if len(parsedGW) == 0 {
				return fmt.Errorf("IP pool contains invalid gateway configuration: invalid IP")
			}
			if !network.Contains(parsedGW) {
				return fmt.Errorf("IP pool contains invalid gateway configuration: " +
					"gateway is outside of the subnet")
			}
		}
	}
	return nil
}
