package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

// loadWebCookies returns cookies for web.Options.Cookies, preferring an
// explicit GENI_WEB_COOKIES env var and falling back to the host's
// browser stores. When g.browser is set, only that backend is read;
// otherwise every browser is tried in sweetcookie's default order.
func loadWebCookies(g *globalOpts) ([]*http.Cookie, error) {
	if header := os.Getenv("GENI_WEB_COOKIES"); header != "" {
		return web.CookiesFromHeader(header), nil
	}
	var browsers []string
	if g != nil && g.browser != "" {
		browsers = []string{g.browser}
	}
	cookies, err := browserCookieFetcher(browsers...)
	if err != nil {
		return nil, fmt.Errorf("could not read geni.com cookies from any browser "+
			"(set GENI_WEB_COOKIES to the Cookie header from a logged-in browser as a fallback): %w", err)
	}
	return cookies, nil
}
