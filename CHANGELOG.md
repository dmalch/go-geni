## 0.6.0 (Unreleased)

- Project API: added `Client.GetProjectProfiles(ctx, projectId, page)`,
  `Client.GetProjectCollaborators(ctx, projectId, page)`, and
  `Client.GetProjectFollowers(ctx, projectId, page)` for the
  paginated `/project/<id>/{profiles,collaborators,followers}`
  sub-listings. All three return a `ProfileBulkResponse` (with the
  existing `Page` / `TotalCount` / `NextPage` / `PrevPage` fields).

## 0.5.0

- Document API: added `Client.GetDocumentComments(ctx, docId, page)`,
  `Client.AddDocumentComment(ctx, docId, text, title)`, and
  `Client.GetDocumentProjects(ctx, docId, page)`.
- New `Comment` and `CommentBulkResponse` types covering the
  paginated comment envelope shared with `*/comment` endpoints.
- `ProjectBulkResponse` now also carries `Page` / `NextPage` /
  `PrevPage` so paginated project lists can be walked.

## 0.4.0

- Union API: added `Client.AddPartnerToUnion(ctx, unionId)` and
  `Client.AddChildToUnion(ctx, unionId, opts...)` for the
  union-scoped add-* variants. Both return the **new profile** even
  though Geni's docs page describes the response as a union — the
  live endpoint returns a `ProfileResponse`, so the godoc and
  signatures match observed behaviour. `AddChildToUnion` accepts
  `WithModifier("adopt" | "foster")` to drop the new child into the
  union's `adopted_children` / `foster_children` list.

## 0.3.0

- Discovery: added `Client.SearchProfiles(ctx, names, page)` for
  Geni's `/profile/search` endpoint. Returns the existing
  `ProfileBulkResponse`, which now also carries the `NextPage` /
  `PrevPage` URL hints surfaced by paginated endpoints.

## 0.2.0

- Tree API: added `Client.GetImmediateFamily`, `Client.GetAncestors`, and
  `Client.GetPathTo`. New types: `FamilyResponse` / `FamilyNodes` (with
  `Profile`, `Union`, `ProfileIds`, `UnionIds` accessors) for the
  heterogeneous profile+union node map keyed by Geni-prefixed id; typed
  `PathType` and `PathStatus` value objects; `TreeOption` functional
  options (`WithGenerations`, `WithPathType`, `WithRefresh`, `WithSearch`,
  `WithSkipEmail`, `WithSkipNotify`). `GetPathTo` is a thin pass-through
  — when `Status == PathStatusPending` the caller is expected to back
  off and re-issue.
- `ProfileRequest.DetailStrings` now uses `omitempty`. A nil map is no
  longer serialised as `"detail_strings": null` — Geni's update endpoint
  crashes on that body (`undefined method has_key? for nil:NilClass`).
  Callers who want to clear all detail strings must send an explicit
  empty map.
- Tests: adopted Ginkgo + Gomega for the acceptance suite alongside the
  existing `testing.T` + Gomega unit tests. Added a sandbox E2E suite
  under `test/acceptance/` (opt-in via `make test-acceptance`; CI does
  not run it).

## 0.1.1

- Bumped `golang.org/x/oauth2` from 0.31.0 to 0.36.0 (#1).

## 0.1.0

Initial release. Extracted from
[`terraform-provider-genealogy`](https://github.com/dmalch/terraform-provider-genealogy)
`v0.20.1`'s `internal/geni/` package, plus the OAuth helper formerly at
`internal/authn/` (now `auth/`).

Public surface:

- `geni.Client` — HTTP client for `api.geni.com` (production) or
  `api.sandbox.geni.com` (sandbox).
- Profile API: `CreateProfile`, `GetProfile`, `GetProfiles`,
  `GetManagedProfiles`, `UpdateProfile`, `DeleteProfile`, `AddPartner`,
  `AddChild`, `AddSibling`, `MergeProfiles`, `WipeEventDates`.
- Union API: `GetUnion`, `GetUnions`, `UpdateUnion`.
- Document API: `CreateDocument`, `GetDocument`, `GetDocuments`,
  `GetUploadedDocuments`, `UpdateDocument`, `DeleteDocument`, `TagDocument`,
  `UntagDocument`, `AddDocumentToProject`.
- Project API: `GetProject`, `AddProfileToProject`.
- Request/response types covering every field these endpoints accept and
  return.
- `auth` subpackage: browser-based OAuth implicit-flow `TokenSource` plus a
  file-backed caching `TokenSource`.

Pre-1.0 cleanups applied during extraction:

- `BaseUrl` renamed to `BaseURL` to follow Go's all-caps acronym convention.
- The `WithRequestKey` / `WithPrepareBulkRequest` / `WithParseBulkResponse`
  functional options have been unexported. They were never consumable from
  outside the package (the `*opt` argument type is unexported) and are an
  internal mechanism for the bulk-read coalescing used by the package's own
  `Get*` methods.

**Known pre-1.0 style debt to address before tagging `v1.0.0`:** several
struct fields use Go-anti-pattern casing (`Id`, `Guid`, `Url` rather than `ID`,
`GUID`, `URL`). Renames were deferred from this release to keep the parallel
provider migration tractable. A follow-up minor release will apply them, with
deprecation aliases where feasible.

---

Notes for migrators from `terraform-provider-genealogy`'s `internal/geni`:

- Logging switched from `terraform-plugin-log/tflog` to stdlib `log/slog` —
  configure your own slog handler at process start to control output.
- Package import path changed from `…/terraform-provider-genealogy/internal/geni`
  to `github.com/dmalch/go-geni`; the OAuth helper moved from
  `…/internal/authn` to `github.com/dmalch/go-geni/auth`.
- No other API changes — all exported names, signatures, and request/response
  field tags are byte-for-byte identical.
