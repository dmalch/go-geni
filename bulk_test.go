package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

// Bulk-read scenarios that aren't covered by the singular-id-fallback
// tests. Each resource gets a 3-id decode check plus error-mapping
// (404/403) verification on the bulk path. The singular path's error
// mapping is already pinned in the corresponding *_test.go files.

func TestGetProfiles_BulkThreeIds(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[
		{"id":"profile-1","first_name":"A"},
		{"id":"profile-2","first_name":"B"},
		{"id":"profile-3","first_name":"C"}
	]}`)

	res, err := c.Profile().GetBulk(context.Background(), []string{"profile-1", "profile-2", "profile-3"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("profile-1,profile-2,profile-3"))
	Expect(res.Results).To(HaveLen(3))

	ids := []string{res.Results[0].Id, res.Results[1].Id, res.Results[2].Id}
	Expect(ids).To(ConsistOf("profile-1", "profile-2", "profile-3"))
}

func TestGetProfiles_BulkErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Profile().GetBulk(context.Background(), []string{"profile-1", "profile-2"})
		Expect(err).To(MatchError(ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.Profile().GetBulk(context.Background(), []string{"profile-1", "profile-2"})
		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestGetUnions_BulkThreeIds(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[
		{"id":"union-1","status":"spouse"},
		{"id":"union-2","status":"spouse"},
		{"id":"union-3","status":"ex_spouse"}
	]}`)

	res, err := c.Union().GetBulk(context.Background(), []string{"union-1", "union-2", "union-3"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/union"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("union-1,union-2,union-3"))
	Expect(res.Results).To(HaveLen(3))
	Expect(res.Results[2].Status).To(Equal("ex_spouse"))
}

func TestGetUnions_BulkErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Union().GetBulk(context.Background(), []string{"union-1", "union-2"})
		Expect(err).To(MatchError(ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.Union().GetBulk(context.Background(), []string{"union-1", "union-2"})
		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestGetDocuments_BulkThreeIds(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[
		{"id":"document-1","title":"A"},
		{"id":"document-2","title":"B"},
		{"id":"document-3","title":"C"}
	]}`)

	res, err := c.Document().GetBulk(context.Background(), []string{"document-1", "document-2", "document-3"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/document"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("document-1,document-2,document-3"))
	Expect(res.Results).To(HaveLen(3))
	Expect(res.Results[2].Title).To(Equal("C"))
}

func TestGetDocuments_BulkErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Document().GetBulk(context.Background(), []string{"document-1", "document-2"})
		Expect(err).To(MatchError(ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.Document().GetBulk(context.Background(), []string{"document-1", "document-2"})
		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestGetPhotos_BulkThreeIds(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[
		{"id":"photo-1","title":"A"},
		{"id":"photo-2","title":"B"},
		{"id":"photo-3","title":"C"}
	]}`)

	res, err := c.Photo().GetBulk(context.Background(), []string{"photo-1", "photo-2", "photo-3"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("photo-1,photo-2,photo-3"))
	Expect(res.Results).To(HaveLen(3))
}

func TestGetPhotos_BulkErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Photo().GetBulk(context.Background(), []string{"photo-1", "photo-2"})
		Expect(err).To(MatchError(ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.Photo().GetBulk(context.Background(), []string{"photo-1", "photo-2"})
		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestGetVideos_BulkThreeIds(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[
		{"id":"video-1","title":"A"},
		{"id":"video-2","title":"B"},
		{"id":"video-3","title":"C"}
	]}`)

	res, err := c.Video().GetBulk(context.Background(), []string{"video-1", "video-2", "video-3"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("video-1,video-2,video-3"))
	Expect(res.Results).To(HaveLen(3))
}

func TestGetVideos_BulkErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Video().GetBulk(context.Background(), []string{"video-1", "video-2"})
		Expect(err).To(MatchError(ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.Video().GetBulk(context.Background(), []string{"video-1", "video-2"})
		Expect(err).To(MatchError(ErrAccessDenied))
	})
}
