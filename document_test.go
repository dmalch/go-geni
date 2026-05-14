package geni

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetDocumentComments_Request(t *testing.T) {
	t.Run("GETs /api/<documentId>/comments and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetDocumentComments(context.Background(), "doc-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/doc-1/comments"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetDocumentComments(context.Background(), "doc-1", 2)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("2"))
	})

	t.Run("decodes results + pagination", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{
			"results": [
				{"id":"c-1","comment":"hi","created_at":"2026-01-01T00:00:00Z"},
				{"id":"c-2","comment":"there","title":"greeting","created_at":"2026-01-02T00:00:00Z"}
			],
			"page": 1,
			"next_page": "https://www.geni.com/api/doc-1/comments?page=2"
		}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.GetDocumentComments(context.Background(), "doc-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.Results[0].Id).To(Equal("c-1"))
		Expect(res.Results[0].Comment).To(Equal("hi"))
		Expect(res.Results[1].Title).To(Equal("greeting"))
		Expect(res.Page).To(Equal(1))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.GetDocumentComments(context.Background(), "doc-1", 0)

		Expect(err).To(MatchError(ErrResourceNotFound))
	})
}

func TestAddDocumentComment_Request(t *testing.T) {
	t.Run("POSTs to /api/<documentId>/comment with text and title", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.AddDocumentComment(context.Background(), "doc-1", "hello", "greeting")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/doc-1/comment"))
		Expect(ft.lastRequest.URL.Query().Get("text")).To(Equal("hello"))
		Expect(ft.lastRequest.URL.Query().Get("title")).To(Equal("greeting"))
	})

	t.Run("empty title is omitted from the query", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.AddDocumentComment(context.Background(), "doc-1", "hello", "")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("text")).To(Equal("hello"))
		Expect(ft.lastRequest.URL.Query().Has("title")).To(BeFalse())
	})

	t.Run("403 maps to ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)

		_, err := c.AddDocumentComment(context.Background(), "doc-1", "hello", "")

		Expect(err).To(MatchError(ErrAccessDenied))
	})
}

func TestGetDocumentProjects_Request(t *testing.T) {
	t.Run("GETs /api/<documentId>/projects and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.GetDocumentProjects(context.Background(), "doc-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/doc-1/projects"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("decodes ProjectBulkResponse with pagination", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{
			"results": [
				{"id":"project-1","name":"Family Roots"},
				{"id":"project-2","name":"Maternal Line"}
			],
			"page": 1,
			"next_page": "https://www.geni.com/api/doc-1/projects?page=2"
		}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.GetDocumentProjects(context.Background(), "doc-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.Results[0].Id).To(Equal("project-1"))
		Expect(res.Results[0].Name).To(Equal("Family Roots"))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})
}
