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

package k8sclient_test

import (
	"os"
	"path"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/k8sclient"
)

var _ = Describe("k8sclient tests", func() {
	var (
		tmpDir                string
		validKubeconfigPath   string
		invalidKubeconfigPath string
	)

	BeforeEach(func() {
		var err error
		tmpDir := GinkgoT().TempDir()

		validKubeconfigPath = filepath.Join(tmpDir, "kube.conf")
		invalidKubeconfigPath = filepath.Join(tmpDir, "kube-invalid.conf")

		data, err := os.ReadFile(path.Join("..", "..", "..", "testdata", "test.kubeconfig"))
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(validKubeconfigPath, data, 0o644)
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(invalidKubeconfigPath, []byte("This is an invalid kubeconfig content"), 0o644)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("FromKubeconfig", func() {
		It("creates k8s deffered client if kubeconfig valid", func() {
			_, err := k8sclient.FromKubeconfig(validKubeconfigPath)
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails if kubeconfig path not found", func() {
			_, err := k8sclient.FromKubeconfig(filepath.Join(tmpDir, "does-not-exist.conf"))
			Expect(err).To(HaveOccurred())
		})

		It("fails if kubeconfig file contains garbage", func() {
			_, err := k8sclient.FromKubeconfig(invalidKubeconfigPath)
			Expect(err).To(HaveOccurred())
		})
	})
})
