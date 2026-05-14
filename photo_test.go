package geni

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// readMultipart parses a request body the client built and returns the
// recorded form values + the file part's filename, content type, and
// raw bytes.
func readMultipart(t *testing.T, req *http.Request) (fields map[string]string, fileName string, fileBody []byte) {
	t.Helper()
	ct := req.Header.Get("Content-Type")
	Expect(ct).To(HavePrefix("multipart/form-data;"))

	_, params, err := mime.ParseMediaType(ct)
	Expect(err).ToNot(HaveOccurred())
	boundary, ok := params["boundary"]
	Expect(ok).To(BeTrue())

	mr := multipart.NewReader(req.Body, boundary)
	fields = map[string]string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		Expect(err).ToNot(HaveOccurred())
		buf, err := io.ReadAll(part)
		Expect(err).ToNot(HaveOccurred())
		if part.FileName() != "" {
			fileName = part.FileName()
			fileBody = buf
		} else {
			fields[part.FormName()] = string(buf)
		}
	}
	return
}

func TestCreatePhoto_Request(t *testing.T) {
	t.Run("POSTs multipart/form-data with title and file", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1","title":"Hello"}`)

		photo, err := c.CreatePhoto(
			context.Background(),
			"Hello",
			"hello.jpg",
			bytes.NewReader([]byte("\xff\xd8raw-jpeg-bytes")),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(photo.Id).To(Equal("photo-1"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo/add"))

		fields, fileName, fileBody := readMultipart(t, ft.lastRequest)
		Expect(fields["title"]).To(Equal("Hello"))
		Expect(fileName).To(Equal("hello.jpg"))
		Expect(fileBody).To(Equal([]byte("\xff\xd8raw-jpeg-bytes")))
	})

	t.Run("WithPhotoAlbum / WithPhotoDescription / WithPhotoDate set form fields", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1"}`)

		_, err := c.CreatePhoto(
			context.Background(),
			"Title",
			"img.png",
			strings.NewReader("png-bytes"),
			WithPhotoAlbum("album-7"),
			WithPhotoDescription("a description"),
			WithPhotoDate("2026-05-14"),
		)
		Expect(err).ToNot(HaveOccurred())

		fields, _, _ := readMultipart(t, ft.lastRequest)
		Expect(fields).To(HaveKeyWithValue("title", "Title"))
		Expect(fields).To(HaveKeyWithValue("album_id", "album-7"))
		Expect(fields).To(HaveKeyWithValue("description", "a description"))
		Expect(fields).To(HaveKeyWithValue("date", "2026-05-14"))
	})

	t.Run("empty title returns an error before sending the request", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{}`)

		_, err := c.CreatePhoto(context.Background(), "", "f.png", strings.NewReader("x"))

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("title is required"))
		Expect(ft.lastRequest).To(BeNil(), "no HTTP request should have been sent")
	})

	t.Run("nil file returns an error before sending the request", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{}`)

		_, err := c.CreatePhoto(context.Background(), "Title", "f.png", nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("file is required"))
		Expect(ft.lastRequest).To(BeNil())
	})

	t.Run("Content-Type carries the multipart boundary, not application/json", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1"}`)

		_, err := c.CreatePhoto(context.Background(), "Title", "x.bin", strings.NewReader("x"))
		Expect(err).ToNot(HaveOccurred())

		got := ft.lastRequest.Header.Values("Content-Type")
		Expect(got).To(HaveLen(1))
		Expect(got[0]).To(HavePrefix("multipart/form-data; boundary="))
	})
}

func TestGetPhoto_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"photo-9","title":"X"}`)

	photo, err := c.GetPhoto(context.Background(), "photo-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(photo.Id).To(Equal("photo-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-9"))
}

func TestGetPhotos_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"photo-1"},{"id":"photo-2"}]}`)

	res, err := c.GetPhotos(context.Background(), []string{"photo-1", "photo-2"})

	Expect(err).ToNot(HaveOccurred())
	Expect(res.Results).To(HaveLen(2))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("photo-1,photo-2"))
}

func TestDeletePhoto_Request(t *testing.T) {
	t.Run("POSTs to /api/<photoId>/delete", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"result":"ok"}`)

		err := c.DeletePhoto(context.Background(), "photo-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-9/delete"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		err := c.DeletePhoto(context.Background(), "photo-9")

		Expect(err).To(MatchError(ErrResourceNotFound))
	})
}
