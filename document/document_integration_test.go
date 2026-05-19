package document

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni/transport"
)

type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (r *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	return r.base.RoundTrip(req)
}

func newClientFor(server *httptest.Server) *Client {
	target, err := url.Parse(server.URL)
	Expect(err).ToNot(HaveOccurred())

	t := transport.New(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "acc-test"}), true)
	t.SetHTTPClient(&http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: target}})
	return NewClient(t)
}

const fixtureComments = `{
	"results": [
		{
			"id": "comment-1",
			"comment": "Looks like grandpa's hand!",
			"title": "Author note",
			"created_at": "2026-01-01T12:00:00Z"
		},
		{
			"id": "comment-2",
			"comment": "Agreed — the J on the signature matches.",
			"created_at": "2026-01-02T09:30:00Z"
		}
	],
	"page": 1,
	"next_page": "https://www.geni.com/api/doc-1/comments?page=2"
}`

const fixtureProjects = `{
	"results": [
		{
			"id": "project-100",
			"name": "Family Roots",
			"created_at": "2026-01-01T00:00:00Z",
			"updated_at": "2026-01-15T00:00:00Z"
		},
		{
			"id": "project-101",
			"name": "Maternal Line",
			"created_at": "2026-01-05T00:00:00Z"
		}
	],
	"page": 1
}`

var _ = Describe("Document comments + projects + tags endpoints", func() {
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

	Describe("Comments", func() {
		It("decodes results + pagination + forwards the page param", func() {
			serve(http.StatusOK, []byte(fixtureComments), http.MethodGet, "/api/doc-1/comments")

			res, err := client.Comments(ctx, "doc-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Id).To(Equal("comment-1"))
			Expect(res.Results[0].Title).To(Equal("Author note"))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("AddComment", func() {
		It("POSTs the text and optional title as query params", func() {
			serve(http.StatusOK, []byte(fixtureComments), http.MethodPost, "/api/doc-1/comment")

			_, err := client.AddComment(ctx, "doc-1", "Nice", "thanks")

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("text")).To(Equal("Nice"))
			Expect(recorded.URL.Query().Get("title")).To(Equal("thanks"))
		})
	})

	Describe("Projects", func() {
		It("decodes project.BulkResponse from the projects fixture", func() {
			serve(http.StatusOK, []byte(fixtureProjects), http.MethodGet, "/api/doc-1/projects")

			res, err := client.Projects(ctx, "doc-1", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Id).To(Equal("project-100"))
			Expect(res.Results[0].Name).To(Equal("Family Roots"))
		})
	})

	Describe("Tags", func() {
		It("decodes profile.BulkResponse for the tagged-profiles listing", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/doc-1/tags")

			res, err := client.Tags(ctx, "doc-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})
})
