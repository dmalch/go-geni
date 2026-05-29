// Package document exposes the website-only document text-body
// endpoints. The OAuth API can neither read nor update a document's
// text body — see the parent package doc for legal caveats.
package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/internal/htmlparse"
)

// Client wraps a *web.Client with the document text endpoints.
type Client struct {
	web *web.Client
}

// NewClient returns a document Client backed by the given web Client.
func NewClient(w *web.Client) *Client { return &Client{web: w} }

// GetText fetches the text body of a document. guid is the document's
// globally unique identifier (e.g. "6000000222741066971"), not the
// "document-NNN" short id — resolve it via the OAuth API if needed.
func (c *Client) GetText(ctx context.Context, guid string) (string, error) {
	u := c.web.BaseURL() + "/documents/view?" + url.Values{"doc_id": {guid}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.web.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("documents/view: HTTP %d", resp.StatusCode)
	}
	return htmlparse.TextareaContent(resp.Body, "document[content]")
}

// SaveText replaces the text body of a document. The call is
// idempotent — POSTing the existing text is a no-op. On a 422 from
// the server (typically a stale authenticity_token) the client
// invalidates the cached CSRF, refetches, and retries once.
func (c *Client) SaveText(ctx context.Context, guid, body string) error {
	err := c.postSave(ctx, guid, body)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errCSRFRetry) {
		return err
	}
	c.web.InvalidateCSRF()
	return c.postSave(ctx, guid, body)
}

var errCSRFRetry = fmt.Errorf("csrf retry needed")

func (c *Client) postSave(ctx context.Context, guid, body string) error {
	token, err := c.web.CSRFToken(ctx)
	if err != nil {
		return fmt.Errorf("get csrf: %w", err)
	}
	form := url.Values{
		"authenticity_token": {token},
		"document[id]":       {guid},
		"document[content]":  {body},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.web.BaseURL()+"/documents/save_document_content", strings.NewReader(form))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.web.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnprocessableEntity {
		_, _ = io.Copy(io.Discard, resp.Body)
		return errCSRFRetry
	}
	// Geni's Rails app responds to a successful save with a 302 redirect
	// back to /documents (the Rails post-redirect-get pattern). Treat any
	// 2xx or 3xx as success.
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("save_document_content: HTTP %d", resp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
