package tree

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dmalch/go-geni/transport"
)

// Client wraps a transport.Client with the family-graph endpoints.
type Client struct {
	transport *transport.Client
}

// NewClient returns a tree Client backed by the supplied transport.
func NewClient(t *transport.Client) *Client {
	return &Client{transport: t}
}

// ImmediateFamily fetches the one-hop family graph around profileId
// (parents, partners, children, siblings) as a [FamilyResponse]. The
// response's Nodes map is heterogeneous — use [FamilyNodes.Profile]
// and [FamilyNodes.Union] to decode individual entries.
func (c *Client) ImmediateFamily(ctx context.Context, profileId string) (*FamilyResponse, error) {
	return c.getFamily(ctx, "api/"+profileId+"/immediate-family")
}

// Ancestors fetches the ancestor graph rooted at profileId.
// [WithGenerations] controls depth; the Geni-documented maximum is 20
// generations and values above that are clamped client-side.
//
// Observed behavior on the Geni sandbox (test account, 2026-05-14):
//   - 403 (surfaced as [transport.ErrAccessDenied]) for
//     freshly-created profiles, managed profiles, and hand-built
//     parent→child chains.
//   - `me` as the path id returns 500 ("No action responded to me").
//   - Sibling endpoints (ImmediateFamily, PathTo) succeed against the
//     same token on the same profiles.
//
// The public docs do not describe an access rule for this endpoint,
// and Geni publishes no OAuth scope catalog beyond a `read_profile,
// write_profile` example in /platform/developer/help/oauth_intro.
func (c *Client) Ancestors(ctx context.Context, profileId string, opts ...Option) (*FamilyResponse, error) {
	return c.getFamily(ctx, "api/"+profileId+"/ancestors", opts...)
}

func (c *Client) getFamily(ctx context.Context, path string, opts ...Option) (*FamilyResponse, error) {
	url := c.transport.BaseURL() + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	for _, o := range opts {
		o(req)
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var family FamilyResponse
	if err := json.Unmarshal(body, &family); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &family, nil
}

// PathTo fetches the kinship path between fromId and toId. The call is
// asynchronous on Geni's side: a [PathStatusPending] response means
// the server is still computing and the caller should back off and
// re-issue. Geni's path-to also has side effects (email + on-site
// notifications) unless suppressed via [WithSkipEmail] /
// [WithSkipNotify].
func (c *Client) PathTo(ctx context.Context, fromId, toId string, opts ...Option) (*PathToResponse, error) {
	url := c.transport.BaseURL() + "api/" + fromId + "/path-to/" + toId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	for _, o := range opts {
		o(req)
	}

	body, err := c.transport.Do(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	var path PathToResponse
	if err := json.Unmarshal(body, &path); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	return &path, nil
}
