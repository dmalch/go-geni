package geni

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client photo CRUD endpoints", func() {
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

	Describe("UpdatePhoto", func() {
		It("POSTs the JSON body and decodes the updated photo", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-1","title":"After","description":"updated"}`),
				http.MethodPost, "/api/photo-1/update")

			photo, err := client.UpdatePhoto(ctx, "photo-1", &PhotoRequest{
				Title:       "After",
				Description: "updated",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(photo.Title).To(Equal("After"))
			Expect(photo.Description).To(Equal("updated"))
		})
	})

	Describe("TagPhoto / UntagPhoto", func() {
		It("targets the path-based tag endpoint and surfaces the updated tags", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-1","tags":["profile-9"]}`),
				http.MethodPost, "/api/photo-1/tag/profile-9")

			photo, err := client.TagPhoto(ctx, "photo-1", "profile-9")

			Expect(err).ToNot(HaveOccurred())
			Expect(photo.Tags).To(ConsistOf("profile-9"))
		})

		It("targets the path-based untag endpoint", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-1","tags":[]}`),
				http.MethodPost, "/api/photo-1/untag/profile-9")

			photo, err := client.UntagPhoto(ctx, "photo-1", "profile-9")

			Expect(err).ToNot(HaveOccurred())
			Expect(photo.Tags).To(BeEmpty())
		})
	})

	Describe("GetPhotoTags", func() {
		It("decodes a paginated profile list", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"},{"id":"profile-2"}],"page":1,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/photo-1/tags")

			res, err := client.GetPhotoTags(ctx, "photo-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("AddPhotoComment + GetPhotoComments", func() {
		It("posts a comment and lists comments", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"c-1","comment":"hi"}],"page":1}`),
				http.MethodPost, "/api/photo-1/comment")

			res, err := client.AddPhotoComment(ctx, "photo-1", "hi", "")

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("text")).To(Equal("hi"))
			Expect(res.Results).To(HaveLen(1))
			Expect(res.Results[0].Comment).To(Equal("hi"))
		})

		It("GetPhotoComments decodes the CommentBulkResponse envelope", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"c-1","comment":"hi"}],"page":1}`),
				http.MethodGet, "/api/photo-1/comments")

			res, err := client.GetPhotoComments(ctx, "photo-1", 0)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(1))
		})
	})
})

var _ = Describe("Client.CreatePhoto end-to-end", func() {
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
		photo, err := client.CreatePhoto(ctx, "Family portrait", "family.jpg", bytes.NewReader(raw),
			WithPhotoAlbum("album-7"))

		Expect(err).ToNot(HaveOccurred())
		Expect(photo.Id).To(Equal("photo-42"))
		Expect(photo.Title).To(Equal("Family portrait"))
		Expect(photo.AlbumId).To(Equal("album-7"))
		Expect(photo.Sizes).To(HaveKeyWithValue("small", "https://photos.geni.test/photo-42/small.jpg"))

		Expect(capturedTitle).To(Equal("Family portrait"))
		Expect(capturedFileName).To(Equal("family.jpg"))
		Expect(capturedFileBody).To(Equal(raw))
	})
})
