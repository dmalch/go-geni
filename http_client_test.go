package geni

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestBaseURL(t *testing.T) {
	t.Run("Returns production URL when not using sandbox", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(BaseURL(false)).To(Equal("https://www.geni.com/"))
	})

	t.Run("Returns sandbox URL when using sandbox", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(BaseURL(true)).To(Equal("https://sandbox.geni.com/"))
	})
}

func TestApiUrl(t *testing.T) {
	t.Run("Returns production API URL when not using sandbox", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(apiUrl(false)).To(Equal("https://www.geni.com/api/"))
	})

	t.Run("Returns sandbox API URL when using sandbox", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(apiUrl(true)).To(Equal("https://api.sandbox.geni.com/"))
	})
}
