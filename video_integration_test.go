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

var _ = Describe("Client video endpoints", func() {
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

	Describe("CreateVideo", func() {
		It("sends multipart with the file part and decodes the new video", func() {
			var capturedTitle, capturedFileName string
			var capturedFileBody []byte

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/video/add"))

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
					b, err := io.ReadAll(part)
					Expect(err).ToNot(HaveOccurred())
					switch {
					case part.FormName() == "title":
						capturedTitle = string(b)
					case part.FormName() == "file":
						capturedFileName = part.FileName()
						capturedFileBody = b
					}
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"id": "video-42",
					"title": "Reunion 1972",
					"sizes": {"small": "https://videos.geni.test/v-42/small.mp4"},
					"created_at": "2026-05-15T09:00:00Z"
				}`))
			}))
			client = newClientFor(server)

			raw := []byte("mp4-bytes-here")
			video, err := client.CreateVideo(ctx, "Reunion 1972", "reunion.mp4", bytes.NewReader(raw),
				WithVideoDescription("annual family reunion"))

			Expect(err).ToNot(HaveOccurred())
			Expect(video.Id).To(Equal("video-42"))
			Expect(video.Sizes).To(HaveKeyWithValue("small", "https://videos.geni.test/v-42/small.mp4"))
			Expect(capturedTitle).To(Equal("Reunion 1972"))
			Expect(capturedFileName).To(Equal("reunion.mp4"))
			Expect(capturedFileBody).To(Equal(raw))
		})
	})

	Describe("UpdateVideo / TagVideo / GetVideoTags / Comments", func() {
		It("UpdateVideo POSTs JSON and decodes the response", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"video-1","title":"After"}`),
				http.MethodPost, "/api/video-1/update")

			video, err := client.UpdateVideo(ctx, "video-1", &VideoRequest{Title: "After"})

			Expect(err).ToNot(HaveOccurred())
			Expect(video.Title).To(Equal("After"))
		})

		It("TagVideo targets the path-based endpoint", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"video-1","tags":["profile-9"]}`),
				http.MethodPost, "/api/video-1/tag/profile-9")

			video, err := client.TagVideo(ctx, "video-1", "profile-9")

			Expect(err).ToNot(HaveOccurred())
			Expect(video.Tags).To(ConsistOf("profile-9"))
		})

		It("GetVideoTags decodes a paginated profile list", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"profile-1"}],"page":1,"next_page":"…?page=2"}`),
				http.MethodGet, "/api/video-1/tags")

			res, err := client.GetVideoTags(ctx, "video-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(1))
		})

		It("AddVideoComment sends text + optional title as query params", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[{"id":"c-1","comment":"hi"}]}`),
				http.MethodPost, "/api/video-1/comment")

			_, err := client.AddVideoComment(ctx, "video-1", "hi", "")

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("text")).To(Equal("hi"))
			Expect(recorded.URL.Query().Has("title")).To(BeFalse())
		})
	})
})
