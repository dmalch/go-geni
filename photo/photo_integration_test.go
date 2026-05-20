package photo

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
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

var _ = Describe("Photo CRUD endpoints", func() {
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

	Describe("Update", func() {
		It("POSTs the JSON body and decodes the updated photo", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-1","title":"After","description":"updated"}`),
				http.MethodPost, "/api/photo-1/update")

			p, err := client.Update(ctx, "photo-1", &Request{
				Title:       "After",
				Description: "updated",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(p.Title).To(Equal("After"))
			Expect(p.Description).To(Equal("updated"))
		})
	})

	Describe("Tag / Untag", func() {
		It("targets the path-based tag endpoint and surfaces the updated tags", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-1","tags":["profile-9"]}`),
				http.MethodPost, "/api/photo-1/tag/profile-9")

			p, err := client.Tag(ctx, "photo-1", "profile-9")

			Expect(err).ToNot(HaveOccurred())
			Expect(p.Tags).To(ConsistOf("profile-9"))
		})

		It("targets the path-based untag endpoint", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-1","tags":[]}`),
				http.MethodPost, "/api/photo-1/untag/profile-9")

			p, err := client.Untag(ctx, "photo-1", "profile-9")

			Expect(err).ToNot(HaveOccurred())
			Expect(p.Tags).To(BeEmpty())
		})
	})

	Describe("Tags", func() {
		It("decodes a paginated profile list", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/photo-1/tags")

			res, err := client.Tags(ctx, "photo-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("AddComment + Comments", func() {
		It("posts a comment and lists comments", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"c-1","comment":"hi"}],"page":1}`),
				http.MethodPost, "/api/photo-1/comment")

			res, err := client.AddComment(ctx, "photo-1", "hi", "")

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("text")).To(Equal("hi"))
			Expect(res.Results).To(HaveLen(1))
			Expect(res.Results[0].Comment).To(Equal("hi"))
		})

		It("Comments decodes the comment.BulkResponse envelope", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"c-1","comment":"hi"}],"page":1}`),
				http.MethodGet, "/api/photo-1/comments")

			res, err := client.Comments(ctx, "photo-1", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})
	})
})

var _ = Describe("Photo Create end-to-end", func() {
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

	It("sends multipart/form-data with the file part + form fields and decodes the photo response", func() {
		var capturedTitle, capturedFileName string
		var capturedFileBody []byte

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()

			Expect(r.Method).To(Equal(http.MethodPost))
			Expect(r.URL.Path).To(Equal("/api/photo/add"))

			ct := r.Header.Get("Content-Type")
			Expect(ct).To(HavePrefix("multipart/form-data;"))
			_, params, err := mime.ParseMediaType(ct)
			Expect(err).ToNot(HaveOccurred())

			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				Expect(err).ToNot(HaveOccurred())
				body, err := io.ReadAll(part)
				Expect(err).ToNot(HaveOccurred())
				switch {
				case part.FormName() == "title":
					capturedTitle = string(body)
				case part.FormName() == "file":
					capturedFileName = part.FileName()
					capturedFileBody = body
				}
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "photo-42",
				"title": "Family portrait",
				"album_id": "album-7",
				"sizes": {
					"small":  "https://photos.geni.test/photo-42/small.jpg",
					"medium": "https://photos.geni.test/photo-42/medium.jpg"
				},
				"created_at": "2026-05-14T19:00:00Z"
			}`))
		}))
		client = newClientFor(server)

		raw := []byte("\xff\xd8tiny-jpeg-bytes")
		p, err := client.Create(ctx, "Family portrait", "family.jpg", bytes.NewReader(raw),
			WithAlbum("album-7"))

		Expect(err).ToNot(HaveOccurred())
		Expect(p.Id).To(Equal("photo-42"))
		Expect(p.Title).To(Equal("Family portrait"))
		Expect(p.AlbumId).To(Equal("album-7"))
		Expect(p.Sizes).To(HaveKeyWithValue("small", "https://photos.geni.test/photo-42/small.jpg"))

		Expect(capturedTitle).To(Equal("Family portrait"))
		Expect(capturedFileName).To(Equal("family.jpg"))
		Expect(capturedFileBody).To(Equal(raw))
	})
})

var _ = Describe("Photo profile-scoped endpoints", func() {
	var (
		ctx      context.Context
		server   *httptest.Server
		client   *Client
		recorded *http.Request
	)

	BeforeEach(func() { ctx = context.Background(); recorded = nil })
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

	Describe("ForProfile", func() {
		It("decodes the photo bulk envelope with pagination links", func() {
			serve(http.StatusOK, []byte(`{
				"results":[{"id":"photo-100","title":"Family portrait","sizes":{"small":"https://x/small.jpg"}}],
				"page":1,"next_page":"…?page=2"
			}`), http.MethodGet, "/api/profile-1/photos")

			res, err := client.ForProfile(ctx, "profile-1", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
			Expect(res.Results[0].Id).To(Equal("photo-100"))
			Expect(res.Results[0].Sizes).To(HaveKeyWithValue("small", "https://x/small.jpg"))
		})
	})

	Describe("AddToProfile", func() {
		It("POSTs a JSON body with the Base64 file to /add-photo", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-9","title":"Snapshot"}`),
				http.MethodPost, "/api/profile-1/add-photo")

			b64 := "aGVsbG8="
			p, err := client.AddToProfile(ctx, "profile-1", &Request{Title: "Snapshot", File: &b64})

			Expect(err).ToNot(HaveOccurred())
			Expect(p.Id).To(Equal("photo-9"))
			Expect(recorded.Header.Get("Content-Type")).To(HavePrefix("application/json"))
		})
	})

	Describe("AddMugshotToProfile", func() {
		It("POSTs the mugshot body to /add-mugshot", func() {
			serve(http.StatusOK, []byte(`{"id":"photo-9"}`),
				http.MethodPost, "/api/profile-1/add-mugshot")

			existing := "photo-100"
			_, err := client.AddMugshotToProfile(ctx, "profile-1", &MugshotRequest{PhotoId: &existing})

			Expect(err).ToNot(HaveOccurred())
		})
	})
})
