package acceptance

import (
	"context"
	"os"
	"path"
	"testing"

	"golang.org/x/oauth2"

	"github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/auth"
)

// sandboxClientId is the Geni OAuth client id registered for the
// sandbox environment (matches the value the Terraform provider uses).
const sandboxClientId = "8"

func sandboxAuthURL() string {
	return geni.BaseURL(true) + "platform/oauth/authorize"
}

// sandboxTokenCachePath is the on-disk location for the cached sandbox
// access token. Matches the path used by terraform-provider-genealogy,
// so a previously-authorized provider session avoids a fresh browser
// prompt here.
func sandboxTokenCachePath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home dir: %v", err)
	}
	return path.Join(home, ".genealogy", "geni_sandbox_token.json")
}

// tokenSource resolves a sandbox OAuth TokenSource for an acceptance
// test or skips the test when no usable path is configured. Order:
//
//  1. GENI_ACCESS_TOKEN — static token (non-interactive, CI-friendly).
//  2. Cached token at ~/.genealogy/geni_sandbox_token.json — written by
//     a prior interactive run (or by the Terraform provider). When the
//     cache exists we always allow refresh via OAuth, since a human
//     already authorized this machine at some point.
//  3. Interactive browser OAuth flow, gated on GENI_OAUTH=1 so that a
//     bare `go test ./...` in CI still self-skips. `make test-acceptance`
//     sets the flag for you.
//
// On any other combination the test skips with a hint.
func tokenSource(t *testing.T) oauth2.TokenSource {
	t.Helper()

	if tok := os.Getenv("GENI_ACCESS_TOKEN"); tok != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
	}

	cachePath := sandboxTokenCachePath(t)
	cacheExists := false
	if _, err := os.Stat(cachePath); err == nil {
		cacheExists = true
	}

	if !cacheExists && os.Getenv("GENI_OAUTH") == "" {
		t.Skip("acceptance tests need a sandbox token. Run `make test-acceptance` to start an interactive OAuth flow, or set GENI_ACCESS_TOKEN to a token minted at https://sandbox.geni.com/platform/developer/api_explorer")
	}

	return oauth2.ReuseTokenSource(nil,
		auth.NewCachingTokenSource(cachePath,
			auth.NewAuthTokenSource(&oauth2.Config{
				ClientID: sandboxClientId,
				Endpoint: oauth2.Endpoint{AuthURL: sandboxAuthURL()},
			})))
}

// newTestClient resolves a token source (skipping the test if none is
// available) and returns a go-geni Client pointed at the sandbox.
func newTestClient(t *testing.T) *geni.Client {
	t.Helper()
	return geni.NewClient(tokenSource(t), true)
}

// strPtr is a tiny helper for building optional string fields in
// ProfileRequest / DocumentRequest etc.
func strPtr(s string) *string { return &s }

// createFixtureProfile creates a deceased, public profile in the sandbox
// and registers a t.Cleanup hook that deletes it. Tests keep created
// profiles non-living and public to minimize side effects on the
// sandbox tree.
func createFixtureProfile(t *testing.T, ctx context.Context, client *geni.Client, firstName, lastName string) *geni.ProfileResponse {
	t.Helper()
	created, err := client.CreateProfile(ctx, &geni.ProfileRequest{
		Names: map[string]geni.NameElement{
			"en-US": {
				FirstName: strPtr(firstName),
				LastName:  strPtr(lastName),
			},
		},
		IsAlive: false,
		Public:  true,
	})
	if err != nil {
		t.Fatalf("create fixture profile: %v", err)
	}
	t.Cleanup(func() {
		if err := client.DeleteProfile(context.Background(), created.Id); err != nil {
			t.Logf("cleanup: delete profile %s: %v", created.Id, err)
		}
	})
	return created
}
