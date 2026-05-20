## 1.1.0 (Unreleased)

- New `user.Client.Add` — implements Geni's `/user/add` endpoint, the
  last unimplemented resource endpoint. It creates a new Geni account
  and returns that account's fresh OAuth access token, which Geni
  delivers in the `X-API-OAuth-access_token` response header. New
  types: `user.AddRequest` (email / first_name / last_name / gender,
  all required by Geni) and `user.AddResult` (the created `*User`
  plus its `AccessToken`).
- New `transport.DoWithResponse` method and `transport.Response`
  type — a variant of `transport.Client.Do` that returns the response
  headers alongside the body, for endpoints whose contract carries
  data in a header. `Do`'s signature and behaviour are unchanged; it
  now delegates to the same shared core.

## 1.0.0

First stable release. The pre-1.0 reshape is complete: HTTP plumbing
lives in the `transport/` package, every Geni resource has its own
sub-package reached through a typed accessor on the root façade, and
`Id` is spelled `ID` across the public API. The exported surface is
now stable under semantic versioning. The breaking changes below are
the cumulative diff from `0.17.0`.

- Dependency diet (no API change). The `auth` package's OAuth
  callback server is rewritten on the standard library's `net/http`
  instead of `labstack/echo/v4`. This drops `echo` and its six
  exclusive transitive dependencies (`labstack/gommon`,
  `valyala/fasttemplate`, `valyala/bytebufferpool`,
  `mattn/go-colorable`, `mattn/go-isatty`, `golang.org/x/crypto`)
  from the module graph. `auth.NewAuthTokenSource` and its public
  behaviour are unchanged.
- Test reorganisation (no API change). The root package's
  cross-resource bulk-read tests (`bulk_test.go`,
  `bulk_fallback_test.go`) — each really a single-resource test
  stranded in root after the reshape — fold into the respective
  resource sub-packages, giving every resource a complete in-package
  `GetBulk` suite (single-id fallback + multi-id decode + 404/403
  mapping). The `BulkCoalescer` direct-unit test moves into the
  `transport` package where the type lives. Root now contains only
  the façade (`http_client.go`) plus the genuinely cross-resource
  coalescing integration test (`coalesce_test.go`).
- **BREAKING:** `Id` → `ID` across the public API. Every wire type's
  `Id` field is now `ID` (`profile.Profile.ID`, `union.Union.ID`,
  `document.Document.ID`, `photo.Photo.ID`, `video.Video.ID`,
  `project.Project.ID`, `surname.Surname.ID`, `revision.Revision.ID`,
  `user.User.ID`, `comment.Comment.ID`, `photoalbum.PhotoAlbum.ID`,
  `tree.PathRelation.ID`). The JSON wire tag stays `id` — only the Go
  field identifier changed. `Guid`, `Url`, `Html`, `Json` keep their
  mixed-case spelling deliberately; this rename covered `Id` only.

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
- **BREAKING:** Profile wire types lift into a new
  `github.com/dmalch/go-geni/profile` package. Profile *methods*
  (`GetProfile`, `CreateProfile`, …) still live on the root
  `geni.Client` for this PR — the type-only move unblocks the leaf
  sub-packages (surname, search, project, …) that take or return
  Profile-shaped values. Rename map:
  - `*geni.ProfileResponse`     → `*profile.Profile`
  - `*geni.ProfileRequest`      → `*profile.Request`
  - `*geni.ProfileBulkResponse` → `*profile.BulkResponse`
  - `geni.NameElement`          → `profile.NameElement`
  - `geni.EventElement`         → `profile.EventElement`
  - `geni.DateElement`          → `profile.DateElement`
  - `geni.LocationElement`      → `profile.LocationElement`
  - `geni.DetailsString`        → `profile.DetailsString`
- **BREAKING:** `geni.ResultResponse` → `transport.Result`. The
  generic `{"result":"OK"}` envelope returned by delete / tag /
  follow endpoints moves into the transport package so each
  resource doesn't redeclare it.
