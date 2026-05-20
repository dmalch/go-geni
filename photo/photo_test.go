package photo

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

func TestCreate_Request(t *testing.T) {
	t.Run("POSTs multipart/form-data with title and file", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1","title":"Hello"}`)

		p, err := c.Create(
			context.Background(),
			"Hello",
			"hello.jpg",
			bytes.NewReader([]byte("\xff\xd8raw-jpeg-bytes")),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(p.Id).To(Equal("photo-1"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo/add"))

		fields, fileName, fileBody := readMultipart(t, ft.lastRequest)
		Expect(fields["title"]).To(Equal("Hello"))
		Expect(fileName).To(Equal("hello.jpg"))
		Expect(fileBody).To(Equal([]byte("\xff\xd8raw-jpeg-bytes")))
	})

	t.Run("WithAlbum / WithDescription / WithDate set form fields", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1"}`)

		_, err := c.Create(
			context.Background(),
			"Title",
			"img.png",
			strings.NewReader("png-bytes"),
			WithAlbum("album-7"),
			WithDescription("a description"),
			WithDate("2026-05-14"),
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

		_, err := c.Create(context.Background(), "", "f.png", strings.NewReader("x"))

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("title is required"))
		Expect(ft.lastRequest).To(BeNil(), "no HTTP request should have been sent")
	})

	t.Run("nil file returns an error before sending the request", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{}`)

		_, err := c.Create(context.Background(), "Title", "f.png", nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("file is required"))
		Expect(ft.lastRequest).To(BeNil())
	})

	t.Run("Content-Type carries the multipart boundary, not application/json", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1"}`)

		_, err := c.Create(context.Background(), "Title", "x.bin", strings.NewReader("x"))
		Expect(err).ToNot(HaveOccurred())

		got := ft.lastRequest.Header.Values("Content-Type")
		Expect(got).To(HaveLen(1))
		Expect(got[0]).To(HavePrefix("multipart/form-data; boundary="))
	})
}

func TestGet_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"photo-9","title":"X"}`)

	p, err := c.Get(context.Background(), "photo-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(p.Id).To(Equal("photo-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-9"))
}

func TestGetBulk_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"photo-1"},{"id":"photo-2"}]}`)

	res, err := c.GetBulk(context.Background(), []string{"photo-1", "photo-2"})

	Expect(err).ToNot(HaveOccurred())
	Expect(res.Results).To(HaveLen(2))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("photo-1,photo-2"))
}

func TestUpdate_Request(t *testing.T) {
	t.Run("POSTs JSON to /api/<photoId>/update", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1","title":"After"}`)

		p, err := c.Update(context.Background(), "photo-1", &Request{
			Title:       "After",
			Description: "updated",
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(p.Title).To(Equal("After"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-1/update"))

		got, err := io.ReadAll(ft.lastRequest.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(got)).To(ContainSubstring(`"title":"After"`))
		Expect(string(got)).To(ContainSubstring(`"description":"updated"`))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.Update(context.Background(), "photo-1", &Request{Title: "X"})

		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestTag_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1","tags":["profile-9"]}`)

	p, err := c.Tag(context.Background(), "photo-1", "profile-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(p.Tags).To(ConsistOf("profile-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-1/tag/profile-9"))
}

func TestUntag_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"photo-1","tags":[]}`)

	_, err := c.Untag(context.Background(), "photo-1", "profile-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-1/untag/profile-9"))
}

func TestTags_Request(t *testing.T) {
	t.Run("GETs /api/<photoId>/tags and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.Tags(context.Background(), "photo-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-1/tags"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.Tags(context.Background(), "photo-1", 2)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("2"))
	})

	t.Run("decodes profile results", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"results":[{"id":"profile-1","first_name":"A"},{"id":"profile-2"}],"page":1}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.Tags(context.Background(), "photo-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.Results[0].Id).To(Equal("profile-1"))
	})
}

func TestComments_Request(t *testing.T) {
	t.Run("GETs /api/<photoId>/comments and decodes Comment results", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"results":[{"id":"c-1","comment":"nice"}],"page":1}`
		c, ft := newFakeClient(http.StatusOK, body)

		res, err := c.Comments(context.Background(), "photo-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-1/comments"))
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Comment).To(Equal("nice"))
	})
}

func TestAddComment_Request(t *testing.T) {
	t.Run("POSTs text and optional title as query params", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.AddComment(context.Background(), "photo-1", "hi there", "greeting")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-1/comment"))
		Expect(ft.lastRequest.URL.Query().Get("text")).To(Equal("hi there"))
		Expect(ft.lastRequest.URL.Query().Get("title")).To(Equal("greeting"))
	})

	t.Run("empty title is omitted from the query", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.AddComment(context.Background(), "photo-1", "hi", "")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Has("title")).To(BeFalse())
	})
}

func TestDelete_Request(t *testing.T) {
	t.Run("POSTs to /api/<photoId>/delete", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"result":"ok"}`)

		err := c.Delete(context.Background(), "photo-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/photo-9/delete"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		err := c.Delete(context.Background(), "photo-9")

		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestForProfile_Request(t *testing.T) {
	t.Run("GETs /api/<profileId>/photos", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.ForProfile(context.Background(), "profile-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/photos"))
	})

	t.Run("decodes results + pagination", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"results":[{"id":"photo-9","title":"Family portrait"}],"page":1,"next_page":"…?page=2"}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.ForProfile(context.Background(), "profile-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Id).To(Equal("photo-9"))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})
}

func TestAddToProfile_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"photo-9","title":"Snapshot"}`)

	b64 := "aGVsbG8="
	res, err := c.AddToProfile(context.Background(), "profile-1", &Request{
		Title: "Snapshot",
		File:  &b64,
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(res.Id).To(Equal("photo-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-photo"))
	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"title":"Snapshot"`))
	Expect(string(got)).To(ContainSubstring(`"file":"aGVsbG8="`))
}

func TestAddMugshotToProfile_Request(t *testing.T) {
	t.Run("File path sets file, omits photo_id", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-9"}`)

		b64 := "aGVsbG8="
		_, err := c.AddMugshotToProfile(context.Background(), "profile-1", &MugshotRequest{File: &b64})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-mugshot"))
		got, _ := io.ReadAll(ft.lastRequest.Body)
		Expect(string(got)).To(ContainSubstring(`"file":"aGVsbG8="`))
		Expect(string(got)).ToNot(ContainSubstring(`"photo_id"`))
	})

	t.Run("PhotoId path sets photo_id, omits file", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-9"}`)

		existing := "photo-100"
		_, err := c.AddMugshotToProfile(context.Background(), "profile-1", &MugshotRequest{PhotoId: &existing})

		Expect(err).ToNot(HaveOccurred())
		got, _ := io.ReadAll(ft.lastRequest.Body)
		Expect(string(got)).To(ContainSubstring(`"photo_id":"photo-100"`))
		Expect(string(got)).ToNot(ContainSubstring(`"file"`))
	})
}
