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
	// failures reading a CHROMIUM-family browser cookie store. Grant
	// it in System Settings → Privacy & Security → Full Disk Access
	// for the binary running this code (e.g. your terminal); macOS
	// grants the permission per binary, not per user.
	ErrFullDiskAccessRequired = errors.New(
		"browsercookies: cannot read browser cookie store (on macOS, " +
			"grant Full Disk Access in System Settings → Privacy & Security)")

	// ErrSafariCookiesUnreadable is Safari's case, and it is NOT the
	// one above: macOS reserves the Safari container to Safari itself,
	// so Full Disk Access does not lift it. Verified 2026-09-19 on
	// macOS 27 — a binary holding FDA (it reads Photos.sqlite) still
	// gets EPERM on Cookies.binarycookies. Telling the user to grant
	// FDA here sends them to a setting that cannot help.
	ErrSafariCookiesUnreadable = errors.New(
		"browsercookies: Safari's cookie store is unreadable — macOS reserves it " +
			"to Safari itself and Full Disk Access does NOT lift that; copy the " +
			"Cookie header from a logged-in page (Web Inspector → Network → any " +
			"geni.com request) into GENI_WEB_COOKIES_FILE or GENI_WEB_COOKIES")
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
		return nil, emptyResultError(res.Warnings)
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

// emptyResultError explains an empty read. sweetcookie reports a store
// it could not OPEN as a warning and still returns (Result{}, nil), so
// without this the caller cannot tell "you are not logged in" from "the
// file is there and the OS refused it" — and reports the first, which
// is the opposite of the truth and sends the reader hunting for a login
// problem that does not exist.
func emptyResultError(warnings []string) error {
	for _, w := range warnings {
		if !isPermissionDeniedMsg(w) {
			continue
		}
		if strings.Contains(strings.ToLower(w), "safari") {
			return fmt.Errorf("%w: %s", ErrSafariCookiesUnreadable, w)
		}
		return fmt.Errorf("%w: %s", ErrFullDiskAccessRequired, w)
	}
	if len(warnings) == 0 {
		return ErrNoCookies
	}
	return fmt.Errorf("%w (%s)", ErrNoCookies, strings.Join(warnings, "; "))
}

func isPermissionDenied(err error) bool {
	return isPermissionDeniedMsg(err.Error())
}

func isPermissionDeniedMsg(msg string) bool {
	return strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "permission denied")
}
