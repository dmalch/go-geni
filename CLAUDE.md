# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`github.com/dmalch/go-geni` — Go client for the Geni.com genealogy API.
Extracted from `terraform-provider-genealogy` v0.20.1's `internal/geni/`
so the same HTTP layer is reusable from CLI tools and migration scripts.
The package was reshaped for 1.0 into one sub-package per resource
(`profile/`, `union/`, `document/`, …) behind a thin root façade; see
`CHANGELOG.md`. Alongside the OAuth client the module ships a
cookie-authenticated client for Geni's private AJAX endpoints (`web/`)
and the `geni` CLI (`cmd/geni/`).

## Commands

```bash
make build                               # go build ./...
make test                                # unit + Ginkgo integration (in-process)
make lint                                # golangci-lint
make test-acceptance                     # E2E against sandbox (see tier 3 below)
make check                               # build + vet + lint + test (CI parity)

go test -run TestProfile ./...           # single test by name
go test -run TestFoo/subtest ./...       # subtest of a table-driven test
```

CI (`.github/workflows/ci.yaml`) runs build + test + vet + golangci-lint
on every push/PR to `main`. Keep the working tree warning-free under the
enabled linters (`.golangci.yml`: errcheck, staticcheck, unused, unparam,
godot, modernize, nilnil, …). CI pins golangci-lint **v2.12.2**; without
a local install, `go run
github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...`
gives the same result. CI does **not** run `test/acceptance/` — those need
a real OAuth token, which the harness can't mint.

Tests come in three tiers:

1. **Unit** — plain `testing.T` + Gomega matchers, in-package. Each
   resource package fakes the wire with its own `fakeTransport` +
   `newFakeClient` (`<resource>/*_test.go`); `transport/` has
   `headerEchoTransport`, `scriptedTransport` (queued responses, for the
   retry ladder) and `bodyRecordingTransport`; the root `coalesce_test.go`
   uses `countingTransport`. `cmd/geni` tests drive `run()` end to end and
   swap package-level seams (`browserCookieFetcher`, `newAPITransport`,
   `newAPIWebClient`) for `httptest` servers.
2. **Integration** — Ginkgo v2 BDD specs, one suite per resource package:
   a `suite_test.go` bootstrap (`Test<Resource>Integration`) plus
   `*_integration_test.go`. Still in-process: a `rewriteTransport` sends the
   transport's fixed geni.com URLs to an `httptest.NewServer` serving inline
   JSON. `web/conflicts`, `web/matches` and `web/treeconflicts` parse
   saved HTML pages from their `testdata/`.
3. **Sandbox E2E** (`test/acceptance/`, a Ginkgo suite) — the token comes
   from `GENI_ACCESS_TOKEN`, else the cached
   `~/.genealogy/geni_sandbox_token.json`, else an interactive browser
   OAuth flow gated on `GENI_OAUTH=1` (which `make test-acceptance` sets);
   with none of these the specs self-skip. A static token can be minted at
   <https://sandbox.geni.com/platform/developer/api_explorer>. Run it
   manually before pushing changes that touch endpoint code or request
   shape. Fixtures created in the sandbox are cleaned up with Ginkgo's
   `DeferCleanup` — keep new tests read-only or self-cleaning.

## Architecture

- **Root package `geni`** (`http_client.go`) — only the façade.
  `NewClient(tokenSource, useSandboxEnv)` builds one `*transport.Client`
  and hands it to a client per resource, reached through accessors
  (`Client.Profile()`, `Union()`, `Document()`, `Photo()`, `Video()`,
  `PhotoAlbum()`, `Project()`, `Surname()`, `Revision()`, `Stats()`,
  `User()`, `Search()`, `Tree()`). It re-exports `ErrResourceNotFound`,
  `ErrAccessDenied` and `BaseURL`.
