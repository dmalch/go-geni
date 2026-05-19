package geni

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Revision endpoints", func() {
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

	serve := func(status int, body []byte, wantMethod, wantPath string) {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorded = r.Clone(r.Context())
			Expect(r.Method).To(Equal(wantMethod))
			Expect(r.URL.Path).To(Equal(wantPath))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(body)
		}))
		client = newClientFor(server)
	}

	Describe("GetRevision", func() {
		It("decodes the documented Revision fields", func() {
			serve(http.StatusOK,
				[]byte(`{
					"id": "revision-101",
					"guid": "g-rev-101",
					"action": "update",
					"date_local": "2026-05-15",
					"time_local": "09:00:00",
					"timestamp": "2026-05-15T09:00:00Z",
					"story": "<p>Updated birth date</p>"
				}`),
				http.MethodGet, "/api/revision-101")

			r, err := client.GetRevision(ctx, "revision-101")

			Expect(err).ToNot(HaveOccurred())
			Expect(r.Id).To(Equal("revision-101"))
			Expect(r.Action).To(Equal("update"))
			Expect(r.Story).To(ContainSubstring("<p>"))
		})
	})

	Describe("GetRevisions (bulk)", func() {
		It("2-id call hits /api/revision?ids=…", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"revision-101"},{"id":"revision-102"}]}`),
				http.MethodGet, "/api/revision")

			res, err := client.GetRevisions(ctx, []string{"revision-101", "revision-102"})

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("ids")).To(Equal("revision-101,revision-102"))
			Expect(res.Results).To(HaveLen(2))
		})

		It("1-id call falls back to /api/<id>", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"revision-101","action":"create"}`),
				http.MethodGet, "/api/revision-101")

			res, err := client.GetRevisions(ctx, []string{"revision-101"})

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Has("ids")).To(BeFalse())
			Expect(res.Results).To(HaveLen(1))
		})
	})

})
