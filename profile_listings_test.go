package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetProfileDocuments_Request(t *testing.T) {
	t.Run("GETs /api/<profileId>/documents and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetProfileDocuments(context.Background(), "profile-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/documents"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetProfileDocuments(context.Background(), "profile-1", 2)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("2"))
	})

	t.Run("decodes results + pagination + total_count", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{
			"results": [
				{"id":"document-1","title":"Birth certificate"},
				{"id":"document-2","title":"Marriage record"}
			],
			"page": 1,
			"total_count": 17,
			"next_page": "https://www.geni.com/api/profile-1/documents?page=2"
		}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.GetProfileDocuments(context.Background(), "profile-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.Results[0].Id).To(Equal("document-1"))
		Expect(res.TotalCount).To(Equal(17))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.GetProfileDocuments(context.Background(), "profile-1", 0)

		Expect(err).To(MatchError(ErrResourceNotFound))
	})
}

func TestGetProfilePhotos_Request(t *testing.T) {
	t.Run("GETs /api/<profileId>/photos", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetProfilePhotos(context.Background(), "profile-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/photos"))
	})

	t.Run("decodes results + pagination", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{
			"results": [
				{"id":"photo-9","title":"Family portrait"}
			],
			"page": 1,
			"next_page": "https://www.geni.com/api/profile-1/photos?page=2"
		}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.GetProfilePhotos(context.Background(), "profile-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Id).To(Equal("photo-9"))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})
}
