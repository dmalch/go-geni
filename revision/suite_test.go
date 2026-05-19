package revision

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestRevisionIntegration is the Ginkgo bootstrap for the in-process
// integration tier of the revision package.
func TestRevisionIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "revision integration suite")
}
