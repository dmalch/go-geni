package photoalbum

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPhotoAlbumIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "photoalbum integration suite")
}
