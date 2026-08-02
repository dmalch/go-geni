package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
)

// tokenEndpoint stands in for /platform/oauth/request_token. It records
// the form it was posted and replies with body, verbatim.
type tokenEndpoint struct {
	form        url.Values
	authHeader  string
	status      int
	contentType string
	body        string
}

func (e *tokenEndpoint) start(t *testing.T) *oauth2.Config {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse the token request: %v", err)
		}
		e.form = r.PostForm
		e.authHeader = r.Header.Get("Authorization")

		contentType := e.contentType
		if contentType == "" {
			contentType = "application/json; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		if e.status != 0 {
			w.WriteHeader(e.status)
		}
		_, _ = w.Write([]byte(e.body))
	}))
	t.Cleanup(server.Close)

	return &oauth2.Config{
		ClientID:     "1855",
		ClientSecret: "app-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://www.geni.com/platform/oauth/authorize",
			TokenURL:  server.URL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

func TestGeniEndpoint(t *testing.T) {
	// Geni answers HTTP Basic authentication with "client_id must be
	// provided", so the credentials have to travel in the body and
	// autodetection would waste a round trip on every call.
	t.Run("Puts the client credentials in the request body", func(t *testing.T) {
		RegisterTestingT(t)

		endpoint := GeniEndpoint("https://www.geni.com/")
		Expect(endpoint.AuthURL).To(Equal("https://www.geni.com/platform/oauth/authorize"))
		Expect(endpoint.TokenURL).To(Equal("https://www.geni.com/platform/oauth/request_token"))
		Expect(endpoint.AuthStyle).To(Equal(oauth2.AuthStyleInParams))
	})
}

func TestCodeTokenSourceToken(t *testing.T) {
	t.Run("Exchanges the authorization code for a refreshable token", func(t *testing.T) {
		RegisterTestingT(t)

		endpoint := &tokenEndpoint{body: `{"access_token":"at","refresh_token":"rt","expires_in":86400}`}
		src := NewCodeTokenSource(endpoint.start(t), WithPort(0))

		var opened string
		restore := stubOpenBrowser(respondToCodeCallback(src, &opened, "the-code"))
		defer restore()

		var token *oauth2.Token
		var err error
		captureStderr(t, func() { token, err = src.Token() })

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("at"))
		Expect(token.RefreshToken).To(Equal("rt"))

		Expect(endpoint.form.Get("grant_type")).To(Equal("authorization_code"))
		Expect(endpoint.form.Get("code")).To(Equal("the-code"))
		Expect(endpoint.form.Get("client_id")).To(Equal("1855"))
		Expect(endpoint.form.Get("client_secret")).To(Equal("app-secret"))
		Expect(endpoint.authHeader).To(BeEmpty())
		// No redirect_uri can be sent on the authorization request, so
		// none is registered on the config and none reaches the exchange.
		Expect(endpoint.form.Has("redirect_uri")).To(BeFalse())
	})

	t.Run("Asks for a code on the desktop-sized screen", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewCodeTokenSource(&oauth2.Config{
			ClientID: "1855",
			Endpoint: GeniEndpoint("https://www.geni.com/"),
		})

		q := queryOf(t, src.authCodeURL("some-state"))
		Expect(q.Get("response_type")).To(Equal("code"))
		Expect(q.Get("display")).To(Equal("web"))
		Expect(q.Has("redirect_uri")).To(BeFalse())
	})

	t.Run("Fails when the callback carries no code", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewCodeTokenSource((&tokenEndpoint{}).start(t), WithPort(0))

		var opened string
		restore := stubOpenBrowser(respondToCallback(&src.loopbackFlow, &opened, url.Values{}))
		defer restore()

		var err error
		captureStderr(t, func() { _, err = src.Token() })

		Expect(err).To(MatchError(ContainSubstring("no authorization code was received")))
	})

	// oauth2 attaches the request body to a retrieval error, and that body
	// carries the client secret.
	t.Run("Keeps the client secret out of the exchange error", func(t *testing.T) {
		RegisterTestingT(t)

		endpoint := &tokenEndpoint{
			status: http.StatusBadRequest,
			body:   `{"error":"invalid_request","error_description":"Invalid authorization code"}`,
		}
		src := NewCodeTokenSource(endpoint.start(t), WithPort(0))

		var opened string
		restore := stubOpenBrowser(respondToCodeCallback(src, &opened, "stale-code"))
		defer restore()

		var err error
		captureStderr(t, func() { _, err = src.Token() })

		Expect(err).To(MatchError(ContainSubstring("Invalid authorization code")))
		Expect(err.Error()).ToNot(ContainSubstring("app-secret"))
	})
}

