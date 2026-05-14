# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`github.com/dmalch/go-geni` — Go client for the Geni.com genealogy API.
Extracted from `terraform-provider-genealogy` v0.20.1's `internal/geni/`
so the same HTTP layer is reusable from CLI tools and migration scripts.
Pre-1.0 (`0.1.x`); see `CHANGELOG.md` for the deferred ID/URL casing
cleanup planned for 1.0.

## Commands

```bash
make build                               # go build ./...
make test                                # unit + Ginkgo integration (in-process)
make lint                                # golangci-lint
make test-acceptance                     # E2E against sandbox; needs GENI_ACCESS_TOKEN
make check                               # build + vet + lint + test (CI parity)

go test -run TestProfile ./...           # single test by name
go test -run TestFoo/subtest ./...       # subtest of a table-driven test
```

CI (`.github/workflows/ci.yaml`) runs build + test + vet + golangci-lint
on every push/PR to `main`. Keep the working tree warning-free under the
enabled linters (errcheck, staticcheck, unused, unparam, godot, …). CI
does **not** run `test/acceptance/` — those need a real OAuth token,
which the harness can't mint.

Tests come in three tiers:

1. **Unit** — plain `testing.T` + Gomega matchers, in-package, using the
   `captureTransport` / `fakeTransport` round-trippers (`*_test.go` at
   the repo root).
2. **Integration** — Ginkgo v2 BDD specs registered into the single
   `TestGeniIntegration` bootstrap (`suite_test.go`,
   `*_integration_test.go`). Still in-process; talks to an
   `httptest.NewServer` serving fixtures from `testdata/`.
3. **Sandbox E2E** (`test/acceptance/`, `package acceptance`) —
   self-skips unless `GENI_ACCESS_TOKEN` is exported. Mint a sandbox
   token at
   <https://sandbox.geni.com/platform/developer/api_explorer> and run
   `make test-acceptance` manually before pushing changes that touch
   endpoint code or request shape. Fixtures created in the sandbox are
   cleaned up with `t.Cleanup` — keep new tests read-only or
   self-cleaning.

## Architecture

The package is a single flat Go package (`package geni`) at the repo root,
with two siblings:

- `auth/` — optional OAuth implicit-flow `TokenSource` + file-backed
  caching `TokenSource`. Callers who already have a token can skip this
  entirely and pass any `oauth2.TokenSource` to `NewClient`.
- `examples/getprofile/` — runnable example (excluded from linters).

### `Client` and `doRequest` (`http_client.go`)

Every endpoint method funnels through `Client.doRequest`. That function
owns the cross-cutting behavior that is easy to break by accident:

1. **Auth + standard query params.** `addStandardHeadersAndQueryParams`
   injects `access_token` (from the `oauth2.TokenSource`), `api_version`,
   and `only_ids=true` (forces ID-only references in responses instead of
   URLs — `fixResponse` strips any URLs the API returns anyway when
   `only_ids` is not honored, e.g. for `Unions`).
2. **Rate limiting.** A `golang.org/x/time/rate.Limiter` starts at 1 rps
   and is **dynamically re-tuned** from each response's `X-API-Rate-Limit`
   / `X-API-Rate-Window` headers. Don't replace the limiter with a static
   one — the server's quota changes per token.
3. **Retries via `retry-go`.** Retries fire only for `errCode429WithRetry`
   (the internal sentinel). 429, 401, transient transport errors (DNS not
   found, broken pipe, connection reset, timeouts) are converted into this
   sentinel and retried up to 4 attempts. 403 → `ErrAccessDenied`, 404 →
   `ErrResourceNotFound`, Incapsula block-pages → distinct error. These
   exported sentinels (`ErrResourceNotFound`, `ErrAccessDenied`) are part
   of the public API — callers `errors.Is` against them.
4. **Bulk-read coalescing.** `GetProfile` (and similarly the document/union
   `Get`s) uses the unexported `withRequestKey` / `withPrepareBulkRequest`
   / `withParseBulkResponse` options to merge concurrent single-resource
   reads into one bulk call. Mechanism: while a request waits on the rate
   limiter, its key + cancel func is stored in `Client.urlMap`; the first
   request to win the limiter sweeps the map, appends sibling IDs to its
   `ids=` param, fans the bulk response back out into the map, and cancels
   the siblings' limiter waits so they pick up cached bodies. Callers see
   plain `GetProfile(id)` — coalescing is invisible. Don't export the
   `with*` options; they're internal and were deliberately unexported
   pre-1.0 (see CHANGELOG).

### Mutation endpoints

`CreateProfile`, `UpdateProfile`, `UpdateUnion`, `CreateDocument`,
`UpdateDocument` JSON-encode the request struct, then run it through
`escapeStringToUTF` to convert every non-ASCII rune to `\uXXXX`. Geni's
API has historically mishandled raw UTF-8 in request bodies; the escape
pass is a workaround, not decoration — don't remove it.

`ProfileRequest` and similar request structs use **unusual omitempty
choices on purpose** (e.g. `Title`, `Occupation`, `Suffix` are scalar
strings *without* `omitempty`). The Geni API treats `""` as a "clear
field" sentinel for these flat scalars but ignores omitted keys. The
comments on those fields explain the contract — preserve it.

`WipeEventDates` (`profile.go`) is a targeted PATCH that nulls only the
`date` sub-object of named events. The API deep-merges nested objects
per-key, so sending `"end_month": null` inside an otherwise-populated
`date` is silently a no-op — the only way to clear individual date
sub-fields is to first wipe the whole `date` and then re-PATCH the
desired subset. Profile honors both `"date": {}` and `"date": null`;
union honors only `"date": {}`. Issue #94 has context.

### Sandbox vs production

`NewClient(tokenSource, useSandboxEnv bool)` — the second arg picks the
host (`sandbox.geni.com` vs `www.geni.com`). `BaseURL(bool)` is exported
for callers; the internal `apiUrl` helper differs (api host) and is used
when stripping URL prefixes in `fixResponse`. Tests should run against
sandbox; production calls cost rate-limit budget against real users.

## Conventions worth knowing

- Module path is `github.com/dmalch/go-geni`. `go.mod` pins `go 1.26`.
- Logging is `log/slog` only — no `fmt.Println` / `log` package.
- Identifiers like `Id`, `Guid`, `Url` violate Go's all-caps acronym rule
  but are kept until 1.0 to keep the parallel provider migration
  tractable (see CHANGELOG). Don't bulk-rename in feature PRs; that's
  reserved for the 1.0 cleanup with deprecation aliases.
- Apache-2.0 licensed; library is **not endorsed by Geni.com**.
