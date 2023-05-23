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

package pool_test

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

var _ = Describe("annotations tests", func() {
	Context("SetIPBlockAnnotation", func() {
		testPools := make(map[string]*pool.IPPool)
		testPools["my-pool-1"] = &pool.IPPool{
			Name:    "my-pool-1",
			Subnet:  "192.168.0.0/16",
			StartIP: "192.168.0.2",
			EndIP:   "192.168.0.254",
			Gateway: "192.168.0.1",
		}
		testPools["my-pool-2"] = &pool.IPPool{
			Name:    "my-pool-2",
			Subnet:  "10.100.0.0/16",
			StartIP: "10.100.0.2",
			EndIP:   "10.100.0.254",
			Gateway: "10.100.0.1",
		}

		It("sets annotation successfully", func() {
			n := v1.Node{}
			pool.SetIPBlockAnnotation(&n, testPools)
			Expect(n.GetAnnotations()).ToNot(BeNil())

			data, err := json.Marshal(testPools)
			Expect(err).ToNot(HaveOccurred())
			Expect(n.GetAnnotations()[pool.IPBlocksAnnotation]).To(Equal(string(data)))
		})

		It("overwrites annotation successfully", func() {
			n := v1.Node{}
			annot := map[string]string{
				pool.IPBlocksAnnotation: `{"my-pool": {"some": "content"}}`,
			}
			n.SetAnnotations(annot)

			pool.SetIPBlockAnnotation(&n, testPools)
			Expect(n.GetAnnotations()).ToNot(BeNil())

			data, err := json.Marshal(testPools)
			Expect(err).ToNot(HaveOccurred())
			Expect(n.GetAnnotations()[pool.IPBlocksAnnotation]).To(Equal(string(data)))
		})
	})

	Context("IPBlockAnnotationExists", func() {
		It("returns true if annotation exists", func() {
			n := v1.Node{}
			ipBlockAnnot := make(map[string]string)
			ipBlockAnnot[pool.IPBlocksAnnotation] = "foobar"
			n.SetAnnotations(ipBlockAnnot)

			Expect(pool.IPBlockAnnotationExists(&n)).To(BeTrue())
		})

		It("returns false if annotation does not exists", func() {
			Expect(pool.IPBlockAnnotationExists(&v1.Node{})).To(BeFalse())
		})
	})

	Context("RemoveIPBlockAnnotation", func() {
		It("Succeeds if annotation does not exist", func() {
			n := v1.Node{}
			annot := map[string]string{
				"foo": "bar",
			}
			n.SetAnnotations(annot)

			pool.RemoveIPBlockAnnotation(&n)
			Expect(n.GetAnnotations()).To(HaveKey("foo"))
		})

		It("removes annotation if exists", func() {
			n := v1.Node{}
			annot := map[string]string{
				"foo":                   "bar",
				pool.IPBlocksAnnotation: "baz",
			}
			n.SetAnnotations(annot)

			pool.RemoveIPBlockAnnotation(&n)
			Expect(n.GetAnnotations()).To(HaveKey("foo"))
			Expect(n.GetAnnotations()).ToNot(HaveKey(pool.IPBlocksAnnotation))
		})
	})
})
