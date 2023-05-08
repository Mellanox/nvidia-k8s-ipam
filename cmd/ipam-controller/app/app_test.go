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
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-controller/app"
	"github.com/Mellanox/nvidia-k8s-ipam/cmd/ipam-controller/app/options"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/config"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
)

const (
	pool1Name = "pool1"
	pool2Name = "pool2"
	pool3Name = "pool3"
)

func updateConfigMap(data string) {
	d := map[string]string{config.ConfigMapKey: data}
	err := k8sClient.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: TestConfigMapName, Namespace: TestNamespace},
		Data:       d,
	})
	if err == nil {
		return
	}
	if apiErrors.IsAlreadyExists(err) {
		configMap := &corev1.ConfigMap{}
		Expect(k8sClient.Get(
			ctx, types.NamespacedName{Name: TestConfigMapName, Namespace: TestNamespace}, configMap)).NotTo(HaveOccurred())
		configMap.Data = d
		Expect(k8sClient.Update(
			ctx, configMap)).NotTo(HaveOccurred())
	} else {
		Expect(err).NotTo(HaveOccurred())
	}
}

var validConfig = fmt.Sprintf(`
    {
      "pools": {
        "%s": { "subnet": "192.168.0.0/16", "perNodeBlockSize": 10 , "gateway": "192.168.0.1"},
        "%s": { "subnet": "172.16.0.0/16", "perNodeBlockSize": 50 , "gateway": "172.16.0.1"}
      }
    }
`, pool1Name, pool2Name)

// ranges for two nodes only can be allocated
var validConfig2 = fmt.Sprintf(`
    {
      "pools": {
        "%s": { "subnet": "172.17.0.0/24", "perNodeBlockSize": 100 , "gateway": "172.17.0.1"}
      },
      "nodeSelector": {"foo": "bar"}
    }
`, pool3Name)

var invalidConfig = `{{{`

func createNode(name string) *corev1.Node {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	Expect(k8sClient.Create(ctx, node)).NotTo(HaveOccurred())
	return node
}

func getNode(name string) *corev1.Node {
	node := &corev1.Node{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, node)).NotTo(HaveOccurred())
	return node
}

func updateNode(node *corev1.Node) *corev1.Node {
	Expect(k8sClient.Update(ctx, node)).NotTo(HaveOccurred())
	return node
}

func getRangeFromNode(nodeName string) map[string]*pool.IPPool {
	node := getNode(nodeName)
	mgr, err := pool.NewManagerImpl(node)
	if err != nil {
		return nil
	}
	return mgr.GetPools()
}

// WaitAndCheckForStability wait for condition and then check it is stable for 1 second
func WaitAndCheckForStability(check func(g Gomega), wait interface{}, stability interface{}) {
	Eventually(func(g Gomega) { check(g) }, wait).Should(Succeed())
	Consistently(func(g Gomega) { check(g) }, stability).Should(Succeed())
}

