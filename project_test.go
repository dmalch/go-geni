package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetProjectProfiles_Request(t *testing.T) {
	t.Run("GETs /api/<projectId>/profiles and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetProjectProfiles(context.Background(), "project-7", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/project-7/profiles"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetProjectProfiles(context.Background(), "project-7", 3)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("3"))
	})

	t.Run("decodes results + pagination + total_count", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{
			"results": [
				{"id":"profile-1","first_name":"A"},
				{"id":"profile-2","first_name":"B"}
			],
			"page": 1,
			"total_count": 42,
			"next_page": "https://www.geni.com/api/project-7/profiles?page=2"
		}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.GetProjectProfiles(context.Background(), "project-7", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.Page).To(Equal(1))
		Expect(res.TotalCount).To(Equal(42))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.GetProjectProfiles(context.Background(), "project-7", 0)

		Expect(err).To(MatchError(ErrResourceNotFound))
	})
}

func TestGetProjectCollaborators_Request(t *testing.T) {
	t.Run("targets the collaborators sub-resource", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetProjectCollaborators(context.Background(), "project-7", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/project-7/collaborators"))
	})
}

func TestGetProjectFollowers_Request(t *testing.T) {
	t.Run("targets the followers sub-resource", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetProjectFollowers(context.Background(), "project-7", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/project-7/followers"))
	})
}
