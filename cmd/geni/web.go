package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dmalch/go-geni/web"
	"github.com/dmalch/go-geni/web/browsercookies"
)

const webConsentDisclaimer = `WARNING: this command uses Geni.com's private AJAX endpoints (not the
official OAuth API). These endpoints are undocumented, unsupported by
Geni.com, and may break without notice. Using them may violate
geni.com's Terms of Service.

Only proceed if you are operating on your own genealogy data using a
browser session you established yourself.

This consent is one-time. To revoke it later, delete
~/.genealogy/web_consent.json.

Accept and continue? [y/N]: `

// webBaseURL returns the geni.com host the web/AJAX commands target,
// honoring the global -sandbox flag (and GENI_USE_SANDBOX) the same way
// the OAuth client does. The web package defaults to www.geni.com when
// BaseURL is empty, so this only diverges for sandbox.
func webBaseURL(g *globalOpts) string {
	if g != nil && g.sandbox {
		return "https://sandbox.geni.com"
	}
	return "https://www.geni.com"
}

// newWebClient builds a *web.Client for the AJAX commands with the base
// URL pinned to the active environment (production or sandbox).
func newWebClient(g *globalOpts, cookies []*http.Cookie) (*web.Client, error) {
	return web.NewClient(web.Options{Cookies: cookies, BaseURL: webBaseURL(g)})
}

// browserCookieFetcher is the source for cookies when GENI_WEB_COOKIES
// is unset. Indirected so tests can stub it without touching the host's
// real browser stores. The variadic argument forwards to
// browsercookies.FromGeniCom: empty = default browser priority, one or
// more names = only those backends in that order.
var browserCookieFetcher = browsercookies.FromGeniCom

// webConsentFilePath returns the path of the one-time AJAX-consent
// marker file. Lives alongside the OAuth token cache.
func webConsentFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".genealogy", "web_consent.json"), nil
}

// ensureWebConsent gates first AJAX use behind an explicit, one-time
// y/N prompt. Acceptance is persisted to ~/.genealogy/web_consent.json
// so future invocations skip the prompt. GENI_WEB_CONSENT=accepted
// bypasses the prompt without persisting (per-invocation).
func ensureWebConsent(g *globalOpts) error {
	if os.Getenv("GENI_WEB_CONSENT") == "accepted" {
		return nil
	}
	p, err := webConsentFilePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	_, _ = fmt.Fprint(g.stderr, webConsentDisclaimer)
	if !confirmed(g.stdin) {
		return errors.New("AJAX consent declined")
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(p), err)
	}
	body, err := json.Marshal(struct {
		AcceptedAt string `json:"accepted_at"`
		Version    int    `json:"version"`
	}{AcceptedAt: time.Now().UTC().Format(time.RFC3339), Version: 1})
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, body, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}

// cookieFileEnv names a file holding the Cookie header, as an
// alternative to GENI_WEB_COOKIES. A session cookie in an environment
// variable sits in the shell history and in the process environment,
// where `ps -E` and a crash dump can read it; a 0600 file does not.
// It is the only route left on a Safari-only macOS host, whose cookie
// store no process but Safari may read.
const cookieFileEnv = "GENI_WEB_COOKIES_FILE"

// loadWebCookies returns cookies for web.Options.Cookies, preferring an
// explicit GENI_WEB_COOKIES env var, then GENI_WEB_COOKIES_FILE, and
// falling back to the host's browser stores. When g.browser is set,
// only that backend is read; otherwise every browser is tried in
// sweetcookie's default order.
func loadWebCookies(g *globalOpts) ([]*http.Cookie, error) {
	if header := os.Getenv("GENI_WEB_COOKIES"); header != "" {
		return web.CookiesFromHeader(header), nil
	}
	if path := os.Getenv(cookieFileEnv); path != "" {
		header, err := readCookieFile(path)
		if err != nil {
			return nil, err
		}
		return web.CookiesFromHeader(header), nil
	}
	var browsers []string
	if g != nil && g.browser != "" {
		browsers = []string{g.browser}
	}
	cookies, err := browserCookieFetcher(browsers...)
	if err != nil {
		// A diagnosed failure already says why and how to fix it.
		// Wrapping it in "no cookies in any browser" would contradict
		// it — the store is unreadable, not empty — and repeat the hint.
		if errors.Is(err, browsercookies.ErrSafariCookiesUnreadable) ||
			errors.Is(err, browsercookies.ErrFullDiskAccessRequired) {
			return nil, err
		}
		return nil, fmt.Errorf("could not read geni.com cookies from any browser "+
			"(set %s to a file holding the Cookie header from a logged-in browser, "+
			"or GENI_WEB_COOKIES to the header itself): %w", cookieFileEnv, err)
	}
	return cookies, nil
}

// readCookieFile reads a Cookie header from disk. A pointed-at file
// that cannot be read is an error rather than a fall-through to the
// browser stores: the caller named the file, and silently reading
// somewhere else would hide a typo behind a different failure. The
// trailing newline `pbpaste > file` leaves is trimmed — kept, it would
// ride along inside the last cookie's value.
func readCookieFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: read %s: %w", cookieFileEnv, path, err)
	}
	header := strings.TrimSpace(string(body))
	if header == "" {
		return "", fmt.Errorf("%s: %s is empty", cookieFileEnv, path)
	}
	return header, nil
}
