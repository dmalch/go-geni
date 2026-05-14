package geni

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client union add-* endpoints", func() {
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

	Describe("AddPartnerToUnion", func() {
		It("returns the newly-created partner profile", func() {
			serve(http.StatusOK, fixture("union_add_partner.json"), "/api/union-9/add-partner")

			partner, err := client.AddPartnerToUnion(ctx, "union-9")

			Expect(err).ToNot(HaveOccurred())
			Expect(partner.Id).To(Equal("profile-200"))
			Expect(partner.FirstName).ToNot(BeNil())
			Expect(*partner.FirstName).To(Equal("New"))
		})
	})

	Describe("AddChildToUnion", func() {
		It("forwards the relationship_modifier and returns the new child profile", func() {
			serve(http.StatusOK, fixture("union_add_child.json"), "/api/union-9/add-child")

			child, err := client.AddChildToUnion(ctx, "union-9", WithModifier("adopt"))

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
			Expect(child.Id).To(Equal("profile-201"))
			Expect(child.FirstName).ToNot(BeNil())
			Expect(*child.FirstName).To(Equal("New"))
		})
	})
})
