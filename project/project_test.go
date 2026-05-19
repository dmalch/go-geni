package project

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

func TestProfiles_Request(t *testing.T) {
	t.Run("GETs /api/<projectId>/profiles and omits page by default", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.Profiles(context.Background(), "project-7", 0)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/project-7/profiles"))
		Expect(ft.lastRequest.URL.Query().Has("page")).To(BeFalse())
	})

	t.Run("positive page sets the page query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

		_, err := c.Profiles(context.Background(), "project-7", 3)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("3"))
	})

	t.Run("decodes results + pagination + total_count", func(t *testing.T) {
		RegisterTestingT(t)
		body := `{
			"results": [
				{"id":"profile-1","first_name":"A"},
				{"id":"profile-2","first_name":"B"}
			],
			"page": 1,
			"total_count": 42,
			"next_page": "https://www.geni.com/api/project-7/profiles?page=2"
		}`
		c, _ := newFakeClient(http.StatusOK, body)

		res, err := c.Profiles(context.Background(), "project-7", 1)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Results).To(HaveLen(2))
		Expect(res.Page).To(Equal(1))
		Expect(res.TotalCount).To(Equal(42))
		Expect(res.NextPage).To(ContainSubstring("page=2"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		_, err := c.Profiles(context.Background(), "project-7", 0)

		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
}

func TestCollaborators_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

	_, err := c.Collaborators(context.Background(), "project-7", 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/project-7/collaborators"))
}

func TestFollowers_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

	_, err := c.Followers(context.Background(), "project-7", 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/project-7/followers"))
}
