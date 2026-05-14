package geni

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client.GetUser", func() {
	var (
		ctx    context.Context
		server *httptest.Server
		client *Client
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	It("targets /api/user and decodes the User object", func() {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.Path).To(Equal("/api/user"))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "user-101",
				"guid": "g-user",
				"name": "Acceptance User",
				"account_type": "pro"
			}`))
		}))
		client = newClientFor(server)

		user, err := client.GetUser(ctx)

		Expect(err).ToNot(HaveOccurred())
		Expect(user.Id).To(Equal("user-101"))
		Expect(user.Guid).To(Equal("g-user"))
		Expect(user.Name).To(Equal("Acceptance User"))
		Expect(user.AccountType).To(Equal("pro"))
	})
})
