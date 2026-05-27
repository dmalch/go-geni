// Package revision exposes the website-only revision-history endpoint.
// The OAuth API has no equivalent — api/profile-<id>/revisions returns
// 500. See the parent package doc for legal caveats.
package revision

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/internal/htmlparse"
)

// Client wraps a *web.Client with the revision endpoints.
type Client struct {
	web *web.Client
}

// NewClient returns a revision Client backed by the given web Client.
func NewClient(w *web.Client) *Client { return &Client{web: w} }

// ForProfile lists the revision IDs in a profile's edit history, most
// recent first. guid is the profile's globally unique identifier
// (e.g. "6000000218702371879"), not the "profile-NNN" short id.
//
// Cross over to the OAuth API to fetch details for each ID:
//
//	~/go/bin/geni revision get revision-<id>
func (c *Client) ForProfile(ctx context.Context, guid string) ([]string, error) {
	body := url.Values{"id": {guid}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.web.BaseURL()+"/revisions/profile", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.web.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("revisions/profile: HTTP %d", resp.StatusCode)
	}

	return htmlparse.RevisionIDs(resp.Body)
}
