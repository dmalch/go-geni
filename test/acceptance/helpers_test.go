package acceptance

import (
	"context"
	"os"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
// access token. Matches the path used by terraform-provider-genealogy
// so a previously-authorized provider session avoids a fresh browser
// prompt here.
func sandboxTokenCachePath() string {
	GinkgoHelper()
	home, err := os.UserHomeDir()
	Expect(err).ToNot(HaveOccurred())
	return path.Join(home, ".genealogy", "geni_sandbox_token.json")
}

// tokenSource resolves a sandbox OAuth TokenSource for the current spec
// or calls Skip() when no usable path is configured. Order:
//
//  1. GENI_ACCESS_TOKEN — static token (non-interactive, CI-friendly).
//  2. Cached token at ~/.genealogy/geni_sandbox_token.json — written by
//     a prior interactive run (or by the Terraform provider). When the
//     cache exists we always allow refresh via OAuth, since a human
//     already authorized this machine at some point.
//  3. Interactive browser OAuth flow, gated on GENI_OAUTH=1 so that a
//     bare `go test ./...` in CI still self-skips. `make test-acceptance`
//     sets the flag for you.
func tokenSource() oauth2.TokenSource {
	GinkgoHelper()

	if tok := os.Getenv("GENI_ACCESS_TOKEN"); tok != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
	}

	cachePath := sandboxTokenCachePath()
	cacheExists := false
	if _, err := os.Stat(cachePath); err == nil {
		cacheExists = true
	}

	if !cacheExists && os.Getenv("GENI_OAUTH") == "" {
		Skip("acceptance tests need a sandbox token. Run `make test-acceptance` to start an interactive OAuth flow, or set GENI_ACCESS_TOKEN to a token minted at https://sandbox.geni.com/platform/developer/api_explorer")
	}

	return oauth2.ReuseTokenSource(nil,
		auth.NewCachingTokenSource(cachePath,
			auth.NewAuthTokenSource(&oauth2.Config{
				ClientID: sandboxClientId,
				Endpoint: oauth2.Endpoint{AuthURL: sandboxAuthURL()},
				// "family" is required for ancestor / tree-walking
				// endpoints; without it Geni returns 403. If you see
				// a 403 unexpectedly after upgrading, delete the
				// cached token at the path returned by
				// sandboxTokenCachePath() to force a re-authorization
				// with the broader scope.
				Scopes: []string{"family"},
			})))
}

// newTestClient resolves a token source (skipping the spec if none is
// available) and returns a go-geni Client pointed at the sandbox.
func newTestClient() *geni.Client {
	GinkgoHelper()
	return geni.NewClient(tokenSource(), true)
}

func strPtr(s string) *string { return &s }

// createFixtureProfile creates a deceased, public profile in the sandbox
// and registers a DeferCleanup hook that deletes it after the current
// spec finishes. Profiles are kept non-living and public to minimize
// side effects on the sandbox tree. The last name is always
// "Acceptance" so fixtures are easy to recognise + scrub manually.
func createFixtureProfile(ctx context.Context, client *geni.Client, firstName string) *geni.ProfileResponse {
	GinkgoHelper()
	created, err := client.CreateProfile(ctx, &geni.ProfileRequest{
		Names: map[string]geni.NameElement{
			"en-US": {
				FirstName: strPtr(firstName),
				LastName:  strPtr("Acceptance"),
			},
		},
		IsAlive: false,
		Public:  true,
	})
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		_ = client.DeleteProfile(context.Background(), created.Id)
	})
	return created
}
