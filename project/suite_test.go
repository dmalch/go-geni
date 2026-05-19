package project

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestProjectIntegration is the Ginkgo bootstrap for the in-process
// integration tier of the project package.
func TestProjectIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "project integration suite")
}
