package geni

import (
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client project sub-listings", func() {
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

	Describe("GetProjectProfiles", func() {
		It("decodes results, page, total_count, and pagination links", func() {
			serve(http.StatusOK, fixture("project_profiles.json"), "/api/project-7/profiles")

			res, err := client.GetProjectProfiles(ctx, "project-7", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Id).To(Equal("profile-501"))
			Expect(res.TotalCount).To(Equal(2))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("GetProjectCollaborators", func() {
		It("targets the collaborators sub-resource", func() {
			serve(http.StatusOK, fixture("project_profiles.json"), "/api/project-7/collaborators")

			res, err := client.GetProjectCollaborators(ctx, "project-7", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
		})
	})

	Describe("GetProjectFollowers", func() {
		It("targets the followers sub-resource", func() {
			serve(http.StatusOK, fixture("project_profiles.json"), "/api/project-7/followers")

			res, err := client.GetProjectFollowers(ctx, "project-7", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
		})
	})
})
