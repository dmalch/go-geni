package geni

import (
	"bytes"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

// fakeTransport is the shared in-process round-tripper for the
// root-package façade tests (bulk-read + coalescing integration
// across resources). It records the last request and replays a
// canned status + body.
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
	c := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}), true)
	c.transport.SetHTTPClient(&http.Client{Transport: ft})
	return c, ft
}
