# go-geni

Go client for the [Geni.com](https://www.geni.com) genealogy API. Extracted from
[terraform-provider-genealogy](https://github.com/dmalch/terraform-provider-genealogy)
so the same HTTP layer is usable from CLI tools, migration scripts, and other
projects.

## Disclaimer

This library uses the Geni API but is not endorsed, operated, or sponsored by
Geni.com.

## Install

```bash
go get github.com/dmalch/go-geni
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/dmalch/go-geni"
    "golang.org/x/oauth2"
)

func main() {
    token := os.Getenv("GENI_ACCESS_TOKEN")
    if token == "" {
        log.Fatal("set GENI_ACCESS_TOKEN")
    }

    client := geni.NewClient(
        oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
        true, // true = sandbox, false = production
    )

    profile, err := client.Profile().Get(context.Background(), "profile-1")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("name: %s %s\n",
        derefString(profile.FirstName), derefString(profile.LastName))
}

func derefString(p *string) string {
    if p == nil {
        return ""
    }
    return *p
}
```

A runnable version of this example lives in
[`examples/getprofile/`](examples/getprofile).

## Command-line tool

`cmd/geni` is a CLI façade over the library — handy for OAuth login and
quick read queries (`geni profile get`, `geni profile search`, `geni whoami`,
…) without writing Go:

```bash
go install github.com/dmalch/go-geni/cmd/geni@latest
geni login
geni profile get <id>
```

See [`cmd/geni/README.md`](cmd/geni/README.md) for the full command list,
auth, flags, and examples.

## OAuth

The `auth` subpackage offers browser-based OAuth helpers and a token cache,
suitable for interactive CLI tools. Both flows serve their callback on
`127.0.0.1` only and write the cache with mode `0600`.

**Client-side flow** — no client secret, but the token lasts a day and cannot
be renewed:

```go
import (
    "golang.org/x/oauth2"
    geni "github.com/dmalch/go-geni"
    "github.com/dmalch/go-geni/auth"
)

source := oauth2.ReuseTokenSource(nil,
    auth.NewCachingTokenSource(cachePath,
        auth.NewAuthTokenSource(&oauth2.Config{
            ClientID: "1855",
            Endpoint: auth.GeniEndpoint(geni.BaseURL(false)),
        })))
```

**Server-side flow** — needs the application's client secret and returns a
refresh token, so the browser is only involved once:

```go
cfg := &oauth2.Config{
    ClientID:     "1855",
    ClientSecret: os.Getenv("GENI_CLIENT_SECRET"),
    Endpoint:     auth.GeniEndpoint(geni.BaseURL(false)),
}
src := auth.NewCodeTokenSource(cfg)
source := oauth2.ReuseTokenSource(nil,
    auth.NewRefreshingCachingTokenSource(cachePath, src, src))
```

Refreshing has to sit *below* `oauth2.ReuseTokenSource`, which is why
`NewRefreshingCachingTokenSource` owns it: Geni rotates the refresh token on
every renewal, and a refresher wrapped around the cache would renew into
memory and lose the new token when the process exits.

Two Geni quirks are worth knowing if you build the config yourself.

**The callback address is not yours to choose.** It is whatever the
application's single registered Callback URL says — a Geni application accepts
exactly one — so `auth.WithPort` has to match that, and the authorization
request carries no `redirect_uri`. Two independent mechanisms stop you sending
one: Geni's WAF answers any query parameter holding a scheme-prefixed URL with
an empty **403** (the parameter name is irrelevant — `foo=http://…` is blocked
just the same, and Geni's OAuth layer never sees the request), and a value
crafted to slip past the WAF is then rejected by Geni itself with *"redirect_uri
cannot point to a different server than the one configured in the
application"* — including the protocol-relative form of the exact registered
URL. The application's type, Web or Native/Desktop, makes no difference.

**Geni rejects HTTP Basic client authentication** with `client_id must be
provided`, hence the `AuthStyleInParams` that `auth.GeniEndpoint` sets.

Headless callers can skip `auth` entirely and supply any `oauth2.TokenSource`
to `geni.NewClient`.

## Web (AJAX) client

Works around documented gaps in Geni's OAuth API (revision list, document text
r/w, merge-center matches list, merge data conflicts, tree conflicts) for personal genealogy tooling. The `web/` sub-tree is a **separate**
client that talks to the same private AJAX endpoints geni.com itself uses from
a logged-in browser — cookie auth, per-form CSRF token, HTML responses.

> ⚠️ These endpoints are undocumented, unsupported by Geni.com, and may change
> or break without notice. Using `web/` may violate geni.com's Terms of
> Service — review them before use. The package never logs in for you; it
> requires cookies from a logged-in browser session you established yourself.

```go
import (
    "context"
    "github.com/dmalch/go-geni/web"
    "github.com/dmalch/go-geni/web/revision"
    "github.com/dmalch/go-geni/web/document"
    "github.com/dmalch/go-geni/web/matches"
    "github.com/dmalch/go-geni/web/conflicts"
    "github.com/dmalch/go-geni/web/treeconflicts"
)

c, _ := web.NewClient(web.Options{
    Cookies: web.CookiesFromHeader("_geni_session=...; remember_user_token=..."),
})

// List a profile's revision IDs (the OAuth API can't).
ids, _ := revision.NewClient(c).ForProfile(context.Background(), "<profile-guid>")

// Read / write a document's text body (the OAuth API returns text:null).
text, _ := document.NewClient(c).GetText(context.Background(), "<doc-guid>")
err := document.NewClient(c).SaveText(context.Background(), "<doc-guid>", "new body")

// List the merge-center matches (the OAuth API has no equivalent).
res, _ := matches.NewClient(c).List(context.Background(), matches.ListOptions{
    Collection: matches.CollectionManaged,
    Filter:     matches.FilterTreeMatches,
})

// Drill into one profile's tree-match candidates.
fp, _ := matches.NewClient(c).ForProfile(context.Background(), "<profile-guid>",
    matches.ForProfileOptions{})

// List profiles with unresolved merge data conflicts, then clear one by
// keeping the surviving (primary) profile's values.
cl, _ := conflicts.NewClient(c).List(context.Background(), conflicts.ListOptions{})
err = conflicts.NewClient(c).Resolve(context.Background(), "<profile-guid>", nil)

// List profiles with unresolved tree conflicts (post-merge duplicate close
// relatives). Read-only — each row's TreeURL is the "Open tree" link to
// review and resolve the conflict by hand.
tc, _ := treeconflicts.NewClient(c).List(context.Background(),
    treeconflicts.ListOptions{Collection: "managed"})

// Inspect one tree conflict: its parent unions, the suspected duplicate
// relatives (with subtree sizes), and ready-to-run compare/merge commands.
detail, _ := treeconflicts.NewClient(c).Show(context.Background(), "<profile-id>")
```

Cookies can also be pulled directly from a logged-in browser on the host via
the opt-in `web/browsercookies` sub-package (uses
[`steipete/sweetcookie`](https://github.com/steipete/sweetcookie) — Chrome /
Firefox / Safari / Edge / Brave on macOS, Windows, Linux):

```go
import "github.com/dmalch/go-geni/web/browsercookies"

cookies, err := browsercookies.FromGeniCom()
c, _ := web.NewClient(web.Options{Cookies: cookies})
```

> **macOS, Safari:** this route cannot work. The OS reserves
> `~/Library/Containers/com.apple.Safari/…/Cookies.binarycookies` to Safari
> itself, and **Full Disk Access does not lift it** — a binary that holds FDA
> still gets `operation not permitted`. `FromGeniCom` says so
> (`ErrSafariCookiesUnreadable`) rather than reporting "no cookies found",
> which would read as "you are not logged in". Copy the `Cookie` header
> instead: Web Inspector → Network → any `geni.com` request → Headers.

The `geni` CLI takes that header from either of two environment variables,
checked before any browser store:

| variable | holds |
|---|---|
| `GENI_WEB_COOKIES` | the header itself |
| `GENI_WEB_COOKIES_FILE` | a path to a file holding it |

Prefer the file: a session cookie in an environment variable sits in the shell
history and in the process environment, where `ps -E` and a crash dump can read
it. A named file that cannot be read is an error, never a silent fall-through
to the browser stores, and its trailing newline is trimmed.

The Web client ships with a conservative **1 req/sec** rate limit by default
(`web.Options.RateLimit` overrides it on your own account). Runnable examples
live in [`examples/webrevisions`](examples/webrevisions) and
[`examples/webdocumenttext`](examples/webdocumenttext).

## Behaviour

- 1 request/second rate limit, adjusted on the fly from `X-API-Rate-Limit`
  response headers.
- Retries on 429 (rate limited), 401 (token expired), and transient transport
  errors via `github.com/avast/retry-go`.
- Bulk-read coalescing for profile/document/union endpoints when multiple
  concurrent reads target the same family of resources.
- Sandbox or production environment selectable per client.

## Documentation

API reference: <https://pkg.go.dev/github.com/dmalch/go-geni>

## Contributing

```bash
make test                # unit + Ginkgo integration (in-process)
make lint                # golangci-lint
make check               # build + vet + lint + test (CI parity)
```

The sandbox E2E suite under `test/acceptance/` self-skips unless
`GENI_ACCESS_TOKEN` is exported. Mint a sandbox token at
<https://sandbox.geni.com/platform/developer/api_explorer> and run
`make test-acceptance` before pushing changes that touch endpoint or
request-shape code. CI does not run E2E.

## License

Apache-2.0. See [`LICENSE`](LICENSE).