- **BREAKING:** Surname resource lifts into a new
  `github.com/dmalch/go-geni/surname` package.
  - `client.GetSurname(ctx, id)` → `client.Surname().Get(ctx, id)`
  - `client.GetSurnameFollowers(ctx, id, page)` → `client.Surname().Followers(ctx, id, page)`
  - `client.GetSurnameProfiles(ctx, id, page)` → `client.Surname().Profiles(ctx, id, page)`
  - `*geni.Surname` → `*surname.Surname`
  - `*geni.SurnameBulkResponse` → `*surname.BulkResponse` (still
    returned by `client.GetFollowedSurnames`, which stays on root
    until the user/ resource lifts later)
  - New helper `profile.StripURLs(p, apiURL)` lets sub-packages
    post-process Profile listings without depending on root.
- **BREAKING:** Revision resource lifts into a new
  `github.com/dmalch/go-geni/revision` package.
  - `client.GetRevision(ctx, id)`  → `client.Revision().Get(ctx, id)`
  - `client.GetRevisions(ctx, ids)` → `client.Revision().GetBulk(ctx, ids)`
  - `*geni.Revision`               → `*revision.Revision`
  - `*geni.RevisionBulkResponse`   → `*revision.BulkResponse`
  The single-id bulk fallback behaviour is preserved.
- **BREAKING:** SearchProfiles lifts into a new
  `github.com/dmalch/go-geni/search` package.
  - `client.SearchProfiles(ctx, names, page)` → `client.Search().Profiles(ctx, names, page)`
  Return type is unchanged (`*profile.BulkResponse`).
- New `profile.AddFields(req)` helper + `profile.FieldsQueryValue`
  constant: encapsulates the canonical `fields=` query param that
  every profile-returning endpoint sends. Root's
  `addProfileFieldsQueryParams` now delegates to it; sub-packages
  call `profile.AddFields(req)` directly.
- **BREAKING:** Wire types for Document, Photo, Video, PhotoAlbum,
  Project, Union, and Comment lift into per-resource sub-packages.
  Methods stay on root; the type-only move unblocks the user/
  resource (and the per-resource method PRs that follow). Rename
  map:
  - `*geni.DocumentResponse`     → `*document.Document`
  - `*geni.DocumentRequest`      → `*document.Request`
  - `*geni.DocumentBulkResponse` → `*document.BulkResponse`
  - `*geni.PhotoResponse`        → `*photo.Photo`
  - `*geni.PhotoRequest`         → `*photo.Request`
  - `*geni.PhotoBulkResponse`    → `*photo.BulkResponse`
  - `*geni.VideoResponse`        → `*video.Video`
  - `*geni.VideoRequest`         → `*video.Request`
  - `*geni.VideoBulkResponse`    → `*video.BulkResponse`
  - `*geni.PhotoAlbum`           → `*photoalbum.PhotoAlbum`
  - `*geni.PhotoAlbumRequest`    → `*photoalbum.Request`
  - `*geni.PhotoAlbumBulkResponse` → `*photoalbum.BulkResponse`
  - `*geni.ProjectResponse`      → `*project.Project`
  - `*geni.ProjectBulkResponse`  → `*project.BulkResponse`
  - `*geni.UnionResponse`        → `*union.Union`
  - `*geni.UnionRequest`         → `*union.Request`
  - `*geni.UnionBulkResponse`    → `*union.BulkResponse`
  - `geni.Comment`               → `comment.Comment`
  - `*geni.CommentBulkResponse`  → `*comment.BulkResponse`
- **BREAKING:** User resource (GetUser + the 10 v0.15.0 endpoint
  methods) lifts into a new `github.com/dmalch/go-geni/user`
  package. Root gains a `User() *user.Client` accessor.
  - `client.GetUser(ctx)`                  → `client.User().Get(ctx)`
  - `client.GetFollowedProfiles(ctx, p)`   → `client.User().FollowedProfiles(ctx, p)`
  - `client.GetFollowedDocuments(ctx, p)`  → `client.User().FollowedDocuments(ctx, p)`
  - `client.GetFollowedProjects(ctx, p)`   → `client.User().FollowedProjects(ctx, p)`
  - `client.GetFollowedSurnames(ctx, p)`   → `client.User().FollowedSurnames(ctx, p)`
  - `client.GetMaxFamily(ctx, p)`          → `client.User().MaxFamily(ctx, p)`
  - `client.GetUploadedPhotos(ctx, p)`     → `client.User().UploadedPhotos(ctx, p)`
  - `client.GetUploadedVideos(ctx, p)`     → `client.User().UploadedVideos(ctx, p)`
  - `client.GetMyAlbums(ctx, p)`           → `client.User().Albums(ctx, p)`
  - `client.GetMyLabels(ctx, p)`           → `client.User().Labels(ctx, p)`
  - `client.GetMetadata(ctx, ids...)`      → `client.User().Metadata(ctx, ids...)`
  - `client.UpdateMetadata(ctx, data)`     → `client.User().UpdateMetadata(ctx, data)`
  - `*geni.User`            → `*user.User`
  - `*geni.LabelsResponse`  → `*user.LabelsResponse` (renamed-and-relocated;
    the bare `Labels` name collides with Ginkgo's dot-imported
    `Labels` in test files, so the `Response` suffix stays)
  - `*geni.Metadata`        → `*user.Metadata`
  - `GetManagedProfiles` and `GetUploadedDocuments` stay on root
    for now — they live in `profile.go` / `document.go` and migrate
    when those resources lift later.
