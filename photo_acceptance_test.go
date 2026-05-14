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
