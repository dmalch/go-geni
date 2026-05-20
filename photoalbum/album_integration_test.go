package photoalbum

import (
	"context"
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

var _ = Describe("Photo album endpoints", func() {
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

	Describe("Create", func() {
		It("POSTs the JSON body to /photo_album/add and decodes the new album", func() {
			serve(http.StatusOK,
				[]byte(`{
					"id": "album-9",
					"name": "Vacation 1972",
					"description": "Family trip",
					"photos_count": 0,
					"cover_photo": {"small": "https://photos.geni.test/cover/small.jpg"}
				}`),
				http.MethodPost, "/api/photo_album/add")

			desc := "Family trip"
			album, err := client.Create(ctx, &Request{
				Name:        "Vacation 1972",
				Description: &desc,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(album.ID).To(Equal("album-9"))
			Expect(album.CoverPhoto).To(HaveKeyWithValue("small", "https://photos.geni.test/cover/small.jpg"))
			Expect(string(reqBody)).To(ContainSubstring(`"name":"Vacation 1972"`))
			Expect(string(reqBody)).To(ContainSubstring(`"description":"Family trip"`))
		})
	})

	Describe("Get", func() {
		It("GETs /api/<albumId> and decodes the full PhotoAlbum", func() {
			serve(http.StatusOK,
				[]byte(`{
					"id": "album-1",
					"name": "Family",
					"photos_count": 12,
					"cover_photo": {
						"thumbnail": "https://photos.geni.test/cover/thumb.jpg",
						"large":     "https://photos.geni.test/cover/large.jpg"
					}
				}`),
				http.MethodGet, "/api/photo_album-1")

			album, err := client.Get(ctx, "album-1")

			Expect(err).ToNot(HaveOccurred())
			Expect(album.ID).To(Equal("album-1"))
			Expect(album.PhotosCount).To(Equal(12))
			Expect(album.CoverPhoto).To(HaveLen(2))
		})
	})

	Describe("Photos", func() {
		It("decodes a paginated photo.BulkResponse", func() {
			serve(http.StatusOK,
				[]byte(`{
					"results": [
						{"id":"photo-1","title":"A"},
						{"id":"photo-2","title":"B"}
					],
					"page": 1,
					"next_page": "…?page=2"
				}`),
				http.MethodGet, "/api/photo_album-1/photos")

			res, err := client.Photos(ctx, "album-1", 1)

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("page")).To(Equal("1"))
			Expect(res.Results).To(HaveLen(2))
			Expect(res.NextPage).To(ContainSubstring("page=2"))
		})
	})

	Describe("Update", func() {
		It("POSTs the JSON body to /api/<albumId>/update and decodes the updated album", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"album-1","name":"After"}`),
				http.MethodPost, "/api/photo_album-1/update")

			album, err := client.Update(ctx, "album-1", &Request{
				Name: "After",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(album.Name).To(Equal("After"))
			Expect(string(reqBody)).To(ContainSubstring(`"name":"After"`))
		})
	})
})