- **BREAKING:** Project resource methods lift into
  `github.com/dmalch/go-geni/project` (types already lifted in
  PR 7). Root gains `Project() *project.Client`.
  - `client.GetProject(ctx, id)`                 → `client.Project().Get(ctx, id)`
  - `client.GetProjectProfiles(ctx, id, p)`      → `client.Project().Profiles(ctx, id, p)`
  - `client.GetProjectCollaborators(ctx, id, p)` → `client.Project().Collaborators(ctx, id, p)`
  - `client.GetProjectFollowers(ctx, id, p)`     → `client.Project().Followers(ctx, id, p)`
  - `client.AddProfileToProject(ctx, pid, prj)`  → `client.Project().AddProfile(ctx, pid, prj)`
  - `client.AddDocumentToProject(ctx, did, prj)` → `client.Project().AddDocument(ctx, did, prj)`
- **BREAKING:** PhotoAlbum methods lift into
  `github.com/dmalch/go-geni/photoalbum` (types lifted in PR 7).
  Root gains `PhotoAlbum() *photoalbum.Client`. The `photoAlbumPath`
  helper that normalises `album-{n}` → `photo_album-{n}` moves
  alongside as `albumPath`.
  - `client.CreatePhotoAlbum(ctx, req)`        → `client.PhotoAlbum().Create(ctx, req)`
  - `client.GetPhotoAlbum(ctx, id)`            → `client.PhotoAlbum().Get(ctx, id)`
  - `client.GetPhotoAlbumPhotos(ctx, id, p)`   → `client.PhotoAlbum().Photos(ctx, id, p)`
  - `client.UpdatePhotoAlbum(ctx, id, req)`    → `client.PhotoAlbum().Update(ctx, id, req)`
- **BREAKING:** Photo resource methods (10 of them) lift into
  `github.com/dmalch/go-geni/photo` (types lifted in PR 7). Root
  gains `Photo() *photo.Client`. The Photo coalescer call site
  moves with it.
  - `client.CreatePhoto(ctx, t, fn, r, opts...)` → `client.Photo().Create(ctx, t, fn, r, opts...)`
  - `client.GetPhoto(ctx, id)`                  → `client.Photo().Get(ctx, id)`
  - `client.GetPhotos(ctx, ids)`                → `client.Photo().GetBulk(ctx, ids)`
  - `client.UpdatePhoto(ctx, id, req)`          → `client.Photo().Update(ctx, id, req)`
  - `client.DeletePhoto(ctx, id)`               → `client.Photo().Delete(ctx, id)`
  - `client.TagPhoto(ctx, pid, profileId)`      → `client.Photo().Tag(ctx, pid, profileId)`
  - `client.UntagPhoto(ctx, pid, profileId)`    → `client.Photo().Untag(ctx, pid, profileId)`
  - `client.GetPhotoTags(ctx, pid, p)`          → `client.Photo().Tags(ctx, pid, p)`
  - `client.GetPhotoComments(ctx, pid, p)`      → `client.Photo().Comments(ctx, pid, p)`
  - `client.AddPhotoComment(ctx, pid, t, ttl)`  → `client.Photo().AddComment(ctx, pid, t, ttl)`
  - `geni.CreatePhotoOption`                    → `photo.CreateOption`
  - `geni.WithPhotoAlbum(id)`                   → `photo.WithAlbum(id)`
  - `geni.WithPhotoDescription(d)`              → `photo.WithDescription(d)`
  - `geni.WithPhotoDate(d)`                     → `photo.WithDate(d)`
