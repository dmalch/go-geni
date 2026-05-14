package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

// Geni's bulk-by-id endpoints (/api/profile?ids=, /api/document?ids=,
// /api/union?ids=, /api/photo?ids=) silently return an empty
// `results` array when `ids` carries exactly one identifier. The
// client routes single-id calls through the corresponding singular
// GetX so callers see a consistent envelope. These tests pin the
// URL dispatch: 1 id → /api/<id>; ≥2 ids → /api/<resource>?ids=…

func TestGetProfiles_SingleIdFallback(t *testing.T) {
	t.Run("1 id → /api/<id>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1","first_name":"A"}`)

		res, err := c.GetProfiles(context.Background(), []string{"profile-1"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Id).To(Equal("profile-1"))
	})

	t.Run("2 ids → /api/profile?ids=…", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"profile-1"},{"id":"profile-2"}]}`)

		_, err := c.GetProfiles(context.Background(), []string{"profile-1", "profile-2"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("profile-1,profile-2"))
	})
}

func TestGetDocuments_SingleIdFallback(t *testing.T) {
	t.Run("1 id → /api/<id>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"document-1","title":"X"}`)

		res, err := c.GetDocuments(context.Background(), []string{"document-1"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/document-1"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Id).To(Equal("document-1"))
	})

	t.Run("2 ids → /api/document?ids=…", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"document-1"},{"id":"document-2"}]}`)

		_, err := c.GetDocuments(context.Background(), []string{"document-1", "document-2"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/document"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("document-1,document-2"))
	})
}

func TestGetPhotos_SingleIdFallback(t *testing.T) {
	t.Run("1 id → /api/<id>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1","title":"X"}`)

		res, err := c.GetPhotos(context.Background(), []string{"photo-1"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-1"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Id).To(Equal("photo-1"))
	})

	t.Run("2 ids → /api/photo?ids=…", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"photo-1"},{"id":"photo-2"}]}`)

		_, err := c.GetPhotos(context.Background(), []string{"photo-1", "photo-2"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("photo-1,photo-2"))
	})
}
