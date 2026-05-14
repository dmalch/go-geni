package acceptance

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestGeniAcceptance is the Ginkgo bootstrap for the sandbox acceptance
// suite. Each spec calls newTestClient(), which is what triggers Skip()
// when no sandbox token is reachable — the suite itself is always
// registered with `go test`, but every spec self-skips if the
// environment isn't set up.
func TestGeniAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "geni sandbox acceptance suite")
}
