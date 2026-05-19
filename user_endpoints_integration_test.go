package geni

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("User endpoints", func() {
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

	Describe("Followed listings", func() {
		It("GetFollowedProfiles decodes a paginated ProfileBulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"total_count":17,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/user/followed-profiles")

			res, err := client.GetFollowedProfiles(ctx, 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.TotalCount).To(Equal(17))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
		})

		It("GetFollowedDocuments decodes a paginated DocumentBulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"document-1","title":"T"}],"page":1}`),
				http.MethodGet, "/api/user/followed-documents")

			res, err := client.GetFollowedDocuments(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})

		It("GetFollowedProjects decodes a paginated ProjectBulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"project-1","name":"P"}]}`),
				http.MethodGet, "/api/user/followed-projects")

			res, err := client.GetFollowedProjects(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})

		It("GetFollowedSurnames decodes a paginated surname.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"surname-1","slugged_name":"smith"},{"id":"surname-2","slugged_name":"jones"}],"page":1}`),
				http.MethodGet, "/api/user/followed-surnames")

			res, err := client.GetFollowedSurnames(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].SluggedName).To(Equal("smith"))
		})
	})

	Describe("Uploaded listings", func() {
		It("GetUploadedPhotos decodes a paginated PhotoBulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"photo-1","title":"X"}],"page":1}`),
				http.MethodGet, "/api/user/uploaded-photos")

			res, err := client.GetUploadedPhotos(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})

		It("GetUploadedVideos decodes a paginated VideoBulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"video-1","title":"X"}],"page":1}`),
				http.MethodGet, "/api/user/uploaded-videos")

			res, err := client.GetUploadedVideos(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})
	})

	Describe("My-* listings", func() {
		It("GetMyAlbums decodes a paginated PhotoAlbumBulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[
					{"id":"album-1","name":"Vacation","description":"Summer 2024"},
					{"id":"album-2","name":"Family"}
				],"page":1}`),
				http.MethodGet, "/api/user/my-albums")

			res, err := client.GetMyAlbums(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Name).To(Equal("Vacation"))
			Expect(res.Results[0].Description).To(Equal("Summer 2024"))
		})

		It("GetMyLabels decodes a string-array LabelsResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":["family","work","travel"],"page":1,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/user/my-labels")

			res, err := client.GetMyLabels(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(ConsistOf("family", "work", "travel"))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("MaxFamily", func() {
		It("decodes a paginated ProfileBulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1}`),
				http.MethodGet, "/api/user/max-family")

			res, err := client.GetMaxFamily(ctx, 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
		})
	})

	Describe("Metadata", func() {
		It("GetMetadata round-trips an opaque data blob", func() {
			serve(http.StatusOK,
				[]byte(`{"data":{"theme":"dark","sidebar":42}}`),
				http.MethodGet, "/api/user/metadata")

			md, err := client.GetMetadata(ctx)

			Expect(err).ToNot(HaveOccurred())
			Expect(string(md.Data)).To(ContainSubstring(`"theme":"dark"`))
			Expect(string(md.Data)).To(ContainSubstring(`"sidebar":42`))
		})

		It("GetMetadata with multiple user ids sets ids=", func() {
			serve(http.StatusOK, []byte(`{"data":{}}`),
				http.MethodGet, "/api/user/metadata")

			_, err := client.GetMetadata(ctx, "user-1", "user-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("ids")).To(Equal("user-1,user-2"))
		})

		It("UpdateMetadata POSTs data as a JSON-encoded string", func() {
			serve(http.StatusOK,
				[]byte(`{"data":{"theme":"light"}}`),
				http.MethodPost, "/api/user/update-metadata")

			_, err := client.UpdateMetadata(ctx, json.RawMessage(`{"theme":"light"}`))

			Expect(err).ToNot(HaveOccurred())
			// Wire format: `data` is a JSON-encoded string, not
			// a nested object — Geni's /update-metadata 500s on
			// the nested form ("no implicit conversion of
			// ActionController::Parameters into String").
			Expect(string(reqBody)).To(ContainSubstring(`"data":"{\"theme\":\"light\"}"`))
		})
	})
})
