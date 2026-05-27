// Package web is a Go client for Geni.com's internal AJAX endpoints —
// the same calls the geni.com website makes from a logged-in browser.
//
// These endpoints are undocumented, unsupported by Geni.com, and may
// change or break without notice. Using this package may violate
// geni.com's Terms of Service — review them before use. The package
// is intended for personal interop with your own genealogy data
// (e.g. migration tooling) and does not bypass authentication: it
// requires cookies from a logged-in browser session you established
// yourself.
//
// The package is structurally independent of the OAuth client in the
// repository's root and resource sub-packages. It uses cookie auth
// and per-form CSRF tokens, returns parsed Go values (never HTML),
// and ships with a conservative 1 req/sec default rate limit.
package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"github.com/dmalch/go-geni/web/internal/htmlparse"
)

const (
	defaultBaseURL        = "https://www.geni.com"
	defaultUserAgent      = "go-geni-web/1 (+https://github.com/dmalch/go-geni)"
	defaultCSRFSourcePath = "/documents/save_document_content"
	defaultRateLimit      = 1.0
)

// Client is the AJAX Web client. Construct it once per session;
// resource sub-packages (web/revision, web/document) wrap *Client.
type Client struct {
	httpClient     *http.Client
	baseURL        string
	userAgent      string
	limiter        *rate.Limiter
	csrfSourcePath string

	csrfMu sync.Mutex
	csrf   string
}

// NewClient returns a Client configured from opts.
func NewClient(opts Options) (*Client, error) {
	if opts.Cookies == nil {
		return nil, ErrNoCookies
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	baseU, err := url.Parse(baseURL + "/")
	if err != nil {
		return nil, err
	}
	jar.SetCookies(baseU, opts.Cookies)

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	httpClient.Jar = jar
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	rps := opts.RateLimit
	if rps <= 0 {
		rps = defaultRateLimit
	}
	csrfPath := opts.CSRFSourcePath
	if csrfPath == "" {
		csrfPath = defaultCSRFSourcePath
	}

	return &Client{
		httpClient:     httpClient,
		baseURL:        baseURL,
		userAgent:      ua,
		limiter:        rate.NewLimiter(rate.Limit(rps), 1),
		csrfSourcePath: csrfPath,
	}, nil
}

// BaseURL returns the configured base URL (no trailing slash).
func (c *Client) BaseURL() string { return c.baseURL }

// Do sends req. It enforces the rate limit, attaches the User-Agent,
// follows redirects only inside geni.com (a redirect to /login becomes
// ErrNotLoggedIn), and detects Incapsula block pages.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if err := c.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if isLoginRedirect(resp) {
		_ = resp.Body.Close()
		return nil, ErrNotLoggedIn
	}
	if isIncapsulaBlock(resp) {
		_ = resp.Body.Close()
		return nil, ErrBlocked
	}
	return resp, nil
}

// CSRFToken returns the current authenticity_token, fetching and
// caching it on first use. Safe for concurrent use.
func (c *Client) CSRFToken(ctx context.Context) (string, error) {
	c.csrfMu.Lock()
	if c.csrf != "" {
		tok := c.csrf
		c.csrfMu.Unlock()
		return tok, nil
	}
	c.csrfMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+c.csrfSourcePath, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch CSRF source: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	tok, err := htmlparse.AuthenticityToken(resp.Body)
	if err != nil {
		return "", fmt.Errorf("scrape authenticity_token: %w", err)
	}
	c.csrfMu.Lock()
	c.csrf = tok
	c.csrfMu.Unlock()
	return tok, nil
}

// InvalidateCSRF clears the cached authenticity_token so the next
// CSRFToken call refetches it.
func (c *Client) InvalidateCSRF() {
	c.csrfMu.Lock()
	c.csrf = ""
	c.csrfMu.Unlock()
}

func isLoginRedirect(resp *http.Response) bool {
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return false
	}
	loc, err := resp.Location()
	if err != nil {
		return false
	}
	return strings.HasPrefix(loc.Path, "/login") || strings.HasPrefix(loc.Path, "/signin")
}

func isIncapsulaBlock(resp *http.Response) bool {
	if resp.Header.Get("X-Iinfo") == "" && resp.Header.Get("X-Cdn") == "" {
		return false
	}
	// Peek a small prefix without consuming the whole body.
	const peek = 4096
	buf := make([]byte, peek)
	n, _ := io.ReadFull(resp.Body, buf)
	prefix := string(buf[:n])
	// Restore the body so callers can re-read it. Order matters: prefix
	// first, then the remainder of the original stream.
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(strings.NewReader(prefix), resp.Body), resp.Body}
	return strings.Contains(prefix, "Incapsula")
}
