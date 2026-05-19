package user

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestUserIntegration is the Ginkgo bootstrap for the in-process
// integration tier of the user package.
func TestUserIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "user integration suite")
}
