package geni

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client document comments + projects endpoints", func() {
	var (
		ctx      context.Context
		server   *httptest.Server
		client   *Client
		recorded *http.Request
	)

	BeforeEach(func() {
		ctx = context.Background()
		recorded = nil
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	serve := func(status int, body []byte, wantMethod, wantPath string) {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorded = r.Clone(r.Context())
			Expect(r.Method).To(Equal(wantMethod))
			Expect(r.URL.Path).To(Equal(wantPath))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(body)
		}))
		client = newClientFor(server)
	}

	Describe("GetDocumentComments", func() {
		It("decodes results + pagination + forwards the page param", func() {
			serve(http.StatusOK, fixture("document_comments.json"), http.MethodGet, "/api/doc-1/comments")

			res, err := client.GetDocumentComments(ctx, "doc-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Id).To(Equal("comment-1"))
			Expect(res.Results[0].Title).To(Equal("Author note"))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("AddDocumentComment", func() {
		It("POSTs the text and optional title as query params", func() {
			serve(http.StatusOK, fixture("document_comments.json"), http.MethodPost, "/api/doc-1/comment")

			_, err := client.AddDocumentComment(ctx, "doc-1", "Nice", "thanks")

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("text")).To(Equal("Nice"))
			Expect(recorded.URL.Query().Get("title")).To(Equal("thanks"))
		})
	})

	Describe("GetDocumentProjects", func() {
		It("decodes ProjectBulkResponse from the projects fixture", func() {
			serve(http.StatusOK, fixture("document_projects.json"), http.MethodGet, "/api/doc-1/projects")

			res, err := client.GetDocumentProjects(ctx, "doc-1", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Id).To(Equal("project-100"))
			Expect(res.Results[0].Name).To(Equal("Family Roots"))
		})
	})
})