var _ = Describe("App", func() {
	It("Basic tests", func() {
		done := make(chan interface{})
		go func() {
			testNode1 := "node1"
			testNode2 := "node2"
			testNode3 := "node3"

			cfg1pools := []string{pool1Name, pool2Name}

			By("Create valid cfg1")
			updateConfigMap(validConfig)

			By("Set annotation with valid ranges for node1")
			node1 := createNode(testNode1)
			node1InitialRanges := map[string]*pool.IPPool{pool1Name: {
				Name:    pool1Name,
				Subnet:  "192.168.0.0/16",
				StartIP: "192.168.0.11",
				EndIP:   "192.168.0.20",
				Gateway: "192.168.0.1",
			}, pool2Name: {
				Name:    pool2Name,
				Subnet:  "172.16.0.0/16",
				StartIP: "172.16.0.1",
				EndIP:   "172.16.0.50",
				Gateway: "172.16.0.1",
			}}
			Expect(pool.SetIPBlockAnnotation(node1, node1InitialRanges)).NotTo(HaveOccurred())
			Expect(updateNode(node1))

			By("Set annotation with invalid ranges for node2")
			// create annotation for node2 with invalid config (wrong IP count)
			node2 := createNode(testNode2)
			node2InitialRanges := map[string]*pool.IPPool{pool1Name: {
				Name:    pool1Name,
				Subnet:  "192.168.0.0/16",
				StartIP: "192.168.0.11",
				EndIP:   "192.168.0.14",
				Gateway: "192.168.0.1",
			}}
			Expect(pool.SetIPBlockAnnotation(node2, node2InitialRanges)).NotTo(HaveOccurred())
			Expect(updateNode(node2))

			By("Start controller")

			ctrlCtx, ctrlStopFunc := context.WithCancel(ctx)
			defer ctrlStopFunc()
			controllerStopped := make(chan struct{})

			go func() {
				Expect(app.RunController(logr.NewContext(ctrlCtx, klog.NewKlogr()), cfg, &options.Options{
					ConfigMapName:      TestConfigMapName,
					ConfigMapNamespace: TestNamespace,
				})).NotTo(HaveOccurred())
				close(controllerStopped)
			}()

			By("Create node3 without annotation")

			createNode(testNode3)

			By("Check how controller handled the state")
			WaitAndCheckForStability(func(g Gomega) {
				uniqStartEndIPs := map[string]struct{}{}
				node1Ranges := getRangeFromNode(testNode1)
				node2Ranges := getRangeFromNode(testNode2)
				node3Ranges := getRangeFromNode(testNode3)
				for _, r := range []map[string]*pool.IPPool{node1Ranges, node2Ranges, node3Ranges} {
					g.Expect(r).To(HaveLen(len(cfg1pools)))
					for _, p := range cfg1pools {
						if r[p] != nil {
							uniqStartEndIPs[r[p].StartIP] = struct{}{}
							uniqStartEndIPs[r[p].EndIP] = struct{}{}
							g.Expect(r[p].Gateway).NotTo(BeEmpty())
							g.Expect(r[p].Subnet).NotTo(BeEmpty())
						}
					}
				}
				// we should have unique start/end IPs for each node for each pool
				g.Expect(uniqStartEndIPs).To(HaveLen(len(cfg1pools) * 3 * 2))
				// node1 should have restored ranges
				g.Expect(node1Ranges).To(Equal(node1InitialRanges))

			}, 15, 2)

			By("Set invalid config for controller")
			updateConfigMap(invalidConfig)
			time.Sleep(time.Second)

			By("Set valid cfg2, which ignores all nodes")

			updateConfigMap(validConfig2)

			By("Wait for controller to remove annotations")
			WaitAndCheckForStability(func(g Gomega) {
				g.Expect(getRangeFromNode(testNode1)).To(BeNil())
				g.Expect(getRangeFromNode(testNode2)).To(BeNil())
				g.Expect(getRangeFromNode(testNode3)).To(BeNil())
			}, 15, 2)

			By("Update nodes to match selector in cfg2")

			// node1 should have range
			node1 = getNode(testNode1)
			node1.Labels = map[string]string{"foo": "bar"}
			updateNode(node1)
			WaitAndCheckForStability(func(g Gomega) {
				g.Expect(getRangeFromNode(testNode1)).NotTo(BeNil())
			}, 15, 2)

			// node2 should have range
			node2 = getNode(testNode2)
			node2.Labels = map[string]string{"foo": "bar"}
			var node2Ranges map[string]*pool.IPPool
			updateNode(node2)
			WaitAndCheckForStability(func(g Gomega) {
				node2Ranges = getRangeFromNode(testNode2)
				g.Expect(node2Ranges).NotTo(BeNil())
			}, 15, 2)

			// node3 should have no range, because no free ranges available
			node3 := getNode(testNode3)
			node3.Labels = map[string]string{"foo": "bar"}
			updateNode(node3)
			WaitAndCheckForStability(func(g Gomega) {
				g.Expect(getRangeFromNode(testNode3)).To(BeNil())
			}, 15, 5)

			// remove label from node2, node3 should have a range now
			node2 = getNode(testNode2)
			node2.Labels = nil
			updateNode(node2)
			WaitAndCheckForStability(func(g Gomega) {
				node3Ranges := getRangeFromNode(testNode3)
				g.Expect(node3Ranges).NotTo(BeNil())
				// should reuse ranges from node2
				g.Expect(node3Ranges).To(Equal(node2Ranges))
			}, 15, 2)

			By("Stop controller")
			ctrlStopFunc()
			Eventually(controllerStopped, 30*time.Second).Should(BeClosed())

			close(done)
		}()
		Eventually(done, 5*time.Minute).Should(BeClosed())
	})
})
