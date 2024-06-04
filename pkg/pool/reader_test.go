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
	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

var _ = Describe("pool tests", func() {
	Context("NewConfigReader()", func() {
		It("Creates a Manager successfully if node has ip-pool annotation", func() {
			n := v1.Node{}
			emptyAnnot := map[string]string{
				pool.IPBlocksAnnotation: "{}",
			}
			n.SetAnnotations(emptyAnnot)
			r, err := pool.NewConfigReader(&n)
			Expect(err).ToNot(HaveOccurred())
			Expect(r.GetPools()).To(HaveLen(0))

			annot := map[string]string{
				pool.IPBlocksAnnotation: `{"my-pool":
				{"subnet": "192.168.0.0/16", "startIP": "192.168.0.2",
				"endIP": "192.168.0.254", "gateway": "192.168.0.1"}}`,
			}
			n.SetAnnotations(annot)
			r, err = pool.NewConfigReader(&n)
			Expect(err).ToNot(HaveOccurred())
			Expect(r.GetPools()).To(HaveLen(1))
		})

		It("Fails to create Manager if node is missing ip-pool annotation", func() {
			n := v1.Node{}
			_, err := pool.NewConfigReader(&n)
			Expect(err).To(HaveOccurred())
		})

		It("Fails to create Manager if node has empty/invalid ip-pool annotation", func() {
			n := v1.Node{}
			emptyAnnot := map[string]string{
				pool.IPBlocksAnnotation: "",
			}
			n.SetAnnotations(emptyAnnot)
			_, err := pool.NewConfigReader(&n)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("GetPoolByKey()", func() {
		var r pool.ConfigReader

		BeforeEach(func() {
			var err error
			n := v1.Node{}
			annot := map[string]string{
				pool.IPBlocksAnnotation: `{"my-pool":
				{"subnet": "192.168.0.0/16", "startIP": "192.168.0.2",
				"endIP": "192.168.0.254", "gateway": "192.168.0.1"}}`,
			}
			n.SetAnnotations(annot)
			r, err = pool.NewConfigReader(&n)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns nil if pool does not exist", func() {
			p := r.GetPoolByKey("non-existent-key")
			Expect(p).To(BeNil())
		})

		It("returns pool if exists", func() {
			p := r.GetPoolByKey("my-pool")
			Expect(p).ToNot(BeNil())
			Expect(p.Subnet).To(Equal("192.168.0.0/16"))
		})
	})
})
