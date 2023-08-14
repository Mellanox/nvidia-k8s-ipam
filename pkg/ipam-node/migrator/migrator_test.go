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

package migrator_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/migrator"
	storePkg "github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-node/store"
)

const (
	testPool1    = "pool1"
	testPool2    = "pool2"
	testID1      = "09548191eb996fcce2c6bf4eff0576b8f3df9232931de7e499f084a2c3a34501"
	testID2      = "09548191eb996fcce2c6bf4eff0576b8f3df9232931de7e499f084a2c3a34502"
	testID3      = "09548191eb996fcce2c6bf4eff0576b8f3df9232931de7e499f084a2c3a34503"
	testIF1      = "net1"
	testIF2      = "net2"
	testPool1IP1 = "192.168.0.2"
	testPool1IP2 = "192.168.0.3"
	testPool1IP3 = "192.168.0.4"
	testPool2IP1 = "172.16.0.2"
)

var _ = Describe("Migrator", func() {
	var (
		hostLocalStorePath string
		ctx                context.Context
	)
	BeforeEach(func() {
		ctx = logr.NewContext(context.Background(), klog.NewKlogr())
		hostLocalStorePath = filepath.Join(GinkgoT().TempDir(), "host-local")
		Expect(os.Mkdir(hostLocalStorePath, 0744)).NotTo(HaveOccurred())
		pool1Dir := filepath.Join(hostLocalStorePath, testPool1)
		pool2Dir := filepath.Join(hostLocalStorePath, testPool2)
		Expect(os.Mkdir(pool1Dir, 0744)).NotTo(HaveOccurred())
		Expect(os.Mkdir(pool2Dir, 0744)).NotTo(HaveOccurred())

		Expect(os.WriteFile(filepath.Join(pool1Dir, "lock"), []byte(""), 0644)).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(pool2Dir, "lock"), []byte(""), 0644)).NotTo(HaveOccurred())

		Expect(os.WriteFile(filepath.Join(pool1Dir, "last_reserved_ip.0"),
			[]byte(fmt.Sprintf("%s", testPool1IP3)), 0644)).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(pool2Dir, "last_reserved_ip.0"),
			[]byte(fmt.Sprintf("%s", testPool2IP1)), 0644)).NotTo(HaveOccurred())

		Expect(os.WriteFile(filepath.Join(pool1Dir, testPool1IP1),
			[]byte(fmt.Sprintf("%s\r\n%s", testID1, testIF1)), 0644)).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(pool1Dir, testPool1IP2),
			[]byte(fmt.Sprintf("%s\r\n%s", testID2, testIF1)), 0644)).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(pool1Dir, testPool1IP3),
			[]byte(fmt.Sprintf("%s\r\n%s", testID3, testIF1)), 0644)).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(pool2Dir, testPool2IP1),
			[]byte(fmt.Sprintf("%s\r\n%s", testID1, testIF2)), 0644)).NotTo(HaveOccurred())
	})
	It("Migrate", func() {
		Expect(os.Setenv(migrator.EnvHostLocalStorePath, hostLocalStorePath)).NotTo(HaveOccurred())
		storePath := filepath.Join(GinkgoT().TempDir(), "test_store")
		m := migrator.New(storePkg.New(storePath))
		Expect(m.Migrate(ctx)).NotTo(HaveOccurred())

		session, err := storePkg.New(storePath).Open(ctx)
		defer session.Cancel()
		Expect(err).NotTo(HaveOccurred())

		Expect(session.GetLastReservedIP(testPool1)).NotTo(BeNil())
		Expect(session.GetLastReservedIP(testPool2)).NotTo(BeNil())

		reservationPool1 := session.GetReservationByID(testPool1, testID1, testIF1)
		Expect(reservationPool1).NotTo(BeNil())
		Expect(reservationPool1.ContainerID).To(Equal(testID1))
		Expect(reservationPool1.InterfaceName).To(Equal(testIF1))
		Expect(reservationPool1.IPAddress).To(Equal(net.ParseIP(testPool1IP1)))
		Expect(reservationPool1.Metadata.PodName).To(Equal(migrator.PlaceholderForUnknownField))
		Expect(reservationPool1.Metadata.PodNamespace).To(Equal(migrator.PlaceholderForUnknownField))
		Expect(reservationPool1.Metadata.PodUUID).To(Equal(migrator.PlaceholderForUnknownField))

		reservationPool2 := session.GetReservationByID(testPool2, testID1, testIF2)
		Expect(reservationPool2).NotTo(BeNil())

		// check that host local store is removed
		_, err = os.Stat(hostLocalStorePath)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
	It("No host-local path", func() {
		Expect(os.Setenv(migrator.EnvHostLocalStorePath,
			filepath.Join(hostLocalStorePath, "not-exist"))).NotTo(HaveOccurred())
		storePath := filepath.Join(GinkgoT().TempDir(), "test_store")
		m := migrator.New(storePkg.New(storePath))
		Expect(m.Migrate(ctx)).NotTo(HaveOccurred())
	})
	It("Empty host-local store", func() {
		hostLocalStorePath := filepath.Join(GinkgoT().TempDir(), "host-local2")
		Expect(os.Setenv(migrator.EnvHostLocalStorePath, hostLocalStorePath)).NotTo(HaveOccurred())
		Expect(os.Mkdir(hostLocalStorePath, 0744)).NotTo(HaveOccurred())
		storePath := filepath.Join(GinkgoT().TempDir(), "test_store")
		m := migrator.New(storePkg.New(storePath))
		Expect(m.Migrate(ctx)).NotTo(HaveOccurred())
	})
})
