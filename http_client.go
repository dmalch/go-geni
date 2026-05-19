package geni

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni/stats"
	"github.com/dmalch/go-geni/surname"
	"github.com/dmalch/go-geni/transport"
)

// ErrResourceNotFound is returned for 404 responses from the Geni API.
// Re-exported from the transport package so existing callers
// (errors.Is(err, ErrResourceNotFound)) keep working unchanged.
var ErrResourceNotFound = transport.ErrResourceNotFound

// ErrAccessDenied is returned for 403 responses from the Geni API.
// Re-exported from the transport package.
var ErrAccessDenied = transport.ErrAccessDenied

// Client is the Geni API client. Most endpoint methods hang off this
// type for now; over the pre-1.0 reshape each resource lifts into its
// own sub-package and is exposed through an accessor on Client (e.g.
// [Client.Stats] returns a [stats.Client]). The HTTP plumbing (auth,
// rate limiting, retry, bulk-read coalescing) lives in the transport
// package.
type Client struct {
	useSandboxEnv bool
	transport     *transport.Client
	stats         *stats.Client
	surname       *surname.Client
}

// NewClient constructs a Client. useSandboxEnv selects between
// sandbox.geni.com (true) and www.geni.com (false).
func NewClient(tokenSource oauth2.TokenSource, useSandboxEnv bool) *Client {
	t := transport.New(tokenSource, useSandboxEnv)
	return &Client{
		useSandboxEnv: useSandboxEnv,
		transport:     t,
		stats:         stats.NewClient(t),
		surname:       surname.NewClient(t),
	}
}

// Stats returns the resource client for the platform-wide statistics
// endpoint. Replaces the legacy Client.GetStats method.
func (c *Client) Stats() *stats.Client { return c.stats }

// Surname returns the resource client for the Surname resource.
// Replaces the legacy Client.GetSurname / GetSurnameFollowers /
// GetSurnameProfiles methods.
func (c *Client) Surname() *surname.Client { return c.surname }

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
