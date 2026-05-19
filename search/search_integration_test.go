package search

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

// fixtureSearchProfiles is the canonical multi-result search response
// previously stored as testdata/search_profiles.json — inlined here so
// the search/ package doesn't need a testdata/ directory of its own.
const fixtureSearchProfiles = `{
	"results": [
		{
			"id": "profile-101",
			"guid": "g-aaaa-1",
			"first_name": "Jane",
			"last_name": "Smith",
			"gender": "female",
			"is_alive": true,
			"public": true
		},
		{
			"id": "profile-102",
			"guid": "g-aaaa-2",
			"first_name": "John",
			"last_name": "Smith",
			"gender": "male",
			"is_alive": true,
			"public": true
		}
	],
	"page": 1,
	"next_page": "https://www.geni.com/api/profile/search?names=Smith&page=2"
}`

var _ = Describe("Search endpoints", func() {
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

	serve := func(status int, body []byte) {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorded = r.Clone(r.Context())
			Expect(r.URL.Path).To(Equal("/api/profile/search"))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(body)
		}))
		client = newClientFor(server)
	}

	It("forwards the names query and decodes results plus pagination", func() {
		serve(http.StatusOK, []byte(fixtureSearchProfiles))

		res, err := client.Profiles(ctx, "Smith", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(recorded.URL.Query().Get("names")).To(Equal("Smith"))
		Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
		Expect(res.Results).To(HaveLen(2))
		Expect(res.Results[0].Id).To(Equal("profile-101"))
		Expect(res.Results[1].Id).To(Equal("profile-102"))
		Expect(res.Page).To(Equal(1))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})

	It("omits empty names and zero page from the upstream query", func() {
		serve(http.StatusOK, []byte(`{"results":[]}`))

		_, err := client.Profiles(ctx, "", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(recorded.URL.Query().Has("names")).To(BeFalse())
		Expect(recorded.URL.Query().Has("page")).To(BeFalse())
	})
})
