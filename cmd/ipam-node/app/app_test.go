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

package app_test

import (
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	nodev1 "github.com/Mellanox/nvidia-k8s-ipam/api/grpc/nvidia/ipam/node/v1"
	ipamv1alpha1 "github.com/Mellanox/nvidia-k8s-ipam/api/v1alpha1"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-node/app"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-node/app/options"
)

const (
	testNodeName  = "test-node"
	testPodName   = "test-pod"
	testPoolName1 = "my-pool-1"
	testPoolName2 = "my-pool-2"
	testNamespace = "default"
)

func createTestPools() {
	pool1 := &ipamv1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{Name: testPoolName1, Namespace: testNamespace},
		Spec: ipamv1alpha1.IPPoolSpec{
			Subnet:           "192.168.0.0/16",
			PerNodeBlockSize: 252,
			Gateway:          "192.168.0.1",
		},
	}
	ExpectWithOffset(1, k8sClient.Create(ctx, pool1))

	pool2 := &ipamv1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{Name: testPoolName2, Namespace: testNamespace},
		Spec: ipamv1alpha1.IPPoolSpec{
			Subnet:           "10.100.0.0/16",
			PerNodeBlockSize: 252,
			Gateway:          "10.100.0.1",
		},
	}
	ExpectWithOffset(1, k8sClient.Create(ctx, pool2))

	// Update statuses with range allocation
	Eventually(func(g Gomega) error {
		status := ipamv1alpha1.IPPoolStatus{
			Allocations: []ipamv1alpha1.Allocation{
				{
					NodeName: testNodeName,
					StartIP:  "192.168.0.2",
					EndIP:    "192.168.0.254",
				},
			},
		}
		return updatePoolStatus(testPoolName1, status)
	}, 30, 5).Should(Not(HaveOccurred()))

	Eventually(func(g Gomega) error {
		status := ipamv1alpha1.IPPoolStatus{
			Allocations: []ipamv1alpha1.Allocation{
				{
					NodeName: testNodeName,
					StartIP:  "10.100.0.2",
					EndIP:    "10.100.0.254",
				},
			},
		}
		return updatePoolStatus(testPoolName2, status)
	}, 30, 5).Should(Not(HaveOccurred()))
}

func updatePoolStatus(poolName string, status ipamv1alpha1.IPPoolStatus) error {
	pool := &ipamv1alpha1.IPPool{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: poolName, Namespace: testNamespace}, pool)
	if err != nil {
		return err
	}
	pool.Status = status
	err = k8sClient.Status().Update(ctx, pool)
	if err != nil {
		return err
	}
	return nil
}

func createTestPod() *corev1.Pod {
	podObj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: testPodName, Namespace: testNamespace},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "name", Image: "image"}},
		},
	}
	ExpectWithOffset(1, k8sClient.Create(ctx, podObj))
	return podObj
}

func getOptions(testDir string) *options.Options {
	daemonSocket := "unix://" + filepath.Join(testDir, "daemon")
	storePath := filepath.Join(testDir, "store")
	cniBinDir := filepath.Join(testDir, "cnibin")
	cniConfDir := filepath.Join(testDir, "cniconf")
	dummyCNIBin := filepath.Join(testDir, "dummycni")

	Expect(os.WriteFile(dummyCNIBin, []byte("dummy"), 0777)).NotTo(HaveOccurred())
	Expect(os.Mkdir(cniBinDir, 0777)).NotTo(HaveOccurred())
	Expect(os.Mkdir(cniConfDir, 0777)).NotTo(HaveOccurred())

	opts := options.New()
	opts.NodeName = testNodeName
	opts.ProbeAddr = "0"   // disable
	opts.MetricsAddr = "0" // disable
	opts.BindAddress = daemonSocket
	opts.StoreFile = storePath
	opts.CNIBinFile = dummyCNIBin
	opts.CNIBinDir = cniBinDir
	opts.CNIConfDir = cniConfDir
	opts.CNIDaemonSocket = daemonSocket
	opts.PoolsNamespace = testNamespace
	return opts
}

func getValidReqParams(uid, name, namespace string) *nodev1.IPAMParameters {
	return &nodev1.IPAMParameters{
		Pools:          []string{testPoolName1, testPoolName2},
		CniContainerid: "id1",
		CniIfname:      "net0",
		Metadata: &nodev1.IPAMMetadata{
			K8SPodName:      name,
			K8SPodNamespace: namespace,
			K8SPodUid:       uid,
			DeviceId:        "0000:d8:00.1",
		},
	}
}

var _ = Describe("IPAM Node daemon", func() {
	It("Validate main flows", func() {
		done := make(chan interface{})
		go func() {
			testDir := GinkgoT().TempDir()
			opts := getOptions(testDir)

			createTestPools()
			pod := createTestPod()

			ctx = logr.NewContext(ctx, klog.NewKlogr())

			go func() {
				Expect(app.RunNodeDaemon(ctx, cfg, opts)).NotTo(HaveOccurred())
			}()

			conn, err := grpc.DialContext(ctx, opts.CNIDaemonSocket,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock())
			Expect(err).NotTo(HaveOccurred())

			grpcClient := nodev1.NewIPAMServiceClient(conn)

			params := getValidReqParams(string(pod.UID), pod.Name, pod.Namespace)

			// no allocation yet
			_, err = grpcClient.IsAllocated(ctx,
				&nodev1.IsAllocatedRequest{Parameters: params})
			Expect(status.Code(err) == codes.NotFound).To(BeTrue())

			// allocate
			resp, err := grpcClient.Allocate(ctx, &nodev1.AllocateRequest{Parameters: params})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Allocations).To(HaveLen(2))
			Expect(resp.Allocations[0].Pool).NotTo(BeEmpty())
			Expect(resp.Allocations[0].Gateway).NotTo(BeEmpty())
			Expect(resp.Allocations[0].Ip).NotTo(BeEmpty())

			_, err = grpcClient.IsAllocated(ctx,
				&nodev1.IsAllocatedRequest{Parameters: params})
			Expect(err).NotTo(HaveOccurred())

			// deallocate
			_, err = grpcClient.Deallocate(ctx, &nodev1.DeallocateRequest{Parameters: params})
			Expect(err).NotTo(HaveOccurred())

			// deallocate should be idempotent
			_, err = grpcClient.Deallocate(ctx, &nodev1.DeallocateRequest{Parameters: params})
			Expect(err).NotTo(HaveOccurred())

			// check should fail
			_, err = grpcClient.IsAllocated(ctx,
				&nodev1.IsAllocatedRequest{Parameters: params})
			Expect(status.Code(err) == codes.NotFound).To(BeTrue())
			close(done)
		}()
		Eventually(done, 5*time.Minute).Should(BeClosed())
	})
})
