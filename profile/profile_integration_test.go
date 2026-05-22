package profile

import (
	"context"
	"io"
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

var _ = Describe("Profile endpoints", func() {
	var (
		ctx      context.Context
		server   *httptest.Server
		client   *Client
		recorded *http.Request
		reqBody  []byte
	)

	BeforeEach(func() {
		ctx = context.Background()
		recorded = nil
		reqBody = nil
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	serve := func(status int, respBody []byte, wantMethod, wantPath string) {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorded = r.Clone(r.Context())
			reqBody, _ = io.ReadAll(r.Body)
			Expect(r.Method).To(Equal(wantMethod))
			Expect(r.URL.Path).To(Equal(wantPath))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(respBody)
		}))
		client = newClientFor(server)
	}

	Describe("Create / Get / Update", func() {
		It("Create POSTs the request body and decodes the new profile", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-9","first_name":"Alice","public":true}`),
				http.MethodPost, "/api/profile/add")

			p, err := client.Create(ctx, &Request{
				Names:  map[string]NameElement{"en-US": {FirstName: new("Alice")}},
				Public: true,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(p.ID).To(Equal("profile-9"))
			Expect(string(reqBody)).To(ContainSubstring(`"first_name":"Alice"`))
		})

		It("Get strips the API URL prefix from unions", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-1","unions":["https://api.sandbox.geni.com/union-7"]}`),
				http.MethodGet, "/api/profile-1")

			p, err := client.Get(ctx, "profile-1")

			Expect(err).ToNot(HaveOccurred())
			Expect(p.Unions).To(ConsistOf("union-7"))
		})

		It("Update POSTs to /update and decodes the updated profile", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-1","first_name":"After"}`),
				http.MethodPost, "/api/profile-1/update")

			p, err := client.Update(ctx, "profile-1", &Request{
				Names: map[string]NameElement{"en-US": {FirstName: new("After")}},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(*p.FirstName).To(Equal("After"))
		})
	})

	Describe("Follow / Unfollow", func() {
		It("Follow POSTs to /follow and decodes the targeted profile", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-1","first_name":"A","public":true}`),
				http.MethodPost, "/api/profile-1/follow")

			p, err := client.Follow(ctx, "profile-1")

			Expect(err).ToNot(HaveOccurred())
			Expect(p.ID).To(Equal("profile-1"))
		})

		It("Unfollow targets /unfollow", func() {
			serve(http.StatusOK, []byte(`{"id":"profile-1"}`),
				http.MethodPost, "/api/profile-1/unfollow")

			_, err := client.Unfollow(ctx, "profile-1")

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Relationship adds", func() {
		It("AddParent POSTs the request body and forwards WithModifier", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-parent","first_name":"Mom"}`),
				http.MethodPost, "/api/profile-1/add-parent")

			parent, err := client.AddParent(ctx, "profile-1", &Request{
				Names: map[string]NameElement{"en-US": {FirstName: new("Mom")}},
			}, WithModifier("adopt"))

			Expect(err).ToNot(HaveOccurred())
			Expect(parent.ID).To(Equal("profile-parent"))
			Expect(recorded.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
			Expect(string(reqBody)).To(ContainSubstring(`"first_name":"Mom"`))
		})

		It("AddPartner POSTs to /add-partner", func() {
			serve(http.StatusOK, []byte(`{"id":"profile-200"}`),
				http.MethodPost, "/api/profile-1/add-partner")

			_, err := client.AddPartner(ctx, "profile-1")

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("UpdateBasics", func() {
		It("POSTs the basics body to /update-basics", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-1","first_name":"After"}`),
				http.MethodPost, "/api/profile-1/update-basics")

			updated, err := client.UpdateBasics(ctx, "profile-1", &Request{
				Names: map[string]NameElement{"en-US": {FirstName: new("After")}},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(*updated.FirstName).To(Equal("After"))
		})
	})

	Describe("WipeEventDates", func() {
		It("POSTs a date-wipe payload for the named events", func() {
			serve(http.StatusOK, []byte(`{"id":"profile-1"}`),
				http.MethodPost, "/api/profile-1/update")

			err := client.WipeEventDates(ctx, "profile-1", []string{"birth"})

			Expect(err).ToNot(HaveOccurred())
			Expect(string(reqBody)).To(ContainSubstring(`"birth":{"date":{}}`))
		})
	})
})
