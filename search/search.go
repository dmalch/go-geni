// Package search wraps Geni's /profile/search endpoint — name-based
// profile discovery.
package search

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the search endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a search Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// Profiles performs a name-based profile search against Geni's
// /profile/search endpoint. names is the free-text query Geni matches
// against profile names (passed to the upstream `names` query
// parameter); pass an empty string to omit it. page is 1-indexed and
// selects which page of results to return — values ≤0 omit the
// parameter (Geni defaults to page 1).
//
// The response is a [profile.BulkResponse]; its Results, Page,
// NextPage, and PrevPage fields describe the current page and how to
// navigate forward/backward.
func (c *Client) Profiles(ctx context.Context, names string, page int) (*profile.BulkResponse, error) {
	url := c.transport.BaseURL() + "api/profile/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	profile.AddFields(req)

	query := req.URL.Query()
	if names != "" {
		query.Set("names", names)
	}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	req.URL.RawQuery = query.Encode()

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var profiles profile.BulkResponse
	if err := json.Unmarshal(body, &profiles); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	apiURL := c.transport.APIURL()
	for i := range profiles.Results {
		profile.StripURLs(&profiles.Results[i], apiURL)
	}
	return &profiles, nil
}
