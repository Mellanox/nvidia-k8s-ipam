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

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
)

var _ = Describe("Validate", func() {
	It("Valid", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				Exclusions: []v1alpha1.ExcludeRange{
					{StartIP: "192.168.0.100", EndIP: "192.168.0.110"},
				},
				Gateway: "192.168.0.1",
				NodeSelector: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "foo.bar",
							Operator: corev1.NodeSelectorOpExists,
						}},
					}},
				},
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("Valid - ipv6", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "2001:db8:3333:4444::0/64",
				PerNodeBlockSize: 1000,
				Exclusions: []v1alpha1.ExcludeRange{
					{StartIP: "2001:db8:3333:4444::3", EndIP: "2001:db8:3333:4444::4"},
				},
				Gateway: "2001:db8:3333:4444::1",
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("Valid - no NodeSelector", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				Gateway:          "192.168.0.1",
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("Valid - no Gateway", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("Empty object", func() {
		ipPool := v1alpha1.IPPool{}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(And(
				ContainSubstring("metadata.name"),
				ContainSubstring("spec.subnet"),
				ContainSubstring("spec.perNodeBlockSize"),
			))
	})
	It("Invalid - perNodeBlockSize is too large", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/24",
				PerNodeBlockSize: 300,
				Gateway:          "192.168.0.1",
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.perNodeBlockSize"),
			)
	})
	It("Invalid - exclusions not part of the subnet", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet: "192.168.0.0/24",
				Exclusions: []v1alpha1.ExcludeRange{
					{StartIP: "10.10.10.10", EndIP: "10.10.10.20"},
				},
				PerNodeBlockSize: 10,
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.exclusions"),
			)
	})
	It("Invalid - gateway outside of the subnet", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				Gateway:          "10.0.0.1",
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.gateway"),
			)
	})
	It("Invalid - invalid NodeSelector", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				Gateway:          "192.168.0.1",
				NodeSelector: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "foo.bar",
							Operator: "unknown",
						}},
					}},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.nodeSelector"),
			)
	})
	It("Invalid - no Gateway, defaultGateway true", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				DefaultGateway:   true,
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.defaultGateway"),
			)
	})
	It("Valid - routes", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				Gateway:          "192.168.0.1",
				Routes: []v1alpha1.Route{
					{
						Dst: "5.5.0.0/16",
					},
					{
						Dst: "10.7.1.0/24",
					},
				},
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("Invalid - routes without Gateway", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				Routes: []v1alpha1.Route{
					{
						Dst: "5.5.0.0/16",
					},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.routes"),
			)
	})
	It("Invalid - routes not CIDR", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				Routes: []v1alpha1.Route{
					{
						Dst: "5.5.0.0",
					},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.routes"),
			)
	})
	It("Invalid - routes not same address family", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				Routes: []v1alpha1.Route{
					{
						Dst: "2001:db8:3333:4444::0/64",
					},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.routes"),
			)
	})
	It("Invalid - default routes with defaultGateway true - IPv6", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "2001:db8:3333:4444::0/64",
				PerNodeBlockSize: 128,
				DefaultGateway:   true,
				Routes: []v1alpha1.Route{
					{
						Dst: "::/0",
					},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.routes"),
			)
	})
	It("Invalid - default routes with defaultGateway true - IPv4", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				DefaultGateway:   true,
				Routes: []v1alpha1.Route{
					{
						Dst: "0.0.0.0/0",
					},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.routes"),
			)
	})
	It("Valid - PerNodeExclusions", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 0, EndIndex: 10},
					{StartIndex: 100, EndIndex: 127},
				},
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("Valid - PerNodeExclusions single index", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 5, EndIndex: 5},
				},
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("Valid - PerNodeExclusions max index", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 120, EndIndex: 127}, // 127 is max for block size 128
				},
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("Invalid - PerNodeExclusions negative startIndex", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: -5, EndIndex: 20},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.perNodeExclusions[0].startIndex"),
			)
	})
	It("Invalid - PerNodeExclusions negative endIndex", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 5, EndIndex: -1},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.perNodeExclusions[0].endIndex"),
			)
	})
	It("Invalid - PerNodeExclusions startIndex greater than endIndex", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 25, EndIndex: 24},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.perNodeExclusions[0]"),
			)
	})
	It("Invalid - PerNodeExclusions startIndex outside block range", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 128, EndIndex: 130}, // max is 127 for block size 128
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(And(
				ContainSubstring("spec.perNodeExclusions[0].startIndex"),
				ContainSubstring("outside"),
			))
	})
	It("Invalid - PerNodeExclusions endIndex outside block range", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "192.168.0.0/16",
				PerNodeBlockSize: 128,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 120, EndIndex: 200},
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(And(
				ContainSubstring("spec.perNodeExclusions[0].endIndex"),
				ContainSubstring("outside"),
			))
	})
	It("Valid - PerNodeExclusions IPv6", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "2001:db8:3333:4444::0/64",
				PerNodeBlockSize: 256,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 0, EndIndex: 10},
					{StartIndex: 250, EndIndex: 255},
				},
			},
		}
		Expect(ipPool.Validate()).To(BeEmpty())
	})
	It("PerNodeExclusions - IPv6 large subnet", func() {
		cidrPool := v1alpha1.CIDRPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.CIDRPoolSpec{
				CIDR:                 "fdf8:6aef:d1fe::/48",
				PerNodeNetworkPrefix: 64,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 0, EndIndex: 10},
					{StartIndex: 250, EndIndex: 255},
				},
			},
		}
		validatePoolAndCheckErr(&cidrPool, true)
	})
	It("Invalid - PerNodeExclusions IPv6 outside range", func() {
		ipPool := v1alpha1.IPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.IPPoolSpec{
				Subnet:           "2001:db8:3333:4444::0/64",
				PerNodeBlockSize: 256,
				PerNodeExclusions: []v1alpha1.ExcludeIndexRange{
					{StartIndex: 0, EndIndex: 300}, // max is 255 for block size 256
				},
			},
		}
		Expect(ipPool.Validate().ToAggregate().Error()).
			To(
				ContainSubstring("spec.perNodeExclusions[0].endIndex"),
			)
	})
})
