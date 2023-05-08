package app_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/config"
	"github.com/Mellanox/nvidia-k8s-ipam/pkg/pool"
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

var validConfig = `
    {
      "pools": {
        "pool1": { "subnet": "192.168.0.0/16", "perNodeBlockSize": 10 , "gateway": "192.168.0.1"},
        "pool2": { "subnet": "172.16.0.0/16", "perNodeBlockSize": 50 , "gateway": "172.16.0.1"}
      }
    }
`

// ranges for two nodes only can be allocated
var validConfig2 = `
    {
      "pools": {
        "pool3": { "subnet": "172.17.0.0/24", "perNodeBlockSize": 100 , "gateway": "172.17.0.1"}
      },
      "nodeSelector": {"foo": "bar"}
    }
`

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

		By("Create config and wait for controller to set ranges")

		updateConfigMap(validConfig)
		testNode1 := "node1"
		testNode2 := "node2"
		testNode3 := "node3"

		pools := []string{"pool1", "pool2"}

		createNode(testNode1)
		createNode(testNode2)
		createNode(testNode3)

		WaitAndCheckForStability(func(g Gomega) {
			uniqStartEndIPs := map[string]struct{}{}
			node1Ranges := getRangeFromNode(testNode1)
			node2Ranges := getRangeFromNode(testNode2)
			node3Ranges := getRangeFromNode(testNode3)
			for _, r := range []map[string]*pool.IPPool{node1Ranges, node2Ranges, node3Ranges} {
				g.Expect(r).To(HaveLen(len(pools)))
				for _, p := range pools {
					if r[p] != nil {
						uniqStartEndIPs[r[p].StartIP] = struct{}{}
						uniqStartEndIPs[r[p].EndIP] = struct{}{}
						g.Expect(r[p].Gateway).NotTo(BeEmpty())
						g.Expect(r[p].Subnet).NotTo(BeEmpty())
					}
				}
			}
			// we should have unique start/end IPs for each node for each pool
			g.Expect(uniqStartEndIPs).To(HaveLen(len(pools) * 3 * 2))
		}, 15, 2)

		By("Set invalid config")
		updateConfigMap(invalidConfig)
		time.Sleep(time.Second)

		By("Set valid config, which ignores all nodes")

		updateConfigMap(validConfig2)

		WaitAndCheckForStability(func(g Gomega) {
			g.Expect(getRangeFromNode(testNode1)).To(BeNil())
			g.Expect(getRangeFromNode(testNode2)).To(BeNil())
			g.Expect(getRangeFromNode(testNode3)).To(BeNil())
		}, 15, 2)

		By("Update node to match selector")

		// node1 should have range
		node1 := getNode(testNode1)
		node1.Labels = map[string]string{"foo": "bar"}
		updateNode(node1)
		WaitAndCheckForStability(func(g Gomega) {
			g.Expect(getRangeFromNode(testNode1)).NotTo(BeNil())
		}, 15, 2)

		// node2 should have range
		node2 := getNode(testNode2)
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
	})
})
