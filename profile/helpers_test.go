package profile

import (
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestAddFields(t *testing.T) {
	t.Run("requests project_ids so bulk reads populate Profile.Projects", func(t *testing.T) {
		RegisterTestingT(t)
		req, err := http.NewRequest(http.MethodGet, "https://www.geni.com/api/profile-1", nil)
		Expect(err).ToNot(HaveOccurred())

		AddFields(req)

		fields := req.URL.Query().Get("fields")
		Expect(strings.Split(fields, ",")).To(ContainElement("project_ids"))
	})
}

func TestStripURLs(t *testing.T) {
	t.Run("strips the production API URL prefix from union URLs", func(t *testing.T) {
		RegisterTestingT(t)
		p := &Profile{Unions: []string{
			"https://www.geni.com/api/union-123",
			"https://www.geni.com/api/union-456",
		}}

		StripURLs(p, "https://www.geni.com/api/")

		Expect(p.Unions).To(Equal([]string{"union-123", "union-456"}))
	})

	t.Run("strips the sandbox API URL prefix from union URLs", func(t *testing.T) {
		RegisterTestingT(t)
		p := &Profile{Unions: []string{"https://api.sandbox.geni.com/union-789"}}

		StripURLs(p, "https://api.sandbox.geni.com/")

		Expect(p.Unions).To(Equal([]string{"union-789"}))
	})

	t.Run("handles empty unions", func(t *testing.T) {
		RegisterTestingT(t)
		p := &Profile{Unions: []string{}}

		StripURLs(p, "https://www.geni.com/api/")

		Expect(p.Unions).To(BeEmpty())
	})
}
