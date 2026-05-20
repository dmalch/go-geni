package profile

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProfileIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "profile integration suite")
}