func TestCodeTokenSourceRefresh(t *testing.T) {
	t.Run("Renews the token and returns the rotated refresh token", func(t *testing.T) {
		RegisterTestingT(t)

		endpoint := &tokenEndpoint{body: `{"access_token":"new-at","refresh_token":"new-rt","expires_in":86400}`}
		src := NewCodeTokenSource(endpoint.start(t))

		token, err := src.Refresh(t.Context(), "old-rt")

		Expect(err).ToNot(HaveOccurred())
		Expect(token.AccessToken).To(Equal("new-at"))
		Expect(token.RefreshToken).To(Equal("new-rt"))
		Expect(endpoint.form.Get("grant_type")).To(Equal("refresh_token"))
		Expect(endpoint.form.Get("refresh_token")).To(Equal("old-rt"))
		Expect(endpoint.form.Get("client_secret")).To(Equal("app-secret"))
		// Geni renews happily without one, and sending it would 403 the
		// authorization step, so the config carries no RedirectURL.
		Expect(endpoint.form.Has("redirect_uri")).To(BeFalse())
	})

	t.Run("Keeps the old refresh token when the response omits one", func(t *testing.T) {
		RegisterTestingT(t)

		endpoint := &tokenEndpoint{body: `{"access_token":"new-at","expires_in":86400}`}
		src := NewCodeTokenSource(endpoint.start(t))

		token, err := src.Refresh(t.Context(), "old-rt")

		Expect(err).ToNot(HaveOccurred())
		Expect(token.RefreshToken).To(Equal("old-rt"))
	})

	// Geni labels every rejection "invalid_request" rather than the
	// RFC 6749 "invalid_grant", so the status is what has to decide.
	t.Run("Treats a rejected grant as recoverable by logging in again", func(t *testing.T) {
		RegisterTestingT(t)

		endpoint := &tokenEndpoint{
			status: http.StatusBadRequest,
			body:   `{"error":"invalid_request","error_description":"Invalid refresh token"}`,
		}
		src := NewCodeTokenSource(endpoint.start(t))

		_, err := src.Refresh(t.Context(), "old-rt")

		Expect(err).To(MatchError(ErrRefreshRejected))
		Expect(err.Error()).To(ContainSubstring("Invalid refresh token"))
		Expect(err.Error()).ToNot(ContainSubstring("app-secret"))
	})

	// A bad afternoon at Geni must not be answered with a browser window.
	t.Run("Treats a server error as transient", func(t *testing.T) {
		RegisterTestingT(t)

		endpoint := &tokenEndpoint{
			status:      http.StatusInternalServerError,
			contentType: "text/html",
			body:        "<html>we are sorry</html>",
		}
		src := NewCodeTokenSource(endpoint.start(t))

		_, err := src.Refresh(t.Context(), "old-rt")

		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrRefreshRejected)).To(BeFalse())
		Expect(err.Error()).ToNot(ContainSubstring("app-secret"))
	})

	t.Run("Reports a token response with no access token", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewCodeTokenSource((&tokenEndpoint{body: `{"expires_in":86400}`}).start(t))

		_, err := src.Refresh(t.Context(), "old-rt")

		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ErrRefreshRejected)).To(BeFalse())
	})
}

// respondToCodeCallback stands in for the browser in the server-side
// flow, calling back with an authorization code.
func respondToCodeCallback(src *codeTokenSource, opened *string, code string) func(string) error {
	return respondToCallback(&src.loopbackFlow, opened, url.Values{"code": {code}})
}
