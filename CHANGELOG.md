## 0.18.0 (Unreleased)

- **Internal restructuring — first PRs of the pre-1.0 reshape.**
  HTTP plumbing (auth, rate limiting, retry, error sentinels, bulk
  coalescing, the `escapeStringToUTF` helper) moves out of the root
  package into a new `github.com/dmalch/go-geni/transport` package.
  Subsequent PRs lift each resource (Profile, Union, Document, …)
  into its own sub-package. The root `Client` becomes a façade —
  each lifted resource is exposed via a typed accessor (e.g.
  `client.Stats() *stats.Client`).
- **BREAKING:** `Client.GetStats` and the `StatsResponse` type are
  removed. The `/stats` endpoint now lives in
  `github.com/dmalch/go-geni/stats`. Callers update from
  `client.GetStats(ctx)` to `client.Stats().Get(ctx)`, and
  `*geni.StatsResponse` becomes `*stats.Response`.
- `bulkCoalescer[Item, Envelope]` renamed to
  `transport.BulkCoalescer[Item, Envelope]` with exported fields
  (`CurrentID`, `IDPrefix`, `DecodeBulk`, `ListResults`,
  `IDOfResult`) and methods (`RequestKey`, `PrepareBulkRequest`,
  `ParseBulkResponse`). The Coalescer interface in `transport` is
  the new contract for opt-in bulk-read coalescing.
- Field rename: `Id` → `ID` on the new `BulkCoalescer` type only
  (per the 1.0 acronym policy: `Id` → `ID` allowed; `Url`, `Guid`,
  etc. stay as-is). The wire-bearing response types (`ProfileResponse`,
  `UnionResponse`, …) keep `Id` until their respective sub-package
  PRs.

## 0.17.0

- Photo Album API: four new methods completing the resource.
  - `Client.CreatePhotoAlbum(ctx, *PhotoAlbumRequest)` →
    `*PhotoAlbum` — POST `/api/photo_album/add`.
  - `Client.GetPhotoAlbum(ctx, albumId)` → `*PhotoAlbum`.
  - `Client.GetPhotoAlbumPhotos(ctx, albumId, page)` →
    `*PhotoBulkResponse` — paginated photos in an album.
  - `Client.UpdatePhotoAlbum(ctx, albumId, *PhotoAlbumRequest)` →
    `*PhotoAlbum`.
- New `PhotoAlbumRequest` type (name + description). The existing
  `PhotoAlbum` type (introduced in v0.15.0 as the result type of
  `GetMyAlbums`) gained `CoverPhoto map[string]string` and
  `PhotosCount int` fields.
- Sandbox finding documented in `photoAlbumPath`: Geni returns
  photo-album ids as `album-{n}` but the URL path requires
  `photo_album-{n}` — bare `album-` paths return a 500
  ApiException (`"No action responded to album-{n}"`). The client
  normalises the prefix when constructing URLs so callers pass
  whichever form they received from the API.

## 0.16.0

- Document API: added `Client.GetDocumentTags(ctx, documentId, page)`
  — the read counterpart to the existing `TagDocument` /
  `UntagDocument` write verbs. Returns a paginated
  `ProfileBulkResponse`. Symmetric with `Client.GetPhotoTags`.

## 0.15.0

- User API: ten new endpoint methods rounding out the user-scoped
  surface:
  - `GetFollowedProfiles`, `GetFollowedDocuments`,
    `GetFollowedProjects`, `GetFollowedSurnames` — the four
    `/user/followed-*` listings.
  - `GetMaxFamily` — `/user/max-family`, paginated profile list.
  - `GetUploadedPhotos`, `GetUploadedVideos` — symmetric with
    the existing `GetUploadedDocuments`.
  - `GetMyAlbums`, `GetMyLabels` — `/user/my-*` listings.
  - `GetMetadata`, `UpdateMetadata` — application-specific
    key/value store, opaque to the client.
- New types: `Metadata` (with `Data json.RawMessage`), `PhotoAlbum`
  + `PhotoAlbumBulkResponse`, `LabelsResponse`,
  `SurnameBulkResponse`.
- Sandbox finding documented in `UpdateMetadata` godoc: Geni's
  `/user/update-metadata` expects the `data` field as a JSON-encoded
  *string*, not a nested object — sending a nested object returns
  a 500 ApiException (`"no implicit conversion of
  ActionController::Parameters into String"`). The client now
  serialises the supplied `RawMessage` into a string before
  sending.
