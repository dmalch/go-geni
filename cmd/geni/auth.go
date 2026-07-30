package main

import (
	"fmt"
	"os"
	"path/filepath"

	geni "github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/auth"
	"golang.org/x/oauth2"
)

// authChain builds the token source used by every command. A
// GENI_ACCESS_TOKEN env var short-circuits to a static token;
// otherwise a cache-backed interactive OAuth source is returned, where
// a cache miss opens the browser.
func authChain(useSandbox bool, opts ...auth.Option) (oauth2.TokenSource, error) {
	if t := os.Getenv("GENI_ACCESS_TOKEN"); t != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: t}), nil
	}

	cachePath, refresher, interactive, err := authParts(useSandbox, opts...)
	if err != nil {
		return nil, err
	}

	// ReuseTokenSource memoizes in memory: without it every API call
	// would re-read and re-parse the cache file, since the transport asks
	// for a token per request.
	if refresher == nil {
		return oauth2.ReuseTokenSource(nil, auth.NewCachingTokenSource(cachePath, interactive)), nil
	}
	return oauth2.ReuseTokenSource(nil,
		auth.NewRefreshingCachingTokenSource(cachePath, refresher, interactive)), nil
}

// authParts resolves the pieces authChain composes. refresher is nil when
// no client secret is configured, which is what selects the client-side
// flow and its daily browser round trip.
func authParts(useSandbox bool, opts ...auth.Option) (string, auth.Refresher, oauth2.TokenSource, error) {
	cachePath, err := tokenCacheFilePath(useSandbox)
	if err != nil {
		return "", nil, nil, err
	}

	app, err := clientApp(useSandbox)
	if err != nil {
		return "", nil, nil, err
	}

	cfg := &oauth2.Config{
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		Endpoint:     auth.GeniEndpoint(geni.BaseURL(useSandbox)),
	}

	if app.ClientSecret == "" {
		return cachePath, nil, auth.NewAuthTokenSource(cfg, opts...), nil
	}

	src := auth.NewCodeTokenSource(cfg, opts...)
	return cachePath, src, src, nil
}

// newClient builds a geni.Client for the resolved environment.
func newClient(g *globalOpts) (*geni.Client, error) {
	ts, err := authChain(g.sandbox)
	if err != nil {
		return nil, err
	}
	return geni.NewClient(ts, g.sandbox), nil
}

// clientApp resolves the OAuth application to authenticate as.
//
// The id travels with the secret: the secret for the built-in
// application belongs to its owner, so anyone else registers their own
// application at https://www.geni.com/platform/developer/apps and
// configures both halves.
//
// Order: environment variables, then ~/.genealogy/config.json, then the
// built-in id with no secret.
func clientApp(useSandbox bool) (oauthApp, error) {
	idVar, secretVar := "GENI_CLIENT_ID", "GENI_CLIENT_SECRET"
	if useSandbox {
		idVar, secretVar = "GENI_SANDBOX_CLIENT_ID", "GENI_SANDBOX_CLIENT_SECRET"
	}

	app := oauthApp{ClientID: os.Getenv(idVar), ClientSecret: os.Getenv(secretVar)}
	if app.ClientID != "" || app.ClientSecret != "" {
		if app.ClientID == "" {
			app.ClientID = clientID(useSandbox)
		}
		return app, nil
	}

	c, err := loadUserConfig()
	if err != nil {
		return oauthApp{}, err
	}
	app = c.Prod
	if useSandbox {
		app = c.Sandbox
	}
	if app.ClientID == "" {
		app.ClientID = clientID(useSandbox)
	}
	return app, nil
}

// clientID returns the registered Geni OAuth client id for the
// selected environment.
func clientID(useSandbox bool) string {
	if useSandbox {
		return "8"
	}
	return "1855"
}

// tokenCacheFilePath returns the on-disk token cache location for the
// selected environment.
func tokenCacheFilePath(useSandbox bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	name := "geni_token.json"
	if useSandbox {
		name = "geni_sandbox_token.json"
	}
	return filepath.Join(home, ".genealogy", name), nil
}
