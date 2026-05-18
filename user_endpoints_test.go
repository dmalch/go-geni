package geni

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetFollowedProfiles_Request(t *testing.T) {
	t.Run("GETs /api/user/followed-profiles and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetFollowedProfiles(context.Background(), 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/followed-profiles"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetFollowedProfiles(context.Background(), 3)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("3"))
	})

	t.Run("decodes profile results", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"next_page":"…?page=2"}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.GetFollowedProfiles(context.Background(), 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})
}

func TestGetFollowedDocuments_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[{"id":"document-1","title":"T"}],"page":1}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.GetFollowedDocuments(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/followed-documents"))
	Expect(res.Results).To(HaveLen(1))
}

func TestGetFollowedProjects_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[{"id":"project-1","name":"P"}]}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.GetFollowedProjects(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/followed-projects"))
	Expect(res.Results).To(HaveLen(1))
}

func TestGetFollowedSurnames_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[{"id":"surname-1","slugged_name":"smith"}]}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.GetFollowedSurnames(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/followed-surnames"))
	Expect(res.Results).To(HaveLen(1))
	Expect(res.Results[0].SluggedName).To(Equal("smith"))
}

func TestGetMaxFamily_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"profile-1"}],"page":1}`)

	res, err := c.GetMaxFamily(context.Background(), 2)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/max-family"))
	Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("2"))
	Expect(res.Results).To(HaveLen(1))
}

func TestGetUploadedPhotos_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"photo-1"}]}`)

	res, err := c.GetUploadedPhotos(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/uploaded-photos"))
	Expect(res.Results).To(HaveLen(1))
}

func TestGetUploadedVideos_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"video-1"}]}`)

	res, err := c.GetUploadedVideos(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/uploaded-videos"))
	Expect(res.Results).To(HaveLen(1))
}

func TestGetMyAlbums_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[{"id":"album-1","name":"Vacation"}],"page":1}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.GetMyAlbums(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/my-albums"))
	Expect(res.Results).To(HaveLen(1))
	Expect(res.Results[0].Name).To(Equal("Vacation"))
}

func TestGetMyLabels_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":["family","work","travel"],"page":1}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.GetMyLabels(context.Background(), 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/my-labels"))
	Expect(res.Results).To(ConsistOf("family", "work", "travel"))
}

func TestGetMetadata_Request(t *testing.T) {
	t.Run("self call has no ids= param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"data":{"theme":"dark"}}`)

		md, err := c.GetMetadata(context.Background())

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/metadata"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		// Data is opaque to the client; just confirm it round-tripped.
		Expect(string(md.Data)).To(ContainSubstring(`"theme"`))
	})

	t.Run("multi-user call sets ids=", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"data":{}}`)

		_, err := c.GetMetadata(context.Background(), "user-1", "user-2")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("user-1,user-2"))
	})
}

func TestUpdateMetadata_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"data":{"theme":"light"}}`)

	payload := json.RawMessage(`{"theme":"light"}`)
	md, err := c.UpdateMetadata(context.Background(), payload)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/user/update-metadata"))
	got, _ := io.ReadAll(ft.lastRequest.Body)
	// Geni expects data as a JSON-encoded string, so the wire
	// body is `{"data":"{\"theme\":\"light\"}"}` — inner quotes
	// are escaped.
	Expect(string(got)).To(ContainSubstring(`"data":"{\"theme\":\"light\"}"`))
	Expect(string(md.Data)).To(ContainSubstring(`"theme"`))
}

func TestUserEndpoints_ErrorMapping(t *testing.T) {
	t.Run("403 → ErrAccessDenied (followed-profiles)", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.GetFollowedProfiles(context.Background(), 0)
		Expect(err).To(MatchError(ErrAccessDenied))
	})
	t.Run("404 → ErrResourceNotFound (metadata)", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.GetMetadata(context.Background())
		Expect(err).To(MatchError(ErrResourceNotFound))
	})
}
