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

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gomegaTypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
)

// validatePoolAndCheckErr runs validation for the allocation and checks that the result of
// the validation matches the expected result.
// if isValid is false, optional errMatcher can be provided to validate value of the error
func validatePoolAndCheckErr(pool *v1alpha1.CIDRPool, isValid bool, errMatcher ...gomegaTypes.GomegaMatcher) {
	errList := pool.Validate()
	if isValid {
		ExpectWithOffset(1, errList).To(BeEmpty())
		return
	}
	ExpectWithOffset(1, errList).NotTo(BeEmpty())
	if len(errMatcher) > 0 {
		ExpectWithOffset(1, errList.ToAggregate().Error()).To(And(errMatcher...))
	}
}

var _ = Describe("CIDRPool", func() {
	It("Valid IPv4 pool", func() {
		cidrPool := v1alpha1.CIDRPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.CIDRPoolSpec{
				CIDR:                 "192.168.0.0/16",
				PerNodeNetworkPrefix: 24,
				GatewayIndex:         ptr.To[int32](100),
				Exclusions: []v1alpha1.ExcludeRange{
					{StartIP: "192.168.0.10", EndIP: "192.168.0.20"},
					{StartIP: "192.168.0.25", EndIP: "192.168.0.25"},
				},
				StaticAllocations: []v1alpha1.CIDRPoolStaticAllocation{
					{NodeName: "node1", Prefix: "192.168.5.0/24", Gateway: "192.168.5.10"},
					{NodeName: "node2", Prefix: "192.168.6.0/24"},
					{Prefix: "192.168.7.0/24"},
				},
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
		validatePoolAndCheckErr(&cidrPool, true)
	})
	It("Valid IPv6 pool", func() {
		cidrPool := v1alpha1.CIDRPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.CIDRPoolSpec{
				CIDR:                 "fdf8:6aef:d1fe::/48",
				PerNodeNetworkPrefix: 120,
				GatewayIndex:         ptr.To[int32](5),
				Exclusions: []v1alpha1.ExcludeRange{
					{StartIP: "fdf8:6aef:d1fe::5", EndIP: "fdf8:6aef:d1fe::5"},
				},
				StaticAllocations: []v1alpha1.CIDRPoolStaticAllocation{
					{NodeName: "node1", Prefix: "fdf8:6aef:d1fe::/120", Gateway: "fdf8:6aef:d1fe::15"},
				},
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
		validatePoolAndCheckErr(&cidrPool, true)
	})
	DescribeTable("CIDR",
		func(cidr string, prefix int32, isValid bool) {
			cidrPool := v1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.CIDRPoolSpec{
					CIDR:                 cidr,
					PerNodeNetworkPrefix: prefix,
				}}
			validatePoolAndCheckErr(&cidrPool, isValid, ContainSubstring("spec.cidr"))
		},
		Entry("empty", "", int32(30), false),
		Entry("invalid value", "aaaa", int32(30), false),
		Entry("/32", "192.168.1.1/32", int32(32), false),
		Entry("/128", "2001:db8:3333:4444::0/128", int32(128), false),
		Entry("valid ipv4", "192.168.1.0/24", int32(30), true),
		Entry("valid ipv6", "2001:db8:3333:4444::0/64", int32(120), true),
	)
	DescribeTable("PerNodeNetworkPrefix",
		func(cidr string, prefix int32, isValid bool) {
			cidrPool := v1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.CIDRPoolSpec{
					CIDR:                 cidr,
					PerNodeNetworkPrefix: prefix,
				}}
			validatePoolAndCheckErr(&cidrPool, isValid, ContainSubstring("spec.perNodeNetworkPrefix"))
		},
		Entry("not set", "192.168.0.0/16", int32(0), false),
		Entry("negative", "192.168.0.0/16", int32(-10), false),
		Entry("larger than CIDR", "192.168.0.0/16", int32(8), false),
		Entry("smaller than 31 for IPv4 pool", "192.168.0.0/16", int32(32), false),
		Entry("smaller than 127 for IPv6 pool", "2001:db8:3333:4444::0/64", int32(128), false),
		Entry("match CIDR prefix size - ipv4", "192.168.0.0/16", int32(16), true),
		Entry("match CIDR prefix size - ipv6", "2001:db8:3333:4444::0/64", int32(64), true),
	)
	DescribeTable("NodeSelector",
		func(nodeSelector *corev1.NodeSelector, isValid bool, errMatcher ...gomegaTypes.GomegaMatcher) {
			cidrPool := v1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.CIDRPoolSpec{
					CIDR:                 "192.168.0.0/16",
					PerNodeNetworkPrefix: 24,
					NodeSelector:         nodeSelector,
				},
			}
			validatePoolAndCheckErr(&cidrPool, isValid, errMatcher...)
		},
		Entry("not set", nil, true),
		Entry("valid", &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{
				MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "foo.bar", Operator: "Exists"}},
			}},
		}, true),
		Entry("unknown operation", &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{
				MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "foo.bar", Operator: "unknown"}},
			}},
		}, false, ContainSubstring("spec.nodeSelectorTerms[0].matchExpressions[0].operator")),
	)
	DescribeTable("GatewayIndex",
		func(gatewayIndex int32, isValid bool, errMatcher ...gomegaTypes.GomegaMatcher) {
			cidrPool := v1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.CIDRPoolSpec{
					CIDR:                 "192.168.0.0/16",
					PerNodeNetworkPrefix: 24,
					GatewayIndex:         &gatewayIndex,
				},
			}
			validatePoolAndCheckErr(&cidrPool, isValid, ContainSubstring("spec.gatewayIndex"))
		},
		Entry("negative", int32(-10), false),
		Entry("too large", int32(255), false),
		Entry("index 1 is valid for point to point", int32(1), true),
		Entry("index 2 is valid for point to point", int32(2), true),
	)
	DescribeTable("Exclusions",
		func(exclusions []v1alpha1.ExcludeRange, isValid bool, errMatcher ...gomegaTypes.GomegaMatcher) {
			cidrPool := v1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.CIDRPoolSpec{
					CIDR:                 "192.168.0.0/16",
					PerNodeNetworkPrefix: 24,
					Exclusions:           exclusions,
				},
			}
			validatePoolAndCheckErr(&cidrPool, isValid, errMatcher...)
		},
		Entry("valid", []v1alpha1.ExcludeRange{
			{StartIP: "192.168.0.10", EndIP: "192.168.0.20"},
		}, true),
		Entry("startIP not set", []v1alpha1.ExcludeRange{
			{EndIP: "192.168.0.20"},
		}, false, ContainSubstring("spec.exclusions[0].startIP")),
		Entry("endIP not set", []v1alpha1.ExcludeRange{
			{StartIP: "192.168.0.10"},
		}, false, ContainSubstring("spec.exclusions[0].endIP")),
		Entry("not IPs", []v1alpha1.ExcludeRange{
			{StartIP: "aaa", EndIP: "bb"},
			{StartIP: "192.168.0.25", EndIP: "ccc"},
		}, false,
			ContainSubstring("spec.exclusions[0].startIP"),
			ContainSubstring("spec.exclusions[0].endIP"),
			ContainSubstring("spec.exclusions[1].endIP")),
		Entry("startIP is greater then endIP", []v1alpha1.ExcludeRange{
			{StartIP: "192.168.0.25", EndIP: "192.168.0.24"},
		}, false, ContainSubstring("spec.exclusions[0]")),
		Entry("doesn't belong to cidr", []v1alpha1.ExcludeRange{
			{StartIP: "10.10.33.25", EndIP: "10.10.33.33"},
		}, false, ContainSubstring("spec.exclusions[0]")),
	)
	DescribeTable("StaticAllocations",
		func(staticAllocations []v1alpha1.CIDRPoolStaticAllocation, isValid bool, errMatcher ...gomegaTypes.GomegaMatcher) {
			cidrPool := v1alpha1.CIDRPool{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: v1alpha1.CIDRPoolSpec{
					CIDR:                 "192.168.0.0/16",
					PerNodeNetworkPrefix: 24,
					StaticAllocations:    staticAllocations,
				},
			}
			validatePoolAndCheckErr(&cidrPool, isValid, errMatcher...)
		},
		Entry("valid", []v1alpha1.CIDRPoolStaticAllocation{
			{Prefix: "192.168.0.0/24", Gateway: "192.168.0.1", NodeName: "node1"}}, true),
		Entry("valid - no gateway", []v1alpha1.CIDRPoolStaticAllocation{
			{Prefix: "192.168.0.0/24", NodeName: "node1"}}, true),
		Entry("valid - no node name", []v1alpha1.CIDRPoolStaticAllocation{
			{Prefix: "192.168.0.0/24"}}, true),
		Entry("not a prefix", []v1alpha1.CIDRPoolStaticAllocation{{Prefix: "192.168.0.0"}}, false,
			ContainSubstring("spec.staticAllocations[0].prefix")),
		Entry("wrong prefix size", []v1alpha1.CIDRPoolStaticAllocation{{Prefix: "192.168.0.0/31"}}, false,
			ContainSubstring("spec.staticAllocations[0].prefix")),
		Entry("prefix is not part of the cidr", []v1alpha1.CIDRPoolStaticAllocation{{Prefix: "10.10.10.0/24"}}, false,
			ContainSubstring("spec.staticAllocations[0].prefix")),
		Entry("gateway is not an IP", []v1alpha1.CIDRPoolStaticAllocation{
			{Prefix: "192.168.0.0/24", Gateway: "foo"}}, false,
			ContainSubstring("spec.staticAllocations[0].gateway")),
		Entry("gateway is not in the allocated prefix", []v1alpha1.CIDRPoolStaticAllocation{
			{Prefix: "192.168.0.0/24", Gateway: "192.168.1.1"}}, false,
			ContainSubstring("spec.staticAllocations[0].gateway")),
		Entry("duplicate node names", []v1alpha1.CIDRPoolStaticAllocation{
			{NodeName: "nodeA", Prefix: "192.168.0.0/24"},
			{NodeName: "nodeA", Prefix: "192.168.1.0/24"}}, false,
			ContainSubstring("spec.staticAllocations")),
		Entry("duplicate prefixes", []v1alpha1.CIDRPoolStaticAllocation{
			{NodeName: "nodeA", Prefix: "192.168.1.0/24"},
			{NodeName: "nodeB", Prefix: "192.168.1.0/24"}}, false,
			ContainSubstring("spec.staticAllocations")),
	)
})