- **Resource packages** (`profile/`, `union/`, … `tree/`) — each exposes
  `NewClient(*transport.Client)`, its wire types and its endpoints, and
  builds `*http.Request`s for `transport` to send; none wires HTTP
  plumbing itself. `comment/` holds wire types only.
- **`transport/`** — the shared HTTP layer, below.
- **`auth/`** — optional OAuth `TokenSource`s: the client-side (implicit)
  flow `NewAuthTokenSource`, the server-side code flow `NewCodeTokenSource`
  (needs a client secret; yields refresh tokens), the loopback callback
  listener both use, and the file caches `NewCachingTokenSource` /
  `NewRefreshingCachingTokenSource`. Callers who already have a token pass
  any `oauth2.TokenSource` to `NewClient` and skip it.
- **`web/`** — structurally independent of the above: a cookie-auth client
  for the AJAX endpoints the geni.com site itself calls (`web.Client`), with
  one sub-package per feature (`conflicts`, `matches`, `treeconflicts`,
  `document`, `revision`, `relationships`, `unions`). `web.Client.Do`
  rate-limits to 1 rps, never follows redirects (a redirect to `/login`
  becomes `ErrNotLoggedIn`, an Incapsula page `ErrBlocked`), and
  `CSRFToken` scrapes and caches the form `authenticity_token`.
  `web/browsercookies` is opt-in: importing it pulls in `sweetcookie` and
  its browser backends. These endpoints are undocumented and may break.
- **`cmd/geni/`** — the CLI. A hand-rolled command tree (`commandTree()` in
  `commands.go`, dispatched by `run()` in `main.go`), one `flag.FlagSet` per
  leaf, JSON on stdout via `render`. `geni api` (`api.go`, `apifields.go`)
  is the raw console: any API endpoint through `transport.DoRaw`, or any
  geni.com path with `-web`.
- **`examples/`** — runnable examples (excluded from linters).

### `transport.Client` (`transport/client.go`)

Every API request funnels through the unexported `do`, reached through
`Do(ctx, req, coalescer)` (body only), `DoWithResponse` (body + headers)
or `DoRaw` (a non-retryable non-200 comes back as a `Response` carrying
`StatusCode` and the full body, instead of as an error). `do` owns the
cross-cutting behavior that is easy to break by accident:

1. **Auth + standard query params.** `addStandardHeadersAndQueryParams`
   (`auth.go`) always adds `access_token`, and adds `api_version` and
   `only_ids=true` unless the request already carries them (a repeated
   parameter loses: Rails reads the last one). `only_ids` makes responses
   reference other objects by id instead of URL; where Geni ignores it
   (e.g. `Profile.Unions`) the resource package strips the URLs after
   decoding (`profile.StripURLs`). `Content-Type: application/json` is set
   only when the caller hasn't set one — multipart uploads set their own.
2. **Rate limiting.** A `golang.org/x/time/rate.Limiter` starts at 1 rps
   and is **dynamically re-tuned** from each response's `X-API-Rate-Limit`
   / `X-API-Rate-Window` headers. Don't replace the limiter with a static
   one — the server's quota changes per token.
3. **Retries via `retry-go`**, 4 attempts, each bounded by a 60 s
   `requestTimeout`. `errRetry` covers 429, 401, 502/503/504 and transient
   transport errors (DNS not found, broken pipe, connection reset, HTTP/2
   stream resets, timeouts). An Incapsula block page is `errIncapsula`,
   with its own 45 s delay and at most 2 retries. 403 → `ErrAccessDenied`,
   404 → `ErrResourceNotFound`; anything else is an error carrying the body
   truncated to 512 bytes. The sentinels are public API, and errors come
   back wrapped in `retry.Error`, so callers must use `errors.Is`. Every
   attempt reuses the same request, rewinding its body through `GetBody` —
   build bodies from in-memory readers (`bytes.Buffer`, `bytes.Reader`,
   `strings.Reader`), or a retry cannot replay them.
