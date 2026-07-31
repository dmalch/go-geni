package auth

import (
	"errors"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

// authTokenSource implements oauth2.TokenSource using Geni's client-side
// flow: the access token comes straight back on the callback, and no
// refresh token is issued. See NewCodeTokenSource for the refreshable
// alternative.
type authTokenSource struct {
	config *oauth2.Config
	loopbackFlow
}

func NewAuthTokenSource(config *oauth2.Config, opts ...Option) *authTokenSource {
	return &authTokenSource{
		config:       config,
		loopbackFlow: loopbackFlow{options: newOptions(opts...)},
	}
}

// Token retrieves a new token, performing the OAuth flow if necessary.
func (a *authTokenSource) Token() (*oauth2.Token, error) {
	query, err := a.authorize(a.authCodeURL)
	if err != nil {
		return nil, err
	}
	if query.Get("access_token") == "" {
		return nil, errors.New("no authentication access token was received")
	}
	return a.tokenFrom(query), nil
}

// authCodeURL builds the authorization URL. It is kept free of any
// listener state so the shape of the URL can be asserted on its own.
//
// No redirect_uri is sent, and none can usefully be: Geni's WAF answers
// any query parameter holding a scheme-prefixed URL with an empty 403
// (the parameter name is irrelevant — foo=http://… is blocked too), and
// a value crafted to slip past it is then rejected by Geni itself with
// "redirect_uri cannot point to a different server than the one
// configured in the application" — including the protocol-relative form
// of the exact registered URL. The redirect target is therefore always
// the Callback URL registered with the application.
func (a *authTokenSource) authCodeURL(state string) string {
	return a.config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("response_type", "token"),
		oauth2.SetAuthURLParam("display", displayWeb),
	)
}

// tokenFrom builds the token from the callback query parameters.
func (a *authTokenSource) tokenFrom(query url.Values) *oauth2.Token {
	raw := query.Get("expires_in")
	expiresIn, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || expiresIn <= 0 {
		// A valid access token must not be thrown away over a missing
		// lifetime. Assuming one is safe; leaving Expiry zero is not —
		// oauth2.Token treats that as "never expires", so the cache would
		// serve a dead token forever.
		slog.Warn("the OAuth callback carried no usable expires_in; assuming the default token lifetime",
			"expires_in", raw, "assumed", a.lifetime)
		expiresIn = int64(a.lifetime / time.Second)
	}

	return &oauth2.Token{
		AccessToken: query.Get("access_token"),
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		Expiry:      time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
}
