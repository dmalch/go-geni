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
func authChain(useSandbox bool) (oauth2.TokenSource, error) {
	if t := os.Getenv("GENI_ACCESS_TOKEN"); t != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: t}), nil
	}

	cachePath, err := tokenCacheFilePath(useSandbox)
	if err != nil {
		return nil, err
	}

	cfg := &oauth2.Config{
		ClientID: clientID(useSandbox),
		Endpoint: oauth2.Endpoint{
			AuthURL: geni.BaseURL(useSandbox) + "platform/oauth/authorize",
		},
	}
	return oauth2.ReuseTokenSource(nil,
		auth.NewCachingTokenSource(cachePath, auth.NewAuthTokenSource(cfg))), nil
}

// newClient builds a geni.Client for the resolved environment.
func newClient(g *globalOpts) (*geni.Client, error) {
	ts, err := authChain(g.sandbox)
	if err != nil {
		return nil, err
	}
	return geni.NewClient(ts, g.sandbox), nil
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