- Not implemented (deferred): `/user/add`. The endpoint returns the
  OAuth access token in an `X-API-OAuth-access_token` response
  header, but `Client.doRequest` only exposes body bytes today. A
  later change can plumb a header-extraction path through
  `doRequest` and wire it up.

## 0.14.0

- Profile API: nine new endpoint methods rounding out the
  small-profile-action surface:
  - `Client.FollowProfile(ctx, id)` / `Client.UnfollowProfile(ctx, id)`
    — both return the targeted `ProfileResponse`.
  - `Client.CompareProfiles(ctx, id1, id2)` — fetches immediate-family
    graphs for both profiles in one call. New `ProfileComparison`
    type wraps `[]FamilyResponse` (two entries, one per profile).
  - `Client.AddParent(ctx, id, *ProfileRequest, opts...)` — creates
    and attaches a new parent profile. Accepts `WithModifier` for
    adopt / foster relationships, completing the family-add
    quartet alongside `AddPartner` / `AddChild` / `AddSibling`.
  - `Client.UpdateProfileBasics(ctx, id, *ProfileRequest)` — narrower
    target than `UpdateProfile`, scoped to the basics/about fields.
  - `Client.AddProfilePhoto(ctx, id, *PhotoRequest)` /
    `Client.AddProfileVideo(ctx, id, *VideoRequest)` /
    `Client.AddProfileDocument(ctx, id, *DocumentRequest)` —
    JSON-body media-add endpoints (file is Base64-encoded in the
    request, distinct from the multipart `/photo/add` and `/video/add`
    paths). `PhotoRequest.File` and `VideoRequest.File` gained the
    optional Base64 string for this purpose.
  - `Client.AddProfileMugshot(ctx, id, *MugshotRequest)` — sets a
    profile's mugshot. New `MugshotRequest` type accepts either
    `File` (Base64 upload) or `PhotoId` (reuse an existing photo).

## 0.13.0

- Bulk-read coalescing extended to all single-fetchable resources:
  concurrent `GetUnion`, `GetDocument`, `GetPhoto`, and `GetVideo`
  calls now collapse into a single bulk request the same way
  `GetProfile` always has. The implementation is a generic
  `bulkCoalescer[Item, Envelope]` in `coalesce.go` that owns the
  request-key / prepare / parse-response triple for every resource
  type. `GetProfile` was refactored onto the same helper — same
  behavior, less duplication.
- New concurrency tests (`coalesce_test.go`) verify the wire-level
  collapse: 4 concurrent reads of a single resource type collapse
  into fewer HTTP requests, and a concurrent read across two
  different resource types stays on two distinct URL paths.

## 0.12.0

- Surname API: added `Client.GetSurname(ctx, surnameId)`,
  `Client.GetSurnameFollowers(ctx, surnameId, page)`, and
  `Client.GetSurnameProfiles(ctx, surnameId, page)`. New `Surname`
  type (id, description, slugged_name, url). The followers and
  profiles sub-listings return the existing `ProfileBulkResponse`.
- Revision API: added `Client.GetRevision(ctx, revisionId)` and
  `Client.GetRevisions(ctx, revisionIds)` with the same single-id
  bulk fallback used by Profile / Document / Union / Photo. New
  `Revision` and `RevisionBulkResponse` types covering the
  documented fields (id, guid, action, date_local, time_local,
  timestamp, story).
- Stats API: added `Client.GetStats(ctx)`. New `StatsResponse` type;
  individual entries are kept as `json.RawMessage` because Geni's
  docs don't enumerate per-stat fields.

## 0.11.0

- Video API: full resource mirror of Photo. New methods:
  `Client.CreateVideo`, `Client.GetVideo`, `Client.GetVideos`,
  `Client.UpdateVideo`, `Client.DeleteVideo`, `Client.TagVideo`,
  `Client.UntagVideo`, `Client.GetVideoTags`, `Client.AddVideoComment`,
  `Client.GetVideoComments`. New types: `VideoResponse`,
  `VideoRequest`, `VideoBulkResponse`, plus `CreateVideoOption`
  with `WithVideoDescription` / `WithVideoDate`. Reuses the
  Photo-introduced multipart plumbing and the single-id bulk
  fallback from v0.10.0.
