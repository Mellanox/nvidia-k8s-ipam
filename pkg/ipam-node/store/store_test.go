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

package store_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store/storetest"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/types"
)

const (
	testPoolKey       = "pool1"
	testContainerID   = "id1"
	testNetIfName     = "net0"
	testPodUUID       = "a9516e9d-6f45-4693-b299-cc3d2f83e26a"
	testPodName       = "testPod"
	testNamespaceName = "testNamespace"
	testDeviceID      = "0000:d8:00.1"
	testIP            = "192.168.1.100"
	testIP2           = "192.168.1.200"
)

func createTestReservation(s storePkg.Session) {
	ExpectWithOffset(1, s.Reserve(testPoolKey, testContainerID, testNetIfName, types.ReservationMetadata{
		CreateTime:         time.Now().Format(time.RFC3339Nano),
		PodUUID:            testPodUUID,
		PodName:            testPodName,
		PodNamespace:       testNamespaceName,
		DeviceID:           testDeviceID,
		PoolConfigSnapshot: "something",
	}, net.ParseIP(testIP))).NotTo(HaveOccurred())
}

var _ = Describe("Store", func() {

	var (
		storePath string
		store     storePkg.Store
	)

	BeforeEach(func() {
		storePath = filepath.Join(GinkgoT().TempDir(), "store")
		store = storePkg.New(storePath)
	})

	It("Basic testing", func() {
		By("Open store")
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("Create reservation")
		createTestReservation(s)

		By("Check reservation exist")
		res := s.GetReservationByID(testPoolKey, testContainerID, testNetIfName)
		Expect(res).NotTo(BeNil())
		Expect(res.ContainerID).To(Equal(testContainerID))

		resList := s.ListReservations(testPoolKey)
		Expect(resList).To(HaveLen(1))
		Expect(resList[0].ContainerID).To(Equal(testContainerID))

		pools := s.ListPools()
		Expect(pools).To(Equal([]string{testPoolKey}))

		By("Check last reserved IP")
		Expect(s.GetLastReservedIP(testPoolKey)).To(Equal(net.ParseIP(testIP)))

		By("Set last reserved IP")
		newLastReservedIP := net.ParseIP("192.168.1.200")
		s.SetLastReservedIP(testPoolKey, newLastReservedIP)
		Expect(s.GetLastReservedIP(testPoolKey)).To(Equal(newLastReservedIP))

		By("Release reservation")
		s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, 0)

		By("Check reservation removed")
		Expect(s.GetReservationByID(testPoolKey, testContainerID, testNetIfName)).To(BeNil())

		By("Commit changes")
		Expect(s.Commit()).NotTo(HaveOccurred())
	})
	It("Commit should persist data", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		Expect(s.Commit()).NotTo(HaveOccurred())

		s, err = store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		res := s.GetReservationByID(testPoolKey, testContainerID, testNetIfName)
		Expect(res).NotTo(BeNil())
		Expect(res.ContainerID).To(Equal(testContainerID))
	})
	It("Cancel should rollback changes", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		s.Cancel()

		s, err = store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(s.GetReservationByID(testPoolKey, testContainerID, testNetIfName)).To(BeNil())
	})
	It("Closed session should panic", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		s.Cancel()
		Expect(func() { s.GetReservationByID(testPoolKey, testContainerID, testNetIfName) }).To(Panic())
		Expect(func() { s.ListReservations(testPoolKey) }).To(Panic())
		Expect(func() { s.GetLastReservedIP(testPoolKey) }).To(Panic())
	})
	It("Reload data from the disk", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		Expect(s.Commit()).NotTo(HaveOccurred())

		store2 := storePkg.New(storePath)
		s, err = store2.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(s.GetReservationByID(testPoolKey, testContainerID, testNetIfName)).NotTo(BeNil())
	})
	It("Concurrent access", func() {
		done := make(chan interface{})
		go func() {
			s, err := store.Open(context.Background())
			Expect(err).NotTo(HaveOccurred())
			ch := make(chan int, 2)
			wg := sync.WaitGroup{}
			wg.Add(2)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				time.Sleep(time.Millisecond * 100)
				createTestReservation(s)
				ch <- 1
				Expect(s.Commit()).NotTo(HaveOccurred())
			}()
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				s2, err := store.Open(context.Background())
				Expect(err).NotTo(HaveOccurred())
				ch <- 2
				Expect(s2.GetReservationByID(testPoolKey, testContainerID, testNetIfName)).NotTo(BeNil())
				s2.Cancel()
			}()
			wg.Wait()
			ret := make([]int, 0, 2)
		Loop:
			for {
				select {
				case i := <-ch:
					ret = append(ret, i)
				default:
					break Loop
				}
			}
			Expect(ret).To(HaveLen(2))
			Expect(ret[0]).To(Equal(1))
			Expect(ret[1]).To(Equal(2))

			close(done)
		}()
		Eventually(done, time.Minute).Should(BeClosed())
	})

	It("Invalid data on the disk", func() {
		Expect(os.WriteFile(storePath, []byte("something"), 0664)).NotTo(HaveOccurred())
		_, err := store.Open(context.Background())
		Expect(err).To(HaveOccurred())
	})
	It("Checksum mismatch", func() {
		// create valid store file
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		Expect(s.Commit()).NotTo(HaveOccurred())

		// patch checksum field
		storeRoot := &types.Root{}
		data, err := os.ReadFile(storePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(data, storeRoot))
		storeRoot.Checksum = 123455
		data, err = json.Marshal(storeRoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(storePath, data, 0664)).NotTo(HaveOccurred())

		// try to load the store file
		manager2 := storePkg.New(storePath)
		s, err = manager2.Open(context.Background())
		Expect(err).To(MatchError("store file corrupted, checksum mismatch"))
	})
	It("Already has allocation", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		Expect(
			s.Reserve(testPoolKey, testContainerID, testNetIfName,
				types.ReservationMetadata{}, net.ParseIP(testIP2))).To(MatchError(storePkg.ErrReservationAlreadyExist))
	})
	It("Duplicate IP allocation", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		Expect(
			s.Reserve(testPoolKey, "other", testNetIfName,
				types.ReservationMetadata{}, net.ParseIP(testIP))).To(MatchError(storePkg.ErrIPAlreadyReserved))
	})
	It("Remove pool data", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		Expect(s.ListReservations(testPoolKey)).NotTo(BeEmpty())
		s.RemovePool(testPoolKey)
		Expect(s.ListReservations(testPoolKey)).To(BeEmpty())
	})
	It("Release with a cooldown marks the reservation released instead of removing it", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)

		By("Release with a cooldown")
		s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Minute)

		By("GetReservationByID treats it as gone, since its owner already released it")
		Expect(s.GetReservationByID(testPoolKey, testContainerID, testNetIfName)).To(BeNil())

		By("But the entry is retained and still blocks the IP from reuse")
		res := storetest.FindReservation(s, testPoolKey, testContainerID, testNetIfName)
		Expect(res).NotTo(BeNil())
		Expect(res.IsReleased()).To(BeTrue())
		Expect(res.ReleasedAt).NotTo(BeZero())
		Expect(
			s.Reserve(testPoolKey, "other", testNetIfName,
				types.ReservationMetadata{}, net.ParseIP(testIP))).To(MatchError(storePkg.ErrIPAlreadyReserved))

		By("A retried release (e.g. idempotent CNI DEL) does not reset the release timestamp")
		firstReleasedAt := res.ReleasedAt
		s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Minute)
		res = storetest.FindReservation(s, testPoolKey, testContainerID, testNetIfName)
		Expect(res).NotTo(BeNil())
		Expect(res.ReleasedAt).To(Equal(firstReleasedAt))

		By("It is not removed before the cooldown elapses")
		s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Hour)
		Expect(storetest.FindReservation(s, testPoolKey, testContainerID, testNetIfName)).NotTo(BeNil())

		By("It is removed, freeing the IP, once the cooldown has elapsed")
		s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Nanosecond)
		Expect(storetest.FindReservation(s, testPoolKey, testContainerID, testNetIfName)).To(BeNil())
		Expect(
			s.Reserve(testPoolKey, "other", testNetIfName,
				types.ReservationMetadata{}, net.ParseIP(testIP))).NotTo(HaveOccurred())
	})
	It("Release with no cooldown removes the reservation immediately", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, 0)
		Expect(storetest.FindReservation(s, testPoolKey, testContainerID, testNetIfName)).To(BeNil())
	})
	It("Release is a no-op for an unknown reservation", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(func() { s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Minute) }).NotTo(Panic())
		Expect(s.GetReservationByID(testPoolKey, testContainerID, testNetIfName)).To(BeNil())

		By("it does not dirty the session, so Commit does not write to disk")
		Expect(s.Commit()).NotTo(HaveOccurred())
		_, err = os.Stat(storePath)
		Expect(err).To(MatchError(os.ErrNotExist))
	})
	It("Retrying a release before the cooldown elapses does not write to disk again", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		// first release: transitions the reservation to Released, which does need
		// persisting so it survives a restart
		Expect(s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Hour)).To(BeFalse())
		Expect(s.Commit()).NotTo(HaveOccurred())

		s, err = store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		fileInfoBefore, err := os.Stat(storePath)
		Expect(err).NotTo(HaveOccurred())

		// retried release (e.g. idempotent CNI DEL): already Released, cooldown not
		// elapsed yet -- a genuine no-op that should not dirty the session
		Expect(s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Hour)).To(BeFalse())
		Expect(s.Commit()).NotTo(HaveOccurred())

		fileInfoAfter, err := os.Stat(storePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(fileInfoAfter.ModTime()).To(Equal(fileInfoBefore.ModTime()))
	})
	It("Re-reserving the same id/ifName is rejected while its cooldown is pending, "+
		"even at the exact same address", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Minute)

		By("the cooldown is a floor on reuse of the address, including by its own former owner")
		Expect(
			s.Reserve(testPoolKey, testContainerID, testNetIfName,
				types.ReservationMetadata{}, net.ParseIP(testIP))).To(MatchError(storePkg.ErrReservationAlreadyExist))

		By("the reservation is untouched and still blocks the address")
		res := storetest.FindReservation(s, testPoolKey, testContainerID, testNetIfName)
		Expect(res).NotTo(BeNil())
		Expect(res.IPAddress).To(Equal(net.ParseIP(testIP)))
		Expect(res.IsReleased()).To(BeTrue())
	})
	It("A different id/ifName targeting a cooling-down address gets a retryable error", func() {
		s, err := store.Open(context.Background())
		Expect(err).NotTo(HaveOccurred())
		createTestReservation(s)
		s.ReleaseReservationByID(testPoolKey, testContainerID, testNetIfName, time.Minute)

		Expect(
			s.Reserve(testPoolKey, "other", testNetIfName,
				types.ReservationMetadata{}, net.ParseIP(testIP))).To(MatchError(storePkg.ErrIPAlreadyReserved))
	})
})
