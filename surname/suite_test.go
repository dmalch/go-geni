package surname

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestSurnameIntegration is the Ginkgo bootstrap for the in-process
// integration tier of the surname package. Plain testing.T units
// still run from their own TestXxx functions.
func TestSurnameIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "surname integration suite")
}
