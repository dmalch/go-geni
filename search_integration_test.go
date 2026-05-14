package geni

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client.SearchProfiles", func() {
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
		serve(http.StatusOK, fixture("search_profiles.json"))

		res, err := client.SearchProfiles(ctx, "Smith", 1)

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

		_, err := client.SearchProfiles(ctx, "", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(recorded.URL.Query().Has("names")).To(BeFalse())
		Expect(recorded.URL.Query().Has("page")).To(BeFalse())
	})
})