- Sandbox finding documented in `CreateVideo` godoc: Geni's
  /video/add docs say `file` is optional, but the sandbox 400s on
  metadata-only requests and runs uploads through ffmpeg validation
  — arbitrary byte payloads return 500 "Could not get the duration".
  Sandbox specs Skip cleanly without a real video fixture.

## 0.10.0

- Bulk-by-id single-id fallback: `Client.GetProfiles`,
  `Client.GetDocuments`, `Client.GetUnions`, and `Client.GetPhotos`
  now route single-element calls through the corresponding singular
  `Get*` and wrap the result in the bulk envelope. Reason: Geni's
  bulk-by-id dispatcher silently returns `results: []` when `ids`
  carries exactly one identifier — the server appears to route
  one-element bulk requests through a search/filter path rather than
  a fetch-by-id path. Callers see consistent behaviour regardless of
  input size; no API surface change.
- Sandbox acceptance suite: tightened several specs that were
  previously Skip()-on-relaxed-assertion. Eventually polling now
  asserts:
  - `GetDocuments` bulk eventually returns both ids (the bulk path,
    via 2-id call).
  - `GetImmediateFamily` eventually surfaces a child added via
    `AddChild`.
  - `GetPathTo` eventually settles on a terminal `PathStatus`
    (non-Pending) for a real parent→child path.
  - `GetUnions` for a single id now hits the new fallback and passes
    without skipping.
  Specs that genuinely don't propagate within a reasonable window
  in the sandbox (comments listings, search index, profile media
  listings) are now Skip()'d with the intended Eventually
  assertion preserved — flip one Skip line to re-arm.

## 0.9.0

- Profile API: added `Client.GetProfileDocuments(ctx, profileId, page)`
  and `Client.GetProfilePhotos(ctx, profileId, page)` for the
  paginated `/profile/<id>/documents` and `/profile/<id>/photos`
  sub-listings. Both return existing bulk envelopes
  (`DocumentBulkResponse` and `PhotoBulkResponse`) and respect Geni's
  max-50-per-page cap. `profile/videos` is deferred until the Video
  resource lands.
- `DocumentBulkResponse` and `PhotoBulkResponse` now carry `NextPage`
  / `PrevPage` so paginated listings can be walked the same way
  `ProfileBulkResponse` and `ProjectBulkResponse` already support.

## 0.8.0

- User API: added `Client.GetUser(ctx)` for the `GET /api/user`
  self-endpoint. New `User` type covering the documented fields
  (`Id`, `Guid`, `Name`, `AccountType`); the JSON decoder is
  permissive so any extra fields Geni surfaces are silently dropped.

## 0.7.0

- Photo API: added the minimum viable surface for image uploads —
  `Client.CreatePhoto(ctx, title, fileName, file, opts...)` with
  `WithPhotoAlbum`, `WithPhotoDescription`, `WithPhotoDate` options;
  `Client.GetPhoto(ctx, photoId)`; `Client.GetPhotos(ctx, photoIds)`
  for bulk; `Client.DeletePhoto(ctx, photoId)`. New `PhotoResponse`
  and `PhotoBulkResponse` types covering the documented Geni Photo
  resource (id, title, description, album_id, sizes map, location,
  tags, timestamps).
- Multipart plumbing: `addStandardHeadersAndQueryParams` no longer
  overwrites a caller-set `Content-Type` header, so endpoints that
  build `multipart/form-data` bodies (the new `CreatePhoto`, plus any
  future photo/video upload endpoints) can pre-set their boundary
  header and have it survive intact through `doRequest`.
- Photo API: completed the resource. Added `Client.UpdatePhoto(ctx,
  photoId, *PhotoRequest)` plus a new `PhotoRequest` type;
  `Client.TagPhoto` / `Client.UntagPhoto` (path-based
  `/photo-{id}/tag/profile-{id}`); `Client.GetPhotoTags(ctx, photoId,
  page)` returning a paginated `ProfileBulkResponse`;
  `Client.AddPhotoComment(ctx, photoId, text, title)` and
  `Client.GetPhotoComments(ctx, photoId, page)` returning the shared
  `CommentBulkResponse`.

## 0.6.0

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