4. **Bulk-read coalescing.** The singular `Get`s of Profile, Union,
   Document, Photo and Video pass a `transport.BulkCoalescer`
   (`coalesce.go`, implementing the `Coalescer` interface) to merge
   concurrent single-resource reads into one bulk call. While a request
   waits on the limiter, its key and cancel func sit in the transport's
   `urlMap`; the first to win the limiter sweeps the map, appends sibling
   ids to its `ids=` param, fans the bulk response back into the map, and
   cancels the siblings' waits so they pick up cached bodies. Callers see a
   plain `Get(id)`. On the singular path Geni answers a miss with an empty
   bulk envelope rather than 404; the coalescer maps that to
   `ErrResourceNotFound`.

### Mutation endpoints

Create/update methods JSON-encode their request struct (`profile.Request`,
`union.Request`, `document.Request`, …) and run the body through
`transport.EscapeStringToUTF`, which writes every non-ASCII rune as a
`\uXXXX` escape (a surrogate pair above U+FFFF). Geni's API has
historically mishandled raw UTF-8 in request bodies; the escape pass is a
workaround, not decoration — don't remove it. Escape the `json.Marshal`
output as is: the callers used to collapse `\\` to `\` first, which stored
`\t` as a tab and made `\o` invalid JSON (fixed in 1.31.1).

`profile.Request` and similar request structs use **unusual omitempty
choices on purpose** (e.g. `Title`, `Occupation`, `Suffix` are scalar
strings *without* `omitempty`). The Geni API treats `""` as a "clear
field" sentinel for these flat scalars but ignores omitted keys; and
`DetailStrings` *has* `omitempty` because `"detail_strings": null` makes
Geni answer 500. The comments on those fields explain the contract —
preserve it.

`profile.Client.WipeEventDates` (`profile/client.go`) is a targeted POST
to `<id>/update` that empties only the `date` sub-object of named events,
on a profile or a union. The API deep-merges nested objects per-key, so
sending `"end_month": null` inside an otherwise-populated `date` is
silently a no-op — the only way to clear individual date sub-fields is to
first wipe the whole `date` and then re-send the desired subset. Profile
honors both `"date": {}` and `"date": null`; union honors only
`"date": {}`. Issue #94 has context. Its sibling `WipeEvents` clears whole
events the same way (`{"date": {}, "location": {}}`; `"marriage": {}` is a
no-op and `null` is a 500).

### Sandbox vs production

`NewClient(tokenSource, useSandboxEnv bool)` — the second arg picks the
host. `BaseURL(bool)` is the request host (`https://www.geni.com/` or
`https://sandbox.geni.com/`); endpoints are `BaseURL + "api/<path>"`.
`APIURL(bool)` is the host Geni *prints* in response bodies — the same
`www.geni.com/api/` in production but `https://api.sandbox.geni.com/` in
the sandbox — and is used only to strip or map those URLs back, never to
send requests. Tests should run against sandbox; production calls cost
rate-limit budget against real users.

## Conventions worth knowing

- Module path is `github.com/dmalch/go-geni`. `go.mod` pins `go 1.26`.
- Logging is `log/slog` only — no `fmt.Println` / `log` package. The CLI
  writes its output through the `stdout`/`stderr` writers in `globalOpts`,
  never `os.Stdout` directly, so tests can capture it.
- `Id` was renamed to `ID` across the public API in the 1.0 reshape.
  `Guid`, `Url`, `Html`, `Json` keep their mixed-case spelling
  deliberately — only `Id` → `ID` was in scope. Don't "fix" the
  remaining acronyms.
- `CHANGELOG.md` has no "Unreleased" heading: work in progress goes under
  the next version number, in `### NEW` / `### CHANGED` / `### FIXED` /
  `### REMOVED` / `### NOTES` sections.
- Apache-2.0 licensed; library is **not endorsed by Geni.com**.
