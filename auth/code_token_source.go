package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// ErrRefreshRejected marks a refresh that only a fresh interactive login
// can recover: the refresh token was revoked, already rotated away, or
// belongs to another application.
var ErrRefreshRejected = errors.New("the refresh token was rejected")

// Refresher renews an access token from a refresh token.
type Refresher interface {
	Refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error)
}

// codeTokenSource implements Geni's server-side flow: the browser returns
// an authorization code, which is exchanged for an access token and — the
// reason to prefer this flow — a refresh token.
type codeTokenSource struct {
	config *oauth2.Config
	loopbackFlow
}

// NewCodeTokenSource returns a TokenSource that runs Geni's server-side
// flow. config needs a ClientSecret and an Endpoint carrying both AuthURL
// and TokenURL; use GeniEndpoint to build one.
func NewCodeTokenSource(config *oauth2.Config, opts ...Option) *codeTokenSource {
	return &codeTokenSource{
		config:       config,
		loopbackFlow: loopbackFlow{options: newOptions(opts...)},
	}
}

// GeniEndpoint returns Geni's OAuth endpoints for the given base URL,
// which is what geni.BaseURL produces.
//
// The client credentials go in the request body: Geni answers HTTP Basic
// authentication with "client_id must be provided", so letting
// golang.org/x/oauth2 probe for the style would waste a round trip on
// every exchange and refresh.
func GeniEndpoint(baseURL string) oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:   baseURL + "platform/oauth/authorize",
		TokenURL:  baseURL + "platform/oauth/request_token",
		AuthStyle: oauth2.AuthStyleInParams,
	}
}

// Token runs the browser flow and exchanges the resulting code.
func (c *codeTokenSource) Token() (*oauth2.Token, error) {
	query, err := c.authorize(c.authCodeURL)
	if err != nil {
		return nil, err
	}

	code := query.Get("code")
	if code == "" {
		return nil, errors.New("no authorization code was received")
	}

	token, err := c.config.Exchange(c.ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange the authorization code: %w", redactTokenError(err))
	}
	return token, nil
}

// Refresh renews an access token, returning a token that carries the
// refresh token to store next. Geni rotates the refresh token on every
// refresh, so the result must be persisted, not just used.
func (c *codeTokenSource) Refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	// Building the source per call rather than once is deliberate:
	// oauth2's refresher captures the refresh token at construction time,
	// and the current one is only known after the cache has been read.
	token, err := c.config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken}).Token()
	if err != nil {
		return nil, classifyRefreshError(err)
	}

	// oauth2 already carries the old refresh token forward when the
	// response omits one, so the caller never loses it.
	return token, nil
}

// authCodeURL builds the authorization URL for the server-side flow. As
// in the client-side flow, no redirect_uri is sent; see
// authTokenSource.authCodeURL for the two separate reasons none can be.
func (c *codeTokenSource) authCodeURL(state string) string {
	return c.config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("response_type", "code"),
		oauth2.SetAuthURLParam("display", displayWeb),
	)
}

// classifyRefreshError decides whether a failed refresh is recoverable.
//
// Geni labels every rejection "invalid_request" rather than the
// RFC 6749 "invalid_grant", so the error code says nothing useful and the
// status is what distinguishes a dead grant (4xx, log in again) from a
// bad afternoon at Geni (5xx or a transport failure, report and stop).
func classifyRefreshError(err error) error {
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.Response != nil &&
		retrieveErr.Response.StatusCode >= http.StatusBadRequest &&
		retrieveErr.Response.StatusCode < http.StatusInternalServerError {
		return fmt.Errorf("%w: %s", ErrRefreshRejected, retrieveErr.ErrorDescription)
	}
	return fmt.Errorf("failed to refresh the OAuth token: %w", redactTokenError(err))
}

// redactTokenError strips the request body oauth2 attaches to a retrieval
// error, which would otherwise put the client secret in a log line.
func redactTokenError(err error) error {
	var retrieveErr *oauth2.RetrieveError
	if !errors.As(err, &retrieveErr) {
		return err
	}
	status := "unknown status"
	if retrieveErr.Response != nil {
		status = retrieveErr.Response.Status
	}
	if retrieveErr.ErrorDescription != "" {
		return fmt.Errorf("%s: %s", status, retrieveErr.ErrorDescription)
	}
	return errors.New(status)
}
