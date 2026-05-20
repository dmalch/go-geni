package revision

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
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"revision-101","action":"create","timestamp":"2026-05-15T09:00:00Z"}`)

	r, err := c.Get(context.Background(), "revision-101")

	Expect(err).ToNot(HaveOccurred())
	Expect(r.ID).To(Equal("revision-101"))
	Expect(r.Action).To(Equal("create"))
	Expect(r.Timestamp).To(Equal("2026-05-15T09:00:00Z"))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/revision-101"))
}

func TestGetBulk_SingleIdFallback(t *testing.T) {
	t.Run("1 id → /api/<id>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"revision-101","action":"create"}`)

		res, err := c.GetBulk(context.Background(), []string{"revision-101"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/revision-101"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].ID).To(Equal("revision-101"))
	})

	t.Run("2 ids → /api/revision?ids=…", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"revision-101"},{"id":"revision-102"}]}`)

		_, err := c.GetBulk(context.Background(), []string{"revision-101", "revision-102"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/revision"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("revision-101,revision-102"))
	})
}

func TestGetBulk_ThreeIds(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[
		{"id":"revision-1","action":"create"},
		{"id":"revision-2","action":"update"},
		{"id":"revision-3","action":"delete"}
	]}`)

	res, err := c.GetBulk(context.Background(), []string{"revision-1", "revision-2", "revision-3"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/revision"))
	Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("revision-1,revision-2,revision-3"))
	Expect(res.Results).To(HaveLen(3))
	actions := []string{res.Results[0].Action, res.Results[1].Action, res.Results[2].Action}
	Expect(actions).To(ConsistOf("create", "update", "delete"))
}

func TestGetBulk_ErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.GetBulk(context.Background(), []string{"revision-1", "revision-2"})
		Expect(err).To(MatchError(transport.ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.GetBulk(context.Background(), []string{"revision-1", "revision-2"})
		Expect(err).To(MatchError(transport.ErrAccessDenied))
	})
}
