package video

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
// recorded form values + the file part's filename and raw bytes.
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
	t.Run("POSTs multipart with title + file", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"video-1","title":"Reunion 1972"}`)

		v, err := c.Create(
			context.Background(),
			"Reunion 1972",
			"reunion.mp4",
			bytes.NewReader([]byte("mp4-bytes")),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(v.Id).To(Equal("video-1"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video/add"))

		fields, fileName, fileBody := readMultipart(t, ft.lastRequest)
		Expect(fields["title"]).To(Equal("Reunion 1972"))
		Expect(fileName).To(Equal("reunion.mp4"))
		Expect(fileBody).To(Equal([]byte("mp4-bytes")))
	})

	t.Run("file is optional — multipart body omits the file part", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"video-1"}`)

		_, err := c.Create(context.Background(), "External link", "", nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))

		fields, fileName, fileBody := readMultipart(t, ft.lastRequest)
		Expect(fields["title"]).To(Equal("External link"))
		Expect(fileName).To(BeEmpty())
		Expect(fileBody).To(BeEmpty())
	})

	t.Run("WithDescription + WithDate are set as form fields", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"video-1"}`)

		_, err := c.Create(
			context.Background(),
			"Title",
			"v.mp4",
			strings.NewReader("bytes"),
			WithDescription("a description"),
			WithDate("2026-05-15"),
		)
		Expect(err).ToNot(HaveOccurred())

		fields, _, _ := readMultipart(t, ft.lastRequest)
		Expect(fields).To(HaveKeyWithValue("description", "a description"))
		Expect(fields).To(HaveKeyWithValue("date", "2026-05-15"))
	})

	t.Run("empty title returns an error before sending the request", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{}`)

		_, err := c.Create(context.Background(), "", "v.mp4", strings.NewReader("x"))

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("title is required"))
		Expect(ft.lastRequest).To(BeNil(), "no HTTP request should have been sent")
	})
}

func TestGet_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-9","title":"X"}`)

	v, err := c.Get(context.Background(), "video-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(v.Id).To(Equal("video-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-9"))
}

func TestGetBulk_SingleIdFallback(t *testing.T) {
	t.Run("1 id → /api/<id>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"video-9","title":"X"}`)

		res, err := c.GetBulk(context.Background(), []string{"video-9"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-9"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Id).To(Equal("video-9"))
	})

	t.Run("2 ids → /api/video?ids=…", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"video-1"},{"id":"video-2"}]}`)

		_, err := c.GetBulk(context.Background(), []string{"video-1", "video-2"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("video-1,video-2"))
	})
}

func TestUpdate_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-1","title":"After"}`)

	v, err := c.Update(context.Background(), "video-1", &Request{
		Title:       "After",
		Description: "updated",
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(v.Title).To(Equal("After"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/update"))

	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"title":"After"`))
	Expect(string(got)).To(ContainSubstring(`"description":"updated"`))
}

func TestDelete_Request(t *testing.T) {
	t.Run("POSTs to /api/<videoId>/delete", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"result":"ok"}`)

		err := c.Delete(context.Background(), "video-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-9/delete"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		err := c.Delete(context.Background(), "video-9")

		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestTag_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-1","tags":["profile-9"]}`)

	_, err := c.Tag(context.Background(), "video-1", "profile-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/tag/profile-9"))
}

func TestUntag_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-1","tags":[]}`)

	_, err := c.Untag(context.Background(), "video-1", "profile-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/untag/profile-9"))
}

func TestTags_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"profile-1"}],"page":1}`)

	res, err := c.Tags(context.Background(), "video-1", 1)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/tags"))
	Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("1"))
	Expect(res.Results).To(HaveLen(1))
}

func TestComments_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"c-1","comment":"hi"}]}`)

	res, err := c.Comments(context.Background(), "video-1", 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/comments"))
	Expect(res.Results).To(HaveLen(1))
}

func TestAddComment_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

	_, err := c.AddComment(context.Background(), "video-1", "nice clip", "")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/comment"))
	Expect(ft.lastRequest.URL.Query().Get("text")).To(Equal("nice clip"))
}

func TestAddToProfile_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-9","title":"Reel"}`)

	v, err := c.AddToProfile(context.Background(), "profile-1", &Request{Title: "Reel"})

	Expect(err).ToNot(HaveOccurred())
	Expect(v.Id).To(Equal("video-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-video"))
	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"title":"Reel"`))
}
