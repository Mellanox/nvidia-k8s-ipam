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

package types

import (
	"path/filepath"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

// TODO: do we want to support Routes ? DNS entires from ResolvConf as host-local CNI ?

type HostLocalNetConf struct {
	Name       string               `json:"name"`
	CNIVersion string               `json:"cniVersion"`
	IPAM       *HostLocalIPAMConfig `json:"ipam"`
}

type HostLocalIPAMConfig struct {
	Type    string              `json:"type"`
	DataDir string              `json:"dataDir"`
	Ranges  []HostLocalRangeSet `json:"ranges"`
}

type HostLocalRangeSet []HostLocalRange

type HostLocalRange struct {
	RangeStart string `json:"rangeStart,omitempty"` // The first ip, inclusive
	RangeEnd   string `json:"rangeEnd,omitempty"`   // The last ip, inclusive
	Subnet     string `json:"subnet"`
	Gateway    string `json:"gateway,omitempty"`
}

func HostLocalNetConfFromNetConfAndPool(nc *NetConf, p *pool.IPPool) *HostLocalNetConf {
	// Note(adrianc): we use Pool name as Network Name for host-local call so that assignments are managed
	// by host-local ipam by pool name and not the network name.
	return &HostLocalNetConf{
		Name:       p.Name,
		CNIVersion: nc.CNIVersion,
		IPAM: &HostLocalIPAMConfig{
			Type:    "host-local",
			DataDir: filepath.Join(nc.IPAM.DataDir, HostLocalDataDir),
			Ranges: []HostLocalRangeSet{
				[]HostLocalRange{
					{
						RangeStart: p.StartIP,
						RangeEnd:   p.EndIP,
						Subnet:     p.Subnet,
						Gateway:    p.Gateway,
					},
				},
			},
		},
	}
}
