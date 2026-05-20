package union

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUnionIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "union integration suite")
}
