package selector_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"github.com/Mellanox/nvidia-k8s-ipam/pkg/ipam-controller/selector"
)

var _ = Describe("Selector", func() {
	It("Empty selector - match all", func() {
		s := selector.New()
		Expect(s.Match(&corev1.Node{})).To(BeTrue())
	})
	It("Match", func() {
		s := selector.New()
		labels := map[string]string{"foo": "bar"}
		s.Update(labels)
		node := &corev1.Node{}
		node.SetLabels(labels)
		Expect(s.Match(node)).To(BeTrue())
		Expect(s.Match(&corev1.Node{})).To(BeFalse())
		s.Update(map[string]string{"foobar": "foobar"})
		Expect(s.Match(node)).To(BeFalse())
	})
})