- **BREAKING:** Video resource methods (10 of them) lift into
  `github.com/dmalch/go-geni/video` (types lifted in PR 7). Root
  gains `Video() *video.Client`. The Video coalescer call site
  moves with it.
  - `client.CreateVideo(ctx, t, fn, r, opts...)` → `client.Video().Create(ctx, t, fn, r, opts...)`
  - `client.GetVideo(ctx, id)`                  → `client.Video().Get(ctx, id)`
  - `client.GetVideos(ctx, ids)`                → `client.Video().GetBulk(ctx, ids)`
  - `client.UpdateVideo(ctx, id, req)`          → `client.Video().Update(ctx, id, req)`
  - `client.DeleteVideo(ctx, id)`               → `client.Video().Delete(ctx, id)`
  - `client.TagVideo(ctx, vid, pid)`            → `client.Video().Tag(ctx, vid, pid)`
  - `client.UntagVideo(ctx, vid, pid)`          → `client.Video().Untag(ctx, vid, pid)`
  - `client.GetVideoTags(ctx, vid, p)`          → `client.Video().Tags(ctx, vid, p)`
  - `client.GetVideoComments(ctx, vid, p)`      → `client.Video().Comments(ctx, vid, p)`
  - `client.AddVideoComment(ctx, vid, t, ttl)`  → `client.Video().AddComment(ctx, vid, t, ttl)`
  - `geni.CreateVideoOption`                    → `video.CreateOption`
  - `geni.WithVideoDescription(d)`              → `video.WithDescription(d)`
  - `geni.WithVideoDate(d)`                     → `video.WithDate(d)`
  The transitional root helpers `errInvalidArg` (errors.go) and
  `readMultipart` (multipart_test.go) added in PR 11 are removed —
  `errInvalidArg` now lives only inside `photo/` and `video/`, and
  `readMultipart` lives inside each package's tests.
- **BREAKING:** Document resource methods (11 of them) lift into
  `github.com/dmalch/go-geni/document` (types lifted in PR 7). Root
  gains `Document() *document.Client`. The Document coalescer call
  site moves with it.
  - `client.CreateDocument(ctx, req)`              → `client.Document().Create(ctx, req)`
  - `client.GetDocument(ctx, id)`                  → `client.Document().Get(ctx, id)`
  - `client.GetDocuments(ctx, ids)`                → `client.Document().GetBulk(ctx, ids)`
  - `client.UpdateDocument(ctx, id, req)`          → `client.Document().Update(ctx, id, req)`
  - `client.DeleteDocument(ctx, id)`               → `client.Document().Delete(ctx, id)`
  - `client.TagDocument(ctx, did, pid)`            → `client.Document().Tag(ctx, did, pid)`
  - `client.UntagDocument(ctx, did, pid)`          → `client.Document().Untag(ctx, did, pid)`
  - `client.GetDocumentTags(ctx, did, p)`          → `client.Document().Tags(ctx, did, p)`
  - `client.GetDocumentComments(ctx, did, p)`      → `client.Document().Comments(ctx, did, p)`
  - `client.AddDocumentComment(ctx, did, t, ttl)`  → `client.Document().AddComment(ctx, did, t, ttl)`
  - `client.GetDocumentProjects(ctx, did, p)`      → `client.Document().Projects(ctx, did, p)`
