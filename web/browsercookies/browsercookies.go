// Package browsercookies is an OPT-IN helper for bootstrapping
// Options.Cookies from a logged-in browser session on the host
// machine. Importing it pulls in github.com/steipete/sweetcookie and
// its browser backends — callers that already have a cookie header
// should not import this package.
//
// Only valid (non-expired) cookies for geni.com are returned.
package browsercookies

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/steipete/sweetcookie"
)

var (
	// ErrNoCookies is returned when no geni.com cookies were found in
	// any browser store on the host. Log in to geni.com in a browser
	// first.
	ErrNoCookies = errors.New("browsercookies: no geni.com cookies found in any browser")

	// ErrFullDiskAccessRequired wraps macOS "operation not permitted"
	// failures reading browser cookie stores — usually Safari's
	// container, which requires Full Disk Access. Grant it in
	// System Settings → Privacy & Security → Full Disk Access for
	// the binary running this code (e.g. your terminal).
	ErrFullDiskAccessRequired = errors.New(
		"browsercookies: cannot read browser cookie store (on macOS, " +
			"grant Full Disk Access in System Settings → Privacy & Security)")
)

// SupportedBrowsers lists the browser names accepted by FromGeniCom.
// Matches sweetcookie's backends. Stored as lowercase strings to
// avoid leaking the sweetcookie.Browser type to callers.
var SupportedBrowsers = []string{
	"chrome", "edge", "brave", "arc", "chromium",
	"vivaldi", "opera", "firefox", "safari",
}

// readCookies is the sweetcookie entry point, indirected for tests.
var readCookies = func(browsers []sweetcookie.Browser) (sweetcookie.Result, error) {
	return sweetcookie.Get(context.Background(), sweetcookie.Options{
		URL:      "https://www.geni.com/",
		Browsers: browsers,
	})
}

// FromGeniCom reads valid (non-expired) geni.com cookies from the
// host's browser stores and returns them as []*http.Cookie suitable
// for web.Options.Cookies. With no arguments, sweetcookie's default
// browser priority is used. With one or more browser names, only
// those backends are queried (and in the order given). Names are
// case-insensitive; see SupportedBrowsers for the valid set.
func FromGeniCom(browsers ...string) ([]*http.Cookie, error) {
	bs, err := parseBrowsers(browsers)
	if err != nil {
		return nil, err
	}
	res, err := readCookies(bs)
	if err != nil {
		if isPermissionDenied(err) {
			return nil, fmt.Errorf("%w: %w", ErrFullDiskAccessRequired, err)
		}
		return nil, err
	}
	if len(res.Cookies) == 0 {
		return nil, ErrNoCookies
	}
	return toHTTPCookies(res.Cookies), nil
}

func parseBrowsers(names []string) ([]sweetcookie.Browser, error) {
	if len(names) == 0 {
		return nil, nil
	}
	out := make([]sweetcookie.Browser, 0, len(names))
	for _, n := range names {
		norm := strings.ToLower(strings.TrimSpace(n))
		if !slices.Contains(SupportedBrowsers, norm) {
			return nil, fmt.Errorf("browsercookies: unknown browser %q (supported: %s)",
				n, strings.Join(SupportedBrowsers, ", "))
		}
		out = append(out, sweetcookie.Browser(norm))
	}
	return out, nil
}

func toHTTPCookies(in []sweetcookie.Cookie) []*http.Cookie {
	if in == nil {
		return nil
	}
	out := make([]*http.Cookie, len(in))
	for i, c := range in {
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HttpOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
		if c.Expires != nil {
			hc.Expires = *c.Expires
		}
		out[i] = hc
	}
	return out
}

func isPermissionDenied(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "permission denied")
}
