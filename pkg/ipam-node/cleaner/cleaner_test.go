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

package cleaner_test

import (
	"net"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cleanerPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/cleaner"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

const (
	testNamespace = "default"
	testPodName1  = "test-pod1"
	testPodName2  = "test-pod2"
	testPool1     = "pool1"
	testPool2     = "pool2"
	testIFName    = "net0"
)

func createPod(name, namespace string) string {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "something", Image: "something"}}},
	}
	ExpectWithOffset(1, k8sClient.Create(ctx, p)).NotTo(HaveOccurred())
	return string(p.UID)
}

var _ = Describe("Cleaner", func() {
	It("Cleanup test", func() {
		done := make(chan interface{})
		go func() {
			storePath := filepath.Join(GinkgoT().TempDir(), "test_store")
			storeMgr := storePkg.New(storePath)
			cleaner := cleanerPkg.New(k8sClient, storeMgr, time.Millisecond*100, 3)

			pod1UID := createPod(testPodName1, testNamespace)
			_ = createPod(testPodName2, testNamespace)

			store, err := storeMgr.Open(ctx)
			Expect(err).NotTo(HaveOccurred())

			// should keep these reservations
			Expect(store.Reserve(testPool1, "id1", testIFName, types.ReservationMetadata{
				CreateTime:   time.Now().Format(time.RFC3339Nano),
				PodUUID:      pod1UID,
				PodName:      testPodName1,
				PodNamespace: testNamespace,
			}, net.ParseIP("192.168.1.100"))).NotTo(HaveOccurred())

			Expect(store.Reserve(testPool1, "id2", testIFName, types.ReservationMetadata{},
				net.ParseIP("192.168.1.101"))).NotTo(HaveOccurred())

			// should remove these reservations
			Expect(store.Reserve(testPool1, "id3", testIFName, types.ReservationMetadata{
				CreateTime:   time.Now().Format(time.RFC3339Nano),
				PodName:      "unknown",
				PodNamespace: testNamespace,
			}, net.ParseIP("192.168.1.102"))).NotTo(HaveOccurred())
			Expect(store.Reserve(testPool2, "id4", testIFName, types.ReservationMetadata{
				CreateTime:   time.Now().Format(time.RFC3339Nano),
				PodName:      "unknown2",
				PodNamespace: testNamespace,
			}, net.ParseIP("192.168.2.100"))).NotTo(HaveOccurred())
			Expect(store.Reserve(testPool2, "id5", testIFName, types.ReservationMetadata{
				CreateTime:   time.Now().Format(time.RFC3339Nano),
				PodUUID:      "something", // differ from the reservation
				PodName:      testPodName2,
				PodNamespace: testNamespace,
			}, net.ParseIP("192.168.2.101"))).NotTo(HaveOccurred())

			Expect(store.Commit()).NotTo(HaveOccurred())

			go func() {
				cleaner.Start(ctx)
			}()
			Eventually(func(g Gomega) {
				store, err := storeMgr.Open(ctx)
				g.Expect(err).NotTo(HaveOccurred())
				defer store.Cancel()
				g.Expect(store.GetReservationByID(testPool1, "id1", testIFName)).NotTo(BeNil())
				g.Expect(store.GetReservationByID(testPool1, "id2", testIFName)).NotTo(BeNil())
				g.Expect(store.GetReservationByID(testPool1, "id3", testIFName)).To(BeNil())
				g.Expect(store.GetReservationByID(testPool2, "id4", testIFName)).To(BeNil())
				g.Expect(store.GetReservationByID(testPool2, "id5", testIFName)).To(BeNil())
			}, 10).Should(Succeed())

			close(done)
		}()
		Eventually(done, time.Minute).Should(BeClosed())
	})
})
