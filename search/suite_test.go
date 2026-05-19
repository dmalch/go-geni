package search

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestSearchIntegration is the Ginkgo bootstrap for the in-process
// integration tier of the search package.
func TestSearchIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "search integration suite")
}
