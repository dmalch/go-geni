package geni

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestGeniSuite is the Ginkgo bootstrap. Plain testing.T units still run
// from their own TestXxx functions; this just lets `go test ./...` pick
// up the BDD-style acceptance specs registered via Describe / It.
func TestGeniSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "geni acceptance suite")
}
