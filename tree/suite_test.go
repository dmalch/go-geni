package tree

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTreeIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "tree integration suite")
}
