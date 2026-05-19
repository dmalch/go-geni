package document

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDocumentIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "document integration suite")
}
