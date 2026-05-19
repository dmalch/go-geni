package stats

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
	t.Run("GETs /api/stats", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"stats":[{"name":"total_profiles","value":1234567}]}`)

		res, err := c.Get(context.Background())

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Stats).To(HaveLen(1))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/stats"))
	})

	t.Run("empty stats array decodes cleanly", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusOK, `{"stats":[]}`)

		res, err := c.Get(context.Background())

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Stats).To(BeEmpty())
	})
}
