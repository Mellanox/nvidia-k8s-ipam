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
})
