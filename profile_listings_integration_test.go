package geni

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client profile media listings", func() {
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

	Describe("GetProfileDocuments", func() {
		It("decodes the document bulk envelope with pagination + total_count", func() {
			serve(http.StatusOK, []byte(`{
				"results": [
					{"id":"document-1","title":"Birth certificate"},
					{"id":"document-2","title":"Marriage record"}
				],
				"page": 1,
				"total_count": 17,
				"next_page": "…?page=2"
			}`), "/api/profile-1/documents")

			res, err := client.GetProfileDocuments(ctx, "profile-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Title).To(Equal("Birth certificate"))
			Expect(res.TotalCount).To(Equal(17))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("GetProfilePhotos", func() {
		It("decodes the photo bulk envelope with pagination links", func() {
			serve(http.StatusOK, []byte(`{
				"results": [
					{"id":"photo-100","title":"Family portrait","sizes":{"small":"https://x/small.jpg"}}
				],
				"page": 1,
				"next_page": "…?page=2"
			}`), "/api/profile-1/photos")

			res, err := client.GetProfilePhotos(ctx, "profile-1", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Has("page")).To(BeFalse())
			Expect(res.Results).To(HaveLen(1))
			Expect(res.Results[0].Id).To(Equal("photo-100"))
			Expect(res.Results[0].Sizes).To(HaveKeyWithValue("small", "https://x/small.jpg"))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})
})
