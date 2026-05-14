package geni

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestGeniIntegration is the Ginkgo bootstrap for the in-process
// integration tier. Plain testing.T units still run from their own
// TestXxx functions; this just lets `go test ./...` pick up the
// BDD-style integration specs registered via Describe / It. The
// sandbox tier lives under test/acceptance/ (see its own
// TestGeniAcceptance bootstrap).
func TestGeniIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "geni integration suite")
}
