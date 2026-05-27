package web

import "errors"

var (
	// ErrNoCookies is returned by NewClient when Options.Cookies is nil.
	// The Web client never logs in itself; the caller must supply a
	// cookie jar obtained from a logged-in browser.
	ErrNoCookies = errors.New("web: no cookies supplied (use CookiesFromHeader or CookiesFromJar)")

	// ErrNotLoggedIn is returned when the response indicates the session
	// is missing or expired (typically a 302 redirect to /login).
	ErrNotLoggedIn = errors.New("web: not logged in (session cookie missing or expired)")

	// ErrBlocked is returned when geni.com's edge serves an Incapsula /
	// bot-challenge page instead of the requested content.
	ErrBlocked = errors.New("web: request blocked by upstream bot protection")

	// ErrCSRFExpired is returned when geni.com rejects a request with an
	// expired authenticity_token. Callers normally do not see this — the
	// client refreshes and retries internally.
	ErrCSRFExpired = errors.New("web: CSRF authenticity_token rejected")
)
