package ip_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ip Suite")
}