- **BREAKING:** `GetUploadedDocuments` moves from root to the user
  resource (it's user-scoped — `/api/user/uploaded-documents`):
  - `client.GetUploadedDocuments(ctx, page)` → `client.User().UploadedDocuments(ctx, page)`
- **BREAKING:** `AddDocumentToProject` moves to the document resource
  to break a circular import between document/ and project/:
  - `client.Project().AddDocument(ctx, docId, projectId)` → `client.Document().AddToProject(ctx, docId, projectId)`
- **BREAKING:** Union resource methods (5 of them) lift into
  `github.com/dmalch/go-geni/union` (types lifted in PR 7). Root
  gains `Union() *union.Client`. The Union coalescer call site
  moves with it.
  - `client.GetUnion(ctx, id)`              → `client.Union().Get(ctx, id)`
  - `client.GetUnions(ctx, ids)`            → `client.Union().GetBulk(ctx, ids)`
  - `client.UpdateUnion(ctx, id, req)`      → `client.Union().Update(ctx, id, req)`
  - `client.AddPartnerToUnion(ctx, id)`     → `client.Union().AddPartner(ctx, id)`
  - `client.AddChildToUnion(ctx, id, ...)`  → `client.Union().AddChild(ctx, id, ...)`
- **BREAKING:** The `AddOption` type and `WithModifier` constructor
  (shared by profile relationship-adds and union adds) move from
  the root package into `profile`:
  - `geni.AddOption`         → `profile.AddOption`
  - `geni.WithModifier(m)`   → `profile.WithModifier(m)`
- **BREAKING:** Tree traversal (the family-graph endpoints) lifts
  into a new `github.com/dmalch/go-geni/tree` package — types,
  methods, and options. Root gains `Tree() *tree.Client`.
  - `client.GetImmediateFamily(ctx, id)`    → `client.Tree().ImmediateFamily(ctx, id)`
  - `client.GetAncestors(ctx, id, ...)`     → `client.Tree().Ancestors(ctx, id, ...)`
  - `client.GetPathTo(ctx, from, to, ...)`  → `client.Tree().PathTo(ctx, from, to, ...)`
  - `*geni.FamilyResponse` → `*tree.FamilyResponse`; `geni.FamilyNodes`
    → `tree.FamilyNodes`; `*geni.PathToResponse` → `*tree.PathToResponse`;
    `geni.PathRelation` → `tree.PathRelation`
  - `geni.PathType` / `PathStatus` (and their constants) → `tree.PathType`
    / `tree.PathStatus`
  - `geni.TreeOption` → `tree.Option`
  - `geni.WithGenerations` / `WithPathType` / `WithRefresh` /
    `WithSearch` / `WithSkipEmail` / `WithSkipNotify` → the same names
    under `tree`
  `Client.CompareProfiles` still lives on root (Profile resource) but
  its `ProfileComparison.Results` field is now `[]tree.FamilyResponse`.
- **BREAKING — final reshape PR.** The Profile resource lifts into
  `github.com/dmalch/go-geni/profile` and the root `geni` package
  collapses to a pure façade: `NewClient` + 13 resource accessors +
  `BaseURL` + the `ErrResourceNotFound` / `ErrAccessDenied`
  re-exports. No endpoint methods remain on `geni.Client` directly.
  - 14 pure-profile methods → `profile.Client`:
    - `client.CreateProfile`  → `client.Profile().Create`
    - `client.GetProfile`     → `client.Profile().Get`
    - `client.GetProfiles`    → `client.Profile().GetBulk`
    - `client.UpdateProfile`  → `client.Profile().Update`
    - `client.UpdateProfileBasics` → `client.Profile().UpdateBasics`
    - `client.DeleteProfile`  → `client.Profile().Delete`
    - `client.AddPartner`     → `client.Profile().AddPartner`
    - `client.AddChild`       → `client.Profile().AddChild`
    - `client.AddSibling`     → `client.Profile().AddSibling`
    - `client.AddParent`      → `client.Profile().AddParent`
    - `client.MergeProfiles`  → `client.Profile().Merge`
    - `client.FollowProfile`  → `client.Profile().Follow`
    - `client.UnfollowProfile`→ `client.Profile().Unfollow`
    - `client.WipeEventDates` → `client.Profile().WipeEventDates`
  - 8 methods that return a non-profile resource cross-place onto
    that resource (so `profile/` stays foundational — imported by
    every package, importing only `transport/`):
    - `client.GetProfileDocuments` → `client.Document().ForProfile`
    - `client.GetProfilePhotos`    → `client.Photo().ForProfile`
    - `client.AddProfilePhoto`     → `client.Photo().AddToProfile`
    - `client.AddProfileVideo`     → `client.Video().AddToProfile`
    - `client.AddProfileDocument`  → `client.Document().AddToProfile`
    - `client.AddProfileMugshot`   → `client.Photo().AddMugshotToProfile`
      (`geni.MugshotRequest` → `photo.MugshotRequest`)
    - `client.CompareProfiles`     → `client.Tree().Compare`
      (`*geni.ProfileComparison` → `*tree.Comparison`)
    - `client.GetManagedProfiles`  → `client.User().ManagedProfiles`
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
