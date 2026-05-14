package geni

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

// rewriteTransport directs a Client's requests at a local httptest.Server
// without changing the Client's externally-derived BaseURL.
type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return t.base.RoundTrip(clone)
}

func newClientFor(server *httptest.Server) *Client {
	target, err := url.Parse(server.URL)
	Expect(err).ToNot(HaveOccurred())

	c := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "acc-test"}), true)
	c.client = &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: target}}
	return c
}

func fixture(name string) []byte {
	data, err := os.ReadFile(filepath.Join("testdata", name))
	Expect(err).ToNot(HaveOccurred())
	return data
}

var _ = Describe("Client tree endpoints", func() {
	var (
		ctx    context.Context
		server *httptest.Server
		client *Client
		// recorded by the handler so specs can assert on the request that
		// reached the upstream
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
			Expect(r.URL.Path).To(Equal(wantPath))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(body)
		}))
		client = newClientFor(server)
	}

	Describe("GetImmediateFamily", func() {
		It("returns focus and decodes profile and union nodes from the fixture", func() {
			serve(http.StatusOK, fixture("immediate_family.json"), "/api/profile-1/immediate-family")

			res, err := client.GetImmediateFamily(ctx, "profile-1")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Focus).To(Equal("profile-1"))
			Expect(res.Nodes.ProfileIds()).To(ContainElements("profile-1", "profile-2", "profile-3"))
			Expect(res.Nodes.UnionIds()).To(ConsistOf("union-9"))

			partner, err := res.Nodes.Profile("profile-2")
			Expect(err).ToNot(HaveOccurred())
			Expect(partner.FirstName).ToNot(BeNil())
			Expect(*partner.FirstName).To(Equal("Bob"))

			union, err := res.Nodes.Union("union-9")
			Expect(err).ToNot(HaveOccurred())
			Expect(union.Partners).To(ConsistOf("profile-1", "profile-2"))
			Expect(union.Children).To(ConsistOf("profile-3"))
		})
	})

	Describe("GetAncestors", func() {
		It("propagates the requested generation depth", func() {
			serve(http.StatusOK, fixture("ancestors.json"), "/api/profile-1/ancestors")

			res, err := client.GetAncestors(ctx, "profile-1", WithGenerations(5))

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("generations")).To(Equal("5"))
			Expect(res.Nodes.ProfileIds()).To(HaveLen(5))
			Expect(res.Nodes.UnionIds()).To(ConsistOf("union-100", "union-200"))
		})

		It("clamps depth requests above the documented maximum of 20", func() {
			serve(http.StatusOK, fixture("ancestors.json"), "/api/profile-1/ancestors")

			_, err := client.GetAncestors(ctx, "profile-1", WithGenerations(50))

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("generations")).To(Equal("20"))
		})
	})

	Describe("GetPathTo", func() {
		It("returns Done with relations when the path is ready", func() {
			serve(http.StatusOK, fixture("path_to_done.json"), "/api/profile-1/path-to/profile-2")

			res, err := client.GetPathTo(ctx, "profile-1", "profile-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Status).To(Equal(PathStatusDone))
			Expect(res.Relationship).To(Equal("first cousin once removed"))
			Expect(res.Relations).ToNot(BeEmpty())
			Expect(res.Relations[0].Relation).To(Equal("self"))
		})

		It("returns Pending while Geni is computing (caller is expected to retry)", func() {
			serve(http.StatusOK, fixture("path_to_pending.json"), "/api/profile-1/path-to/profile-2")

			res, err := client.GetPathTo(ctx, "profile-1", "profile-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Status).To(Equal(PathStatusPending))
			Expect(res.Relations).To(BeEmpty())
		})

		It("returns NotFound when no path exists", func() {
			serve(http.StatusOK, fixture("path_to_not_found.json"), "/api/profile-1/path-to/profile-2")

			res, err := client.GetPathTo(ctx, "profile-1", "profile-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Status).To(Equal(PathStatusNotFound))
		})

		It("forwards path_type and suppression flags to the upstream query", func() {
			serve(http.StatusOK, fixture("path_to_done.json"), "/api/profile-1/path-to/profile-2")

			_, err := client.GetPathTo(ctx, "profile-1", "profile-2",
				WithPathType(PathTypeBlood),
				WithSkipEmail(true),
				WithSkipNotify(true),
			)

			Expect(err).ToNot(HaveOccurred())
			q := recorded.URL.Query()
			Expect(q.Get("path_type")).To(Equal("blood"))
			Expect(q.Get("skip_email")).To(Equal("true"))
			Expect(q.Get("skip_notify")).To(Equal("true"))
		})
	})
})
