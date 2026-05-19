package stats

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestStatsIntegration is the Ginkgo bootstrap for the in-process
// integration tier of the stats package. Plain testing.T units still
// run from their own TestXxx functions; this lets `go test ./...`
// pick up the BDD-style integration specs registered via Describe.
func TestStatsIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "stats integration suite")
}
