package photoalbum

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni/transport"
)

type fakeTransport struct {
	lastRequest *http.Request
	status      int
	body        string
}

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.lastRequest = req.Clone(req.Context())
	body := t.body
	if body == "" {
		body = "{}"
	}
	return &http.Response{
		StatusCode: t.status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}, nil
}

func newFakeClient(status int, body string) (*Client, *fakeTransport) {
	ft := &fakeTransport{status: status, body: body}
	t := transport.New(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}), true)
	t.SetHTTPClient(&http.Client{Transport: ft})
	return NewClient(t), ft
}

func TestCreate_Request(t *testing.T) {
	t.Run("POSTs JSON body to /api/photo_album/add", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK,
			`{"id":"album-9","name":"Vacation 1972","photos_count":0}`)

		desc := "Family trip"
		album, err := c.Create(context.Background(), &Request{
			Name:        "Vacation 1972",
			Description: &desc,
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(album.ID).To(Equal("album-9"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo_album/add"))
		got, _ := io.ReadAll(ft.lastRequest.Body)
		Expect(string(got)).To(ContainSubstring(`"name":"Vacation 1972"`))
		Expect(string(got)).To(ContainSubstring(`"description":"Family trip"`))
	})

	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.Create(context.Background(), &Request{Name: "X"})
		Expect(err).To(MatchError(transport.ErrAccessDenied))
	})
}

func TestGet_Request(t *testing.T) {
	t.Run("GETs /api/<albumId> and decodes the full PhotoAlbum", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{
			"id": "album-1",
			"name": "Family Reunions",
			"description": "Yearly gatherings",
			"photos_count": 42,
			"cover_photo": {
				"small":  "https://photos.geni.test/cover/small.jpg",
				"medium": "https://photos.geni.test/cover/medium.jpg"
			},
			"url": "https://www.geni.com/api/photo_album-1"
		}`
		c, ft := newFakeClient(http.StatusOK, body)

		album, err := c.Get(context.Background(), "album-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(album.ID).To(Equal("album-1"))
		Expect(album.Name).To(Equal("Family Reunions"))
		Expect(album.PhotosCount).To(Equal(42))
		Expect(album.CoverPhoto).To(HaveKeyWithValue("small", "https://photos.geni.test/cover/small.jpg"))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo_album-1"))
	})

	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.Get(context.Background(), "album-1")
		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestPhotos_Request(t *testing.T) {
	t.Run("GETs /api/<albumId>/photos and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.Photos(context.Background(), "album-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo_album-1/photos"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.Photos(context.Background(), "album-1", 3)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("3"))
	})

	t.Run("decodes paginated photo.BulkResponse", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"results":[{"id":"photo-1"},{"id":"photo-2"}],"page":1,"next_page":"…?page=2"}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.Photos(context.Background(), "album-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})
}

func TestUpdate_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK,
		`{"id":"album-1","name":"After","description":"Renamed"}`)

	desc := "Renamed"
	album, err := c.Update(context.Background(), "album-1", &Request{
		Name:        "After",
		Description: &desc,
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(album.Name).To(Equal("After"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo_album-1/update"))
	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"name":"After"`))
}
