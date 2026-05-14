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

    profile, err := client.GetProfile(context.Background(), "profile-1")
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

## OAuth

The `auth` subpackage offers a browser-based OAuth implicit-flow helper and a
token cache, suitable for interactive CLI tools:

```go
import (
    "golang.org/x/oauth2"
    "github.com/dmalch/go-geni/auth"
)

source := oauth2.ReuseTokenSource(nil,
    auth.NewCachingTokenSource("~/.geni/token.json",
        auth.NewAuthTokenSource(&oauth2.Config{
            ClientID: "1855",
            Endpoint: oauth2.Endpoint{
                AuthURL: "https://www.geni.com/platform/oauth/authorize",
            },
        })))
```

Headless callers can skip `auth` entirely and supply any `oauth2.TokenSource`
to `geni.NewClient`.

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
make test                # unit + Ginkgo acceptance (in-process)
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
