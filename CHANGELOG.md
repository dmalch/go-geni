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
