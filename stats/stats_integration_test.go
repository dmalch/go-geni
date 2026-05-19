package stats

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

var _ = Describe("Stats endpoint", func() {
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

	It("returns the opaque stats array", func() {
		serve(http.StatusOK,
			[]byte(`{"stats":[{"name":"world_family_tree_size","value":250000000},{"name":"daily_searches","value":1000000}]}`),
			http.MethodGet, "/api/stats")

		res, err := client.Get(ctx)

		Expect(err).ToNot(HaveOccurred())
		Expect(recorded.URL.Path).To(Equal("/api/stats"))
		Expect(res.Stats).To(HaveLen(2))
		Expect(string(res.Stats[0])).To(ContainSubstring("world_family_tree_size"))
	})
})
