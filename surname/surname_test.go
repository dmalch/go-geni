package surname

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

func TestGet_Request(t *testing.T) {
	t.Run("GETs /api/<surnameId>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"surname-1","slugged_name":"smith","description":"Smith family"}`)

		s, err := c.Get(context.Background(), "surname-1")

		Expect(err).ToNot(HaveOccurred())
		Expect(s.ID).To(Equal("surname-1"))
		Expect(s.SluggedName).To(Equal("smith"))
		Expect(s.Description).To(Equal("Smith family"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/surname-1"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.Get(context.Background(), "surname-1")

		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestFollowers_Request(t *testing.T) {
	t.Run("GETs /api/<surnameId>/followers", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.Followers(context.Background(), "surname-1", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/surname-1/followers"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.Followers(context.Background(), "surname-1", 2)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("2"))
	})

	t.Run("decodes profile results", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{"results":[{"id":"profile-1"}],"page":1,"next_page":"…?page=2"}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.Followers(context.Background(), "surname-1", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})
}

func TestProfiles_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

	_, err := c.Profiles(context.Background(), "surname-1", 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/surname-1/profiles"))
}
