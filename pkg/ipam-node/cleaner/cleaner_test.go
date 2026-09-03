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
	"context"
	"net"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cleanerPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/cleaner"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store/storetest"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
	poolPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
	poolMockPkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/pool/mocks"
)

const (
	testNamespace = "default"
	testPodName1  = "test-pod1"
	testPodName2  = "test-pod2"
	testPool1     = "pool1"
	testPool2     = "pool2"
	testPool3     = "pool3"
	testIFName    = "net0"
	// used by the release cooldown tests
	testReleaseCooldownIP = "192.168.10.100"
	testPendingCooldownIP = "192.168.11.100"
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
			defer GinkgoRecover()
			defer close(done)
			testCtx, testCancel := context.WithCancel(ctx)
			defer testCancel()
			storePath := filepath.Join(GinkgoT().TempDir(), "test_store")
			store := storePkg.New(storePath)

			poolManager := poolMockPkg.NewManager(GinkgoT())
			// pool2 has no config in the k8s API
			poolManager.On("GetPoolByKey", testPool2).Return(nil)
			// pool3 has config in the k8s API
			poolManager.On("GetPoolByKey", testPool3).Return(&poolPkg.Pool{})

			session, err := store.Open(testCtx)
			Expect(err).NotTo(HaveOccurred())
			// this will create empty pool config
			session.SetLastReservedIP(testPool3, net.ParseIP("192.168.33.100"))

			cleaner := cleanerPkg.New(fakeClient, k8sClient, store, poolManager, time.Millisecond*100, 3, 0)

			pod1UID := createPod(testPodName1, testNamespace)
			_ = createPod(testPodName2, testNamespace)

			// should keep these reservations
			Expect(session.Reserve(testPool1, "id1", testIFName, types.ReservationMetadata{
				CreateTime:   time.Now().Format(time.RFC3339Nano),
				PodUUID:      pod1UID,
				PodName:      testPodName1,
				PodNamespace: testNamespace,
			}, net.ParseIP("192.168.1.100"))).NotTo(HaveOccurred())

			Expect(session.Reserve(testPool1, "id2", testIFName, types.ReservationMetadata{},
				net.ParseIP("192.168.1.101"))).NotTo(HaveOccurred())

			// should remove these reservations
			Expect(session.Reserve(testPool1, "id3", testIFName, types.ReservationMetadata{
				CreateTime:   time.Now().Format(time.RFC3339Nano),
				PodName:      "unknown",
				PodNamespace: testNamespace,
			}, net.ParseIP("192.168.1.102"))).NotTo(HaveOccurred())
			Expect(session.Reserve(testPool2, "id4", testIFName, types.ReservationMetadata{
				CreateTime:   time.Now().Format(time.RFC3339Nano),
				PodName:      "unknown2",
				PodNamespace: testNamespace,
			}, net.ParseIP("192.168.2.100"))).NotTo(HaveOccurred())
			Expect(session.Reserve(testPool2, "id5", testIFName, types.ReservationMetadata{
				CreateTime:   time.Now().Format(time.RFC3339Nano),
				PodUUID:      "something", // differ from the reservation
				PodName:      testPodName2,
				PodNamespace: testNamespace,
			}, net.ParseIP("192.168.2.101"))).NotTo(HaveOccurred())

			Expect(session.Commit()).NotTo(HaveOccurred())

			go func() {
				cleaner.Start(testCtx)
			}()
			Eventually(func(g Gomega) {
				store, err := store.Open(testCtx)
				g.Expect(err).NotTo(HaveOccurred())
				defer store.Cancel()
				g.Expect(store.GetReservationByID(testPool1, "id1", testIFName)).NotTo(BeNil())
				g.Expect(store.GetReservationByID(testPool1, "id2", testIFName)).NotTo(BeNil())
				g.Expect(store.GetReservationByID(testPool1, "id3", testIFName)).To(BeNil())
				g.Expect(store.GetReservationByID(testPool2, "id4", testIFName)).To(BeNil())
				g.Expect(store.GetReservationByID(testPool2, "id5", testIFName)).To(BeNil())
				g.Expect(store.ListPools()).To(And(
					ContainElements(testPool1, testPool3),
					Not(ContainElement(testPool2))))
			}, 10).Should(Succeed())

		}()
		Eventually(done, time.Minute).Should(BeClosed())
	})

	It("Release cooldown test", func() {
		done := make(chan interface{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			testCtx, testCancel := context.WithCancel(ctx)
			defer testCancel()
			storePath := filepath.Join(GinkgoT().TempDir(), "test_store_cooldown")
			store := storePkg.New(storePath)

			poolManager := poolMockPkg.NewManager(GinkgoT())

			session, err := store.Open(testCtx)
			Expect(err).NotTo(HaveOccurred())

			checkInterval := time.Millisecond * 50
			// short enough to keep the test fast; the point is only that it eventually
			// elapses and the cleaner then removes the reservation
			releaseCooldown := time.Millisecond * 100

			Expect(session.Reserve(testPool1, "id1", testIFName, types.ReservationMetadata{},
				net.ParseIP(testReleaseCooldownIP))).NotTo(HaveOccurred())
			session.ReleaseReservationByID(testPool1, "id1", testIFName, releaseCooldown)

			Expect(session.Commit()).NotTo(HaveOccurred())

			cleaner := cleanerPkg.New(fakeClient, k8sClient, store, poolManager, checkInterval, 3, releaseCooldown)

			go func() {
				cleaner.Start(testCtx)
			}()

			// the cleaner ticks every checkInterval; once releaseCooldown has elapsed
			// (in real time) a subsequent tick removes the reservation
			Eventually(func(g Gomega) {
				s, err := store.Open(testCtx)
				g.Expect(err).NotTo(HaveOccurred())
				defer s.Cancel()
				g.Expect(storetest.FindReservation(s, testPool1, "id1", testIFName)).To(BeNil())
			}, 10).Should(Succeed())

			// the IP should be reusable now that the released reservation was removed
			s, err := store.Open(testCtx)
			Expect(err).NotTo(HaveOccurred())
			defer s.Cancel()
			Expect(s.Reserve(testPool1, "id2", testIFName, types.ReservationMetadata{},
				net.ParseIP(testReleaseCooldownIP))).NotTo(HaveOccurred())
		}()
		Eventually(done, time.Minute).Should(BeClosed())
	})

	It("Reservation stays blocked until the release cooldown expires", func() {
		done := make(chan interface{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			testCtx, testCancel := context.WithCancel(ctx)
			defer testCancel()
			storePath := filepath.Join(GinkgoT().TempDir(), "test_store_cooldown_pending")
			store := storePkg.New(storePath)

			poolManager := poolMockPkg.NewManager(GinkgoT())

			session, err := store.Open(testCtx)
			Expect(err).NotTo(HaveOccurred())

			checkInterval := time.Millisecond * 50
			// Use a long enough cooldown to ensure the cleaner ticks several times before it expires
			// Besides, a long cooldown ensures others cannot reuse the IP until the cooldown expires, which is what we want to test.
			releaseCooldown := time.Minute

			Expect(session.Reserve(testPool1, "id1", testIFName, types.ReservationMetadata{},
				net.ParseIP(testPendingCooldownIP))).NotTo(HaveOccurred())
			session.ReleaseReservationByID(testPool1, "id1", testIFName, releaseCooldown)
			Expect(session.Commit()).NotTo(HaveOccurred())

			cleaner := cleanerPkg.New(fakeClient, k8sClient, store, poolManager, checkInterval, 3, releaseCooldown)

			go func() {
				cleaner.Start(testCtx)
			}()

			// with a one-minute cooldown, the reservation must still be present (and
			// its IP still blocked) across several cleaner ticks
			Consistently(func(g Gomega) {
				s, err := store.Open(testCtx)
				g.Expect(err).NotTo(HaveOccurred())
				defer s.Cancel()
				g.Expect(storetest.FindReservation(s, testPool1, "id1", testIFName)).NotTo(BeNil())
				// IP should still be blocked, so a new reservation with the same IP should fail
				g.Expect(
					s.Reserve(testPool1, "id2", testIFName, types.ReservationMetadata{},
						net.ParseIP(testPendingCooldownIP))).To(MatchError(storePkg.ErrIPAlreadyReserved))
			}, checkInterval*5, checkInterval).Should(Succeed())
		}()
		Eventually(done, time.Minute).Should(BeClosed())
	})
})
