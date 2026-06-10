// Package unions exposes the website-only "remove relationships"
// action. The OAuth API cannot detach a profile from a union (and has
// no union-delete endpoint), so this package drives the internal
// /profile_actions/delete_relationships web action that the
// edit_relationships page's "Удалить связь" button POSTs to. See the
// parent package doc for legal caveats.
package unions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/dmalch/go-geni/web"
)

// Client wraps a *web.Client with the union-detach endpoint.
type Client struct {
	web *web.Client
}

// NewClient returns a unions Client backed by the given web Client.
func NewClient(w *web.Client) *Client { return &Client{web: w} }

// errCSRFRetry signals a stale authenticity_token (HTTP 422). The
// caller invalidates the cached CSRF, refetches, and retries once.
var errCSRFRetry = errors.New("csrf retry needed")

// Detach removes profileID's connection to each union in unionIDs — the
// "Удалить связь" / "remove relationships" action on the
// edit_relationships page. profileID and the unionIDs are Geni web ids
// (the bare 6000000… numbers, as in each remove_connection_<id>
// checkbox). Detaching empties the union of this profile; an orphaned
// union is harmless but is not itself deleted.
//
// On a 422 (a stale authenticity_token) the cached CSRF is invalidated,
// refetched, and the POST retried once, mirroring matches.Reject.
func (c *Client) Detach(ctx context.Context, profileID string, unionIDs []string) error {
	err := c.postDetach(ctx, profileID, unionIDs)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errCSRFRetry) {
		return err
	}
	c.web.InvalidateCSRF()
	return c.postDetach(ctx, profileID, unionIDs)
}

func (c *Client) postDetach(ctx context.Context, profileID string, unionIDs []string) error {
	token, err := c.web.CSRFToken(ctx)
	if err != nil {
		return fmt.Errorf("get csrf: %w", err)
	}
	u := c.web.BaseURL() + "/profile_actions/delete_relationships?" +
		url.Values{"id": {profileID}, "uids": {strings.Join(unionIDs, ",")}}.Encode()
	form := url.Values{"authenticity_token": {token}}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.web.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// The page's Geni.Updater POST replies with an HTML fragment (the
	// refreshed cycle list) on success. A 422 means the token went
	// stale — signal a single retry. Treat any other 2xx/3xx as success.
	if resp.StatusCode == http.StatusUnprocessableEntity {
		_, _ = io.Copy(io.Discard, resp.Body)
		return errCSRFRetry
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("delete_relationships: HTTP %d", resp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
