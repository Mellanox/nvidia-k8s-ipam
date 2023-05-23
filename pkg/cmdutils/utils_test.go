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

package cmdutils_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/cmdutils"
)

var _ = Describe("cmdutils testing", func() {
	var (
		tmpDir string
	)

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
	})

	It("Run CopyFileAtomic()", func() {
		var err error
		// create source directory
		srcDir := fmt.Sprintf("%s/src", tmpDir)
		err = os.Mkdir(srcDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		// create destination directory
		destDir := fmt.Sprintf("%s/dest", tmpDir)
		err = os.Mkdir(destDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		// sample source file
		srcFilePath := fmt.Sprintf("%s/sampleInput", srcDir)
		err = os.WriteFile(srcFilePath, []byte("sampleInputABC"), 0744)
		Expect(err).NotTo(HaveOccurred())

		// old files in dest
		destFileName := "sampleInputDest"
		destFilePath := fmt.Sprintf("%s/%s", destDir, destFileName)
		err = os.WriteFile(destFilePath, []byte("inputOldXYZ"), 0611)
		Expect(err).NotTo(HaveOccurred())

		tempFileName := "temp_file"
		err = cmdutils.CopyFileAtomic(srcFilePath, destDir, tempFileName, destFileName)
		Expect(err).NotTo(HaveOccurred())

		// check file mode
		stat, err := os.Stat(destFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Mode()).To(Equal(os.FileMode(0744)))

		// check file contents
		destFileByte, err := os.ReadFile(destFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(destFileByte).To(Equal([]byte("sampleInputABC")))
	})
})
