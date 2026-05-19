package geni

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni/transport"
)

// ErrResourceNotFound is returned for 404 responses from the Geni API.
// Re-exported from the transport package so existing callers
// (errors.Is(err, ErrResourceNotFound)) keep working unchanged.
var ErrResourceNotFound = transport.ErrResourceNotFound

// ErrAccessDenied is returned for 403 responses from the Geni API.
// Re-exported from the transport package.
var ErrAccessDenied = transport.ErrAccessDenied

// Client is the Geni API client. All endpoint methods hang off this
// type. The HTTP plumbing (auth, rate limiting, retry, bulk-read
// coalescing) lives in the transport package.
type Client struct {
	useSandboxEnv bool
	transport     *transport.Client
}

// NewClient constructs a Client. useSandboxEnv selects between
// sandbox.geni.com (true) and www.geni.com (false).
func NewClient(tokenSource oauth2.TokenSource, useSandboxEnv bool) *Client {
	return &Client{
		useSandboxEnv: useSandboxEnv,
		transport:     transport.New(tokenSource),
	}
}

// BaseURL returns the prod or sandbox HTTP host (with trailing slash).
func BaseURL(useSandboxEnv bool) string {
	return transport.BaseURL(useSandboxEnv)
}

// apiUrl returns the prod or sandbox API host (with "api/" suffix and
// trailing slash). Used when stripping URL prefixes from response
// bodies that ignored only_ids=true — e.g. ProfileResponse.Unions.
func apiUrl(useSandboxEnv bool) string {
	return transport.APIURL(useSandboxEnv)
}

// doRequest forwards to the transport layer. Passing a Coalescer
// opts the request into bulk-read coalescing; omit it for plain
// requests.
func (c *Client) doRequest(ctx context.Context, req *http.Request, coalescer ...transport.Coalescer) ([]byte, error) {
	var co transport.Coalescer
	if len(coalescer) > 0 {
		co = coalescer[0]
	}
	return c.transport.Do(ctx, req, co)
}
