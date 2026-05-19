package surname

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

// rewriteTransport redirects every outgoing request to a fixed
// httptest server target while preserving the original path + query.
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

var _ = Describe("Surname endpoints", func() {
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

	Describe("Get", func() {
		It("decodes the documented Surname fields", func() {
			serve(http.StatusOK,
				[]byte(`{
					"id": "surname-1",
					"description": "Family surname",
					"slugged_name": "smith",
					"url": "https://www.geni.com/api/surname-1"
				}`),
				http.MethodGet, "/api/surname-1")

			s, err := client.Get(ctx, "surname-1")

			Expect(err).ToNot(HaveOccurred())
			Expect(s.Id).To(Equal("surname-1"))
			Expect(s.Description).To(Equal("Family surname"))
			Expect(s.SluggedName).To(Equal("smith"))
			Expect(s.Url).To(ContainSubstring("/api/surname-1"))
		})
	})

	Describe("Followers + Profiles", func() {
		It("targets the followers sub-resource and returns a paginated profile envelope", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/surname-1/followers")

			res, err := client.Followers(ctx, "surname-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})

		It("targets the profiles sub-resource", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"}],"page":1}`),
				http.MethodGet, "/api/surname-1/profiles")

			res, err := client.Profiles(ctx, "surname-1", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})
	})
})
