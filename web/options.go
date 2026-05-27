package web

import (
	"net/http"
	"strings"
)

// Options configures a Web Client. Cookies is the only required field.
type Options struct {
	// Cookies carries the geni.com session. NewClient installs them on
	// a cookie jar scoped to BaseURL. Build with CookiesFromHeader for
	// the common case (paste from DevTools).
	Cookies []*http.Cookie

	// UserAgent is sent on every request. Defaults to "go-geni-web/1
	// (+https://github.com/dmalch/go-geni)".
	UserAgent string

	// BaseURL overrides the default https://www.geni.com host. Useful
	// for tests; production callers should leave it empty.
	BaseURL string

	// HTTPClient overrides the default *http.Client. The package
	// always sets the client's Jar to Cookies, so the override does
	// not need to.
	HTTPClient *http.Client

	// RateLimit caps outgoing requests in requests-per-second.
	// Defaults to 1 rps if unset or non-positive. Increase only when
	// you knowingly accept the additional load on your own account.
	RateLimit float64

	// CSRFSourcePath overrides the path the Client fetches to scrape
	// the authenticity_token. Defaults to
	// "/documents/save_document_content" — any page that renders a
	// Geni form with a CSRF input works.
	CSRFSourcePath string
}

// CookiesFromHeader parses a "name=value; name=value" cookie header
// (the form copied out of a browser's DevTools Network panel) into a
// slice of *http.Cookie suitable for Options.Cookies.
func CookiesFromHeader(header string) []*http.Cookie {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	pairs := strings.Split(header, ";")
	cookies := make([]*http.Cookie, 0, len(pairs))
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:  p[:eq],
			Value: p[eq+1:],
		})
	}
	return cookies
}
