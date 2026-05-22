// Package stats wraps Geni's /stats endpoint — the platform-wide
// statistics list.
package stats

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dmalch/go-geni/transport"
)

// Response is Geni's /stats response — an opaque list of "available
// statistics". The public docs describe the response as an array of
// hashes without enumerating fields, so each entry is kept as a raw
// JSON message; callers decode the specific stats they care about.
type Response struct {
	Stats []json.RawMessage `json:"stats,omitempty"`
}

// Client wraps a transport.Client with the /stats endpoint.
type Client struct {
	transport *transport.Client
}

// NewClient returns a stats Client that uses the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// Get fetches the platform-wide statistics list. The shape of
// individual entries is opaque to the client — see [Response.Stats].
func (c *Client) Get(ctx context.Context) (*Response, error) {
	url := c.transport.BaseURL() + "api/stats"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var s Response
	if err := json.Unmarshal(body, &s); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &s, nil
}
