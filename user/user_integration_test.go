package user

import (
	"context"
	"encoding/json"
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
		It("FollowedProfiles decodes a paginated profile.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"total_count":17,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/user/followed-profiles")

			res, err := client.FollowedProfiles(ctx, 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.TotalCount).To(Equal(17))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
		})

		It("FollowedDocuments decodes a paginated document.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"document-1","title":"T"}],"page":1}`),
				http.MethodGet, "/api/user/followed-documents")

			res, err := client.FollowedDocuments(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})

		It("FollowedProjects decodes a paginated project.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"project-1","name":"P"}]}`),
				http.MethodGet, "/api/user/followed-projects")

			res, err := client.FollowedProjects(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})

		It("FollowedSurnames decodes a paginated surname.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"surname-1","slugged_name":"smith"},{"id":"surname-2","slugged_name":"jones"}],"page":1}`),
				http.MethodGet, "/api/user/followed-surnames")

			res, err := client.FollowedSurnames(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].SluggedName).To(Equal("smith"))
		})
	})

	Describe("Uploaded listings", func() {
		It("UploadedPhotos decodes a paginated photo.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"photo-1","title":"X"}],"page":1}`),
				http.MethodGet, "/api/user/uploaded-photos")

			res, err := client.UploadedPhotos(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})

		It("UploadedVideos decodes a paginated video.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"video-1","title":"X"}],"page":1}`),
				http.MethodGet, "/api/user/uploaded-videos")

			res, err := client.UploadedVideos(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})
	})

	Describe("My-* listings", func() {
		It("Albums decodes a paginated photoalbum.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[
					{"id":"album-1","name":"Vacation","description":"Summer 2024"},
					{"id":"album-2","name":"Family"}
				],"page":1}`),
				http.MethodGet, "/api/user/my-albums")

			res, err := client.Albums(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Name).To(Equal("Vacation"))
			Expect(res.Results[0].Description).To(Equal("Summer 2024"))
		})

		It("Labels decodes a string-array Labels envelope", func() {
			serve(http.StatusOK,
				[]byte(`{"results":["family","work","travel"],"page":1,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/user/my-labels")

			res, err := client.Labels(ctx, 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(ConsistOf("family", "work", "travel"))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("MaxFamily", func() {
		It("decodes a paginated profile.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1}`),
				http.MethodGet, "/api/user/max-family")

			res, err := client.MaxFamily(ctx, 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
		})
	})

	Describe("Metadata", func() {
		It("Metadata round-trips an opaque data blob", func() {
			serve(http.StatusOK,
				[]byte(`{"data":{"theme":"dark","sidebar":42}}`),
				http.MethodGet, "/api/user/metadata")

			md, err := client.Metadata(ctx)

			Expect(err).ToNot(HaveOccurred())
			Expect(string(md.Data)).To(ContainSubstring(`"theme":"dark"`))
			Expect(string(md.Data)).To(ContainSubstring(`"sidebar":42`))
		})

		It("Metadata with multiple user ids sets ids=", func() {
			serve(http.StatusOK, []byte(`{"data":{}}`),
				http.MethodGet, "/api/user/metadata")

			_, err := client.Metadata(ctx, "user-1", "user-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("ids")).To(Equal("user-1,user-2"))
		})

		It("UpdateMetadata POSTs data as a JSON-encoded string", func() {
			serve(http.StatusOK,
				[]byte(`{"data":{"theme":"light"}}`),
				http.MethodPost, "/api/user/update-metadata")

			_, err := client.UpdateMetadata(ctx, json.RawMessage(`{"theme":"light"}`))

			Expect(err).ToNot(HaveOccurred())
			Expect(string(reqBody)).To(ContainSubstring(`"data":"{\"theme\":\"light\"}"`))
		})
	})
})
