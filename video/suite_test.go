package video

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVideoIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "video integration suite")
}
