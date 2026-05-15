package geni

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Surname / Revision / Stats endpoints", func() {
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

	Describe("GetSurname", func() {
		It("decodes the documented Surname fields", func() {
			serve(http.StatusOK,
				[]byte(`{
					"id": "surname-1",
					"description": "Family surname",
					"slugged_name": "smith",
					"url": "https://www.geni.com/api/surname-1"
				}`),
				http.MethodGet, "/api/surname-1")

			s, err := client.GetSurname(ctx, "surname-1")

			Expect(err).ToNot(HaveOccurred())
			Expect(s.Id).To(Equal("surname-1"))
			Expect(s.Description).To(Equal("Family surname"))
			Expect(s.SluggedName).To(Equal("smith"))
			Expect(s.Url).To(ContainSubstring("/api/surname-1"))
		})
	})

	Describe("GetSurnameFollowers + GetSurnameProfiles", func() {
		It("targets the followers sub-resource and returns a paginated profile envelope", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/surname-1/followers")

			res, err := client.GetSurnameFollowers(ctx, "surname-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})

		It("targets the profiles sub-resource", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"}],"page":1}`),
				http.MethodGet, "/api/surname-1/profiles")

			res, err := client.GetSurnameProfiles(ctx, "surname-1", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})
	})

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

	Describe("GetStats", func() {
		It("returns the opaque stats array", func() {
			serve(http.StatusOK,
				[]byte(`{"stats":[{"name":"world_family_tree_size","value":250000000},{"name":"daily_searches","value":1000000}]}`),
				http.MethodGet, "/api/stats")

			res, err := client.GetStats(ctx)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Stats).To(HaveLen(2))
			Expect(string(res.Stats[0])).To(ContainSubstring("world_family_tree_size"))
		})
	})
})
