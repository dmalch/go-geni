package union

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni/profile"
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

const fixtureAddPartner = `{
	"id": "profile-200",
	"guid": "g-newpartner",
	"first_name": "New",
	"last_name": "Partner",
	"gender": "female",
	"is_alive": true,
	"public": true
}`

const fixtureAddChild = `{
	"id": "profile-201",
	"guid": "g-newchild",
	"first_name": "New",
	"last_name": "Child",
	"is_alive": true,
	"public": true
}`

var _ = Describe("Union add-* endpoints", func() {
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
			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal(wantPath))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(body)
		}))
		client = newClientFor(server)
	}

	Describe("AddPartner", func() {
		It("returns the newly-created partner profile", func() {
			serve(http.StatusOK, []byte(fixtureAddPartner), "/api/union-9/add-partner")

			partner, err := client.AddPartner(ctx, "union-9")

			Expect(err).ToNot(HaveOccurred())
			Expect(partner.ID).To(Equal("profile-200"))
			Expect(partner.FirstName).ToNot(BeNil())
			Expect(*partner.FirstName).To(Equal("New"))
		})
	})

	Describe("AddChild", func() {
		It("forwards the relationship_modifier and returns the new child profile", func() {
			serve(http.StatusOK, []byte(fixtureAddChild), "/api/union-9/add-child")

			child, err := client.AddChild(ctx, "union-9", profile.WithModifier("adopt"))

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
			Expect(child.ID).To(Equal("profile-201"))
			Expect(child.FirstName).ToNot(BeNil())
			Expect(*child.FirstName).To(Equal("New"))
		})
	})
})
