package project

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

// fixtureProjectProfiles is the canonical multi-result project-profile
// listing response — inlined from the former testdata fixture.
const fixtureProjectProfiles = `{
	"results": [
		{
			"id": "profile-501",
			"first_name": "Eleanor",
			"last_name": "Project-Member",
			"is_alive": false,
			"public": true
		},
		{
			"id": "profile-502",
			"first_name": "Henry",
			"last_name": "Project-Member",
			"is_alive": false,
			"public": true
		}
	],
	"page": 1,
	"total_count": 2,
	"next_page": "https://www.geni.com/api/project-7/profiles?page=2"
}`

var _ = Describe("Project sub-listings", func() {
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

	serve := func(status int, body []byte, wantPath string) {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorded = r.Clone(r.Context())
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal(wantPath))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(body)
		}))
		client = newClientFor(server)
	}

	Describe("Profiles", func() {
		It("decodes results, page, total_count, and pagination links", func() {
			serve(http.StatusOK, []byte(fixtureProjectProfiles), "/api/project-7/profiles")

			res, err := client.Profiles(ctx, "project-7", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Id).To(Equal("profile-501"))
			Expect(res.TotalCount).To(Equal(2))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("Collaborators", func() {
		It("targets the collaborators sub-resource", func() {
			serve(http.StatusOK, []byte(fixtureProjectProfiles), "/api/project-7/collaborators")

			res, err := client.Collaborators(ctx, "project-7", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
		})
	})

	Describe("Followers", func() {
		It("targets the followers sub-resource", func() {
			serve(http.StatusOK, []byte(fixtureProjectProfiles), "/api/project-7/followers")

			res, err := client.Followers(ctx, "project-7", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
		})
	})
})
