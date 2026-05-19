package photo

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPhotoIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "photo integration suite")
}
