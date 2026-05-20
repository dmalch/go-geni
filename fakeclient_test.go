package geni

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

// fakeTransport is the shared in-process round-tripper for root-package
// unit tests — it records the last request and replays a canned status
// + body. Kept in root only while the Profile resource still lives
// here; moves into profile/ alongside the methods in the final
// reshape PR.
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

// rewriteTransport directs a Client's requests at a local
// httptest.Server without changing the Client's externally-derived
// BaseURL. Used by the root-package integration tests.
type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return t.base.RoundTrip(clone)
}

func newClientFor(server *httptest.Server) *Client {
	target, err := url.Parse(server.URL)
	Expect(err).ToNot(HaveOccurred())

	c := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "acc-test"}), true)
	c.transport.SetHTTPClient(&http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: target}})
	return c
}