// validateAllocationAndCheckErr runs validation for the allocation and checks that the result of
// the validation matches the expected result.
// if isValid is false, optional errMatcher can be provided to validate value of the error
func validateAllocationAndCheckErr(allocation *v1alpha1.CIDRPoolAllocation, pool *v1alpha1.CIDRPool,
	isValid bool, errMatcher ...gomegaTypes.GomegaMatcher) {
	errList := allocation.Validate(pool)
	if isValid {
		ExpectWithOffset(1, errList).To(BeEmpty())
		return
	}
	ExpectWithOffset(1, errList).NotTo(BeEmpty())
	if len(errMatcher) > 0 {
		ExpectWithOffset(1, errList.ToAggregate().Error()).To(And(errMatcher...))
	}
}

var _ = Describe("CIDRPoolAllocation", func() {
	var (
		cidrPool *v1alpha1.CIDRPool
	)
	BeforeEach(func() {
		cidrPool = &v1alpha1.CIDRPool{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: v1alpha1.CIDRPoolSpec{
				CIDR:                 "192.168.0.0/16",
				PerNodeNetworkPrefix: 24,
				GatewayIndex:         ptr.To[int32](100),
				StaticAllocations: []v1alpha1.CIDRPoolStaticAllocation{
					{NodeName: "node1", Prefix: "192.168.1.0/24", Gateway: "192.168.1.10"},
					{NodeName: "node2", Prefix: "192.168.2.0/24"},
				},
			},
			Status: v1alpha1.CIDRPoolStatus{
				Allocations: []v1alpha1.CIDRPoolAllocation{{
					NodeName: "node3",
					Prefix:   "192.168.3.0/24",
					Gateway:  "192.168.3.100",
				}},
			},
		}
	})
	Context("Valid", func() {
		It("new", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node4",
				Prefix:   "192.168.4.0/24",
				Gateway:  "192.168.4.100",
			}, cidrPool, true)
		})
		It("exist in status", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node3",
				Prefix:   "192.168.3.0/24",
				Gateway:  "192.168.3.100",
			}, cidrPool, true)
		})
		It("Valid - no gateway", func() {
			cidrPool.Spec.GatewayIndex = nil
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node3",
				Prefix:   "192.168.3.0/24",
			}, cidrPool, true)
		})
	})
	Context("Invalid", func() {
		It("node not specified", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				Prefix:  "192.168.4.0/24",
				Gateway: "192.168.4.100",
			}, cidrPool, false, ContainSubstring("nodeName"))
		})
		It("empty", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{},
				cidrPool, false, ContainSubstring("nodeName"), ContainSubstring("prefix"))
		})
		It("conflict with static allocation - range mismatch", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node1",
				Prefix:   "192.168.33.0/24",
				Gateway:  "192.168.33.100",
			}, cidrPool, false,
				ContainSubstring("gateway"),
				ContainSubstring("prefix"),
				ContainSubstring("static allocation"),
			)
		})
		It("conflict with static allocation - prefix allocated for different node", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node1",
				Prefix:   "192.168.2.0/24",
				Gateway:  "192.168.2.100",
			}, cidrPool, false,
				ContainSubstring("prefix"),
				ContainSubstring("static allocation"),
			)
		})
		It("conflict with static allocation - gateway mismatch", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node2",
				Prefix:   "192.168.2.0/24",
			}, cidrPool, false,
				ContainSubstring("gateway"),
			)
		})
		It("conflict with static allocation - dynamic gw instead of static", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node1",
				Prefix:   "192.168.1.0/24",
				Gateway:  "192.168.1.100",
			}, cidrPool, false,
				ContainSubstring("gateway"),
			)
		})
		It("conflicting allocation", func() {
			cidrPool.Status.Allocations = append(cidrPool.Status.Allocations, v1alpha1.CIDRPoolAllocation{
				NodeName: "node4",
				Prefix:   "192.168.3.0/24",
			})
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node3",
				Prefix:   "192.168.3.0/24",
				Gateway:  "192.168.3.100",
			}, cidrPool, false,
				ContainSubstring("conflicting allocation"),
			)
		})
		It("gateway mismatch", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node5",
				Prefix:   "192.168.5.0/24",
			}, cidrPool, false,
				ContainSubstring("gateway"),
			)
		})
		It("prefix has host bits set", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node5",
				Prefix:   "192.168.5.1/24",
				Gateway:  "192.168.5.100",
			}, cidrPool, false,
				ContainSubstring("prefix"),
			)
		})
		It("wrong prefix size", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node5",
				Prefix:   "192.168.5.0/26",
				Gateway:  "192.168.5.100",
			}, cidrPool, false,
				ContainSubstring("prefix"),
			)
		})
		It("wrong prefix", func() {
			validateAllocationAndCheckErr(&v1alpha1.CIDRPoolAllocation{
				NodeName: "node5",
				Prefix:   "10.10.5.0/24",
				Gateway:  "10.10.5.100",
			}, cidrPool, false,
				ContainSubstring("prefix"),
			)
		})
	})

})
