package tree

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
	clone := req.Clone(req.Context())
	clone.URL.Scheme = r.target.Scheme
	clone.URL.Host = r.target.Host
	return r.base.RoundTrip(clone)
}

func newClientFor(server *httptest.Server) *Client {
	target, err := url.Parse(server.URL)
	Expect(err).ToNot(HaveOccurred())

	t := transport.New(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "acc-test"}), true)
	t.SetHTTPClient(&http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: target}})
	return NewClient(t)
}

// Family-graph fixtures, inlined from the former testdata/*.json files.
const (
	fixtureImmediateFamily = `{
		"focus": {"id":"profile-1","first_name":"Alice","last_name":"Smith","gender":"female","is_alive":true,"public":true},
		"nodes": {
			"profile-1": {"id":"profile-1","first_name":"Alice","last_name":"Smith","gender":"female","is_alive":true,"public":true},
			"profile-2": {"id":"profile-2","first_name":"Bob","last_name":"Smith","gender":"male","is_alive":true,"public":true},
			"profile-3": {"id":"profile-3","first_name":"Carol","last_name":"Smith","gender":"female","is_alive":true,"public":true},
			"union-9": {"id":"union-9","partners":["profile-1","profile-2"],"children":["profile-3"],"status":"spouse"}
		}
	}`

	fixtureAncestors = `{
		"focus": {"id":"profile-1","first_name":"Alice"},
		"nodes": {
			"profile-1": {"id":"profile-1","first_name":"Alice"},
			"profile-10": {"id":"profile-10","first_name":"Edward"},
			"profile-11": {"id":"profile-11","first_name":"Mary"},
			"profile-20": {"id":"profile-20","first_name":"Henry"},
			"profile-21": {"id":"profile-21","first_name":"Margaret"},
			"union-100": {"id":"union-100","partners":["profile-10","profile-11"],"children":["profile-1"]},
			"union-200": {"id":"union-200","partners":["profile-20","profile-21"],"children":["profile-10"]}
		}
	}`

	fixturePathToDone = `{
		"status": "done",
		"relationship": "first cousin once removed",
		"relations": [
			{"id":"profile-1","relation":"self","next_id":"union-100"},
			{"id":"union-100","relation":"mother","next_id":"profile-11"},
			{"id":"profile-11","relation":"mother","next_id":"union-200"},
			{"id":"union-200","relation":"son","next_id":"profile-2"}
		]
	}`

	fixturePathToPending  = `{"status": "pending"}`
	fixturePathToNotFound = `{"status": "not found"}`
)

var _ = Describe("Tree endpoints", func() {
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
			Expect(r.URL.Path).To(Equal(wantPath))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(body)
		}))
		client = newClientFor(server)
	}

	Describe("ImmediateFamily", func() {
		It("returns focus and decodes profile and union nodes from the fixture", func() {
			serve(http.StatusOK, []byte(fixtureImmediateFamily), "/api/profile-1/immediate-family")

			res, err := client.ImmediateFamily(ctx, "profile-1")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Focus).ToNot(BeNil())
			Expect(res.Focus.ID).To(Equal("profile-1"))
			Expect(res.Nodes.ProfileIds()).To(ContainElements("profile-1", "profile-2", "profile-3"))
			Expect(res.Nodes.UnionIds()).To(ConsistOf("union-9"))

			partner, err := res.Nodes.Profile("profile-2")
			Expect(err).ToNot(HaveOccurred())
			Expect(partner.FirstName).ToNot(BeNil())
			Expect(*partner.FirstName).To(Equal("Bob"))

			u, err := res.Nodes.Union("union-9")
			Expect(err).ToNot(HaveOccurred())
			Expect(u.Partners).To(ConsistOf("profile-1", "profile-2"))
			Expect(u.Children).To(ConsistOf("profile-3"))
		})
	})

	Describe("Ancestors", func() {
		It("propagates the requested generation depth", func() {
			serve(http.StatusOK, []byte(fixtureAncestors), "/api/profile-1/ancestors")

			res, err := client.Ancestors(ctx, "profile-1", WithGenerations(5))

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("generations")).To(Equal("5"))
			Expect(res.Nodes.ProfileIds()).To(HaveLen(5))
			Expect(res.Nodes.UnionIds()).To(ConsistOf("union-100", "union-200"))
		})

		It("clamps depth requests above the documented maximum of 20", func() {
			serve(http.StatusOK, []byte(fixtureAncestors), "/api/profile-1/ancestors")

			_, err := client.Ancestors(ctx, "profile-1", WithGenerations(50))

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("generations")).To(Equal("20"))
		})
	})

	Describe("PathTo", func() {
		It("returns Done with relations when the path is ready", func() {
			serve(http.StatusOK, []byte(fixturePathToDone), "/api/profile-1/path-to/profile-2")

			res, err := client.PathTo(ctx, "profile-1", "profile-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Status).To(Equal(PathStatusDone))
			Expect(res.Relationship).To(Equal("first cousin once removed"))
			Expect(res.Relations).ToNot(BeEmpty())
			Expect(res.Relations[0].Relation).To(Equal("self"))
		})

		It("returns Pending while Geni is computing (caller is expected to retry)", func() {
			serve(http.StatusOK, []byte(fixturePathToPending), "/api/profile-1/path-to/profile-2")

			res, err := client.PathTo(ctx, "profile-1", "profile-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Status).To(Equal(PathStatusPending))
			Expect(res.Relations).To(BeEmpty())
		})

		It("returns NotFound when no path exists", func() {
			serve(http.StatusOK, []byte(fixturePathToNotFound), "/api/profile-1/path-to/profile-2")

			res, err := client.PathTo(ctx, "profile-1", "profile-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Status).To(Equal(PathStatusNotFound))
		})

		It("forwards path_type and suppression flags to the upstream query", func() {
			serve(http.StatusOK, []byte(fixturePathToDone), "/api/profile-1/path-to/profile-2")

			_, err := client.PathTo(ctx, "profile-1", "profile-2",
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

var _ = Describe("Tree Compare", func() {
	var (
		ctx    context.Context
		server *httptest.Server
		client *Client
	)

	BeforeEach(func() { ctx = context.Background() })
	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	It("GETs /compare/<other> and decodes both family graphs", func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal("/api/profile-1/compare/profile-2"))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"results":[
				{"focus":{"id":"profile-1"},"nodes":{"profile-1":{"id":"profile-1"}}},
				{"focus":{"id":"profile-2"},"nodes":{"profile-2":{"id":"profile-2"}}}
			]}`))
		}))
		client = newClientFor(server)

		cmp, err := client.Compare(ctx, "profile-1", "profile-2")

		Expect(err).ToNot(HaveOccurred())
		Expect(cmp.Results).To(HaveLen(2))
		Expect(cmp.Results[0].Focus.ID).To(Equal("profile-1"))
		Expect(cmp.Results[1].Nodes).To(HaveKey("profile-2"))
	})
})
