# geni

A command-line client for the [Geni.com](https://www.geni.com) genealogy API —
a thin façade over the [`go-geni`](../../) library. `geni login` runs a
browser-based OAuth handshake and caches the token; the read commands print
JSON to stdout.

## Install

```bash
go install github.com/dmalch/go-geni/cmd/geni@latest
```

The binary lands in `$(go env GOPATH)/bin` — make sure that's on your `PATH`.

## Quick start

```bash
geni login                            # browser OAuth; caches the token
geni whoami                           # the authenticated account
geni profile get profile-122248213    # fetch a profile as JSON
geni profile get -guid 6000000000000   # ...or by bare guid
geni profile open profile-122248213   # open the profile's web page
```

## Authentication

`geni login` opens your browser for Geni's OAuth flow and caches the token at
`~/.genealogy/geni_token.json` (`geni_sandbox_token.json` for sandbox). Read
commands also trigger this flow automatically on a cache miss, so an explicit
`login` is optional. `geni logout` deletes the cached token.

Auth resolution order:

1. `GENI_ACCESS_TOKEN` — if set, used directly (no browser, good for CI).
2. The cached token file, if present and unexpired.
3. The interactive browser flow.

The token cache is shared with the `terraform-provider-genealogy` provider.

## Commands

Run `geni help` for the full list.

| Command | Description |
| --- | --- |
| `geni login` | Authenticate and cache an OAuth token |
| `geni logout` | Delete the cached OAuth token |
| `geni whoami` | Show the authenticated user |
| `geni stats` | Show platform-wide statistics |
| `geni help` | Show usage |
| `geni config show` | Print the persisted CLI config (`~/.genealogy/config.json`) as JSON |
| `geni config browser <name\|"">` | Set or clear the persisted default for `-browser` (see [Cookie source](#cookie-source)) |
| `geni profile get <id>` / `geni profile get -guid <guid>` | Fetch a profile by `profile-NNN` id, or by bare guid with `-guid` (rewritten to the `profile-g<guid>` immutable-id form; the only API shape that resolves a guid). Single-get only — the bulk endpoint can't resolve guids. |
| `geni profile get-bulk <id...>` | Fetch multiple profiles by id |
| `geni profile search <name...>` | Search profiles by name (`-page N`) |
| `geni profile open <id\|guid>` | Open the profile's web page in the browser |
| `geni profile compare <id1> <id2>` | Field-by-field diff of two profiles |
| `geni profile merge [-yes] <keep-id> <dup-id>` | Merge one profile into another (destructive; prompts for confirmation) |
| `geni union get <id>` | Fetch a union |
| `geni union get-bulk <id...>` | Fetch multiple unions by id |
| `geni document for-profile [-page N] <profile-id>` | List documents attached to a profile |
| `geni document get <id>` | Fetch a document |
| `geni document get-bulk <id...>` | Fetch multiple documents by id |
| `geni document open <id\|guid>` | Open the document's web page in the browser |
| `geni document text get <id\|guid>` | Print a document's text body — raw, **not** JSON (AJAX; see [Web (AJAX) commands](#web-ajax-commands)) |
| `geni document text set [-from-file <p>] <id\|guid>` | Replace a document's text body from stdin or `-from-file`; no-op when the body already matches (AJAX) |
| `geni photo get <id>` | Fetch a photo |
| `geni photo get-bulk <id...>` | Fetch multiple photos by id |
| `geni video get <id>` | Fetch a video |
| `geni video get-bulk <id...>` | Fetch multiple videos by id |
| `geni photoalbum get <id>` | Fetch a photo album |
| `geni project get <id>` | Fetch a project |
| `geni surname get <id>` | Fetch a surname |
| `geni revision for-profile <id\|guid>` | List a profile's revision IDs (AJAX; one-time consent prompt — see [Web (AJAX) commands](#web-ajax-commands)) |
| `geni revision get <id>` | Fetch a revision |
| `geni revision get-bulk <id...>` | Fetch multiple revisions by id |
| `geni matches list [flags]` | List profiles with pending tree/record/smart matches in the merge center (AJAX — see [Web (AJAX) commands](#web-ajax-commands)) |
| `geni tree family <id>` | Immediate family of a profile |
| `geni tree ancestors <id>` | Ancestors of a profile (`-generations N`) |

Every command is read-only except **`geni profile merge`**, which mutates
data. It prompts for a `y/N` confirmation before merging; pass `-yes` to skip
the prompt in scripts.

## Web (AJAX) commands

A few CLI commands talk to Geni's **private AJAX endpoints** instead of the
official OAuth API — they cover gaps the OAuth API doesn't address (e.g. the
revision-history list). These endpoints are undocumented, unsupported by
Geni.com, may break without notice, and using them may violate geni.com's
Terms of Service.

| Command | Description |
| --- | --- |
| `geni revision for-profile <id\|guid>` | List a profile's revision IDs (cross over to the OAuth API with `geni revision get revision-<id>` for the body of each) |
| `geni document text get <id\|guid>` | Print a document's text body. **Raw text on stdout, not JSON** — the OAuth API can't read this field. Use redirection (`> body.txt`) to capture. |
| `geni document text set [-from-file <p>] <id\|guid>` | Replace a document's text body. New body comes from `-from-file` or stdin. The command first fetches the current body and skips the POST if it already matches (after stripping `\r` and per-line trailing whitespace). JSON output: `{"status":"updated"\|"unchanged","guid":"…","bytes_written":N}`. |
| `geni matches list [-collection X] [-filter Y] [-order Z] [-direction D] [-page N \| -all] [-limit N]` | List the merge-center matches (the OAuth API has no equivalent). Output is a JSON array of `{profile_guid, name, profile_url, lifespan_text, deceased, privacy, relationship, manager_name, manager_profile_url, updated_at_text, tree_match_count, record_match_count, smart_match_count, smart_match_value}`. `-collection` is one of `managed,relatives,followed,collaborators` (default: `managed`); `-filter` one of `tree,record,smart`; `-order` one of `name,relationship,manager,updated_at,matches`; `-direction` `asc\|desc`. `-all` paginates until exhausted; `-limit` caps total rows. Pipe through `jq` to filter further (e.g. `\| jq '.[] \| select(.tree_match_count + .record_match_count + .smart_match_count > 0)'`). |
| `geni matches for-profile [-group {new,requested,removed}] <id\|guid>` | Tree-match candidates for one profile (drills into a single row of `matches list`). Output is JSON `{source, matches, total_text}`. `source` carries the looked-up profile's name, place, lifespan, immediate family, and manager. Each `matches[]` entry adds `compare_url` (the `/merge/compare/…` link for the merge UI) and `similar_profiles_count` (how many further candidates that match itself has). `-group` defaults to `new`; `requested` and `removed` show confirmed/dismissed matches respectively. |
| `geni matches reject [-yes] <source-id\|guid> <match-id\|guid>` | **Mutating.** Reject the pending match between two profiles (the "remove match" action in the merge center). The pair is symmetric — order does not matter. JSON output `{"status":"rejected","source":"…","match":"…"}`. Reversible: rejected matches move to the `removed` group, viewable via `geni matches for-profile -group removed <source>`. Prompts for a `y/N` confirmation unless `-yes` is passed. |
| `geni conflicts list [-page N \| -all] [-limit N]` | List profiles that still carry an **unresolved merge data conflict** — the field disagreements (names, dates, residence) Geni leaves after merging two profiles (the OAuth API has no equivalent). Output is a JSON array of `{profile_guid, name, profile_url, resolve_url, manager_name, updated_at_text}`. `-all` paginates until exhausted; `-limit` caps total rows. |
| `geni conflicts show <id\|guid>` | Show the conflicting fields for one profile. JSON `{profile_guid, has_conflict, fields}`; each `fields[]` entry is `{field, subject, primary_value, other_values}` — the surviving (primary) profile's value vs. the merged-in profiles' values. A profile with no outstanding conflict prints `has_conflict:false`. |
| `geni conflicts resolve [-yes] [-prefer-nonempty] [-pick field=col]… [-dry-run] <id\|guid>` | **Mutating.** Clear a profile's merge data conflict. The default keeps the surviving (primary) profile's value for every field (correct when the survivor is canonical). `-prefer-nonempty` instead keeps a merged-in value for any field the survivor left **blank** (preserves data an external contributor added). `-pick field=col` (repeatable) resolves a named field to an explicit column (`0` = primary, `1+` = a merged profile's value). `-dry-run` prints the choices that would be submitted without changing anything. JSON output `{"status":"resolved","profile":"…"}`. Prompts for a `y/N` confirmation unless `-yes` (or `-dry-run`) is passed; resolving an already-clean profile is a no-op. |

### One-time consent

The first AJAX command in a session prints a disclaimer and asks
`Accept and continue? [y/N]`. On `y` the answer is recorded in
`~/.genealogy/web_consent.json` and future invocations skip the prompt.
Delete that file to revoke the consent. For scripted use,
`GENI_WEB_CONSENT=accepted` bypasses the prompt without writing the file.

### Cookie source

AJAX commands need a logged-in geni.com session. The CLI tries, in order:

1. **`GENI_WEB_COOKIES`** env var (explicit override) — the value of the
   `Cookie` header copied from a logged-in browser's DevTools.
2. The host's installed browsers (Chrome, Firefox, Safari, Edge, Brave, …)
   via [`steipete/sweetcookie`](https://github.com/steipete/sweetcookie) —
   `geni` reads valid, non-expired geni.com cookies straight from the
   browser cookie store.

By default every backend is tried in sweetcookie's priority order
(Chrome → Edge → Brave → Arc → Chromium → Vivaldi → Opera → Firefox →
Safari). To pin to one browser there are three layers, checked in
priority order:

1. **`-browser=<name>`** global flag — per-invocation override.
2. **`GENI_WEB_BROWSER`** env var — automation override.
3. **`~/.genealogy/config.json`** persisted preference — set once
   with `geni config browser <name>`; cleared with
   `geni config browser ""`. Survives across invocations.

Accepted values: `chrome,edge,brave,arc,chromium,vivaldi,opera,firefox,safari`.

Inspect the persisted config with `geni config show`.

On macOS, reading Safari's cookies requires Full Disk Access for your
terminal in System Settings → Privacy & Security. If neither source yields
cookies, the error message tells you which step failed.

## Flags

- **`-sandbox`** — global flag, placed **before** the command
  (`geni -sandbox whoami`). Targets `sandbox.geni.com` instead of production;
  also enabled by `GENI_USE_SANDBOX=true`.
- **`-browser`** — global flag, placed **before** the command
  (`geni -browser=safari matches list`). Limits AJAX cookie reads to one
  backend. Accepts `chrome,edge,brave,arc,chromium,vivaldi,opera,firefox,safari`;
  empty (default) tries every browser. Also settable via `GENI_WEB_BROWSER`.
- Per-command flags go **after** the command —
  `geni profile search -page 2 Smith`, `geni tree ancestors -generations 3 <id>`.

## Output

Results are pretty-printed JSON on **stdout**, with one exception:
`geni document text get` prints the document's raw text body (it is the
artifact requested, not a record about it). Diagnostics and errors go to
**stderr**. stdout stays pure JSON, so it pipes cleanly:

```bash
geni profile get profile-122248213 | jq -r .guid
```

Exit codes: `0` success, `1` command error, `2` usage error.

## Examples

```bash
# Production
geni profile get profile-6000000012102785219
geni profile search -page 2 "John Smith"
geni tree ancestors -generations 4 profile-122248213

# Bulk fetch — ids space- or comma-separated, prints {"results":[…]}
geni profile get-bulk profile-1 profile-2 profile-3
geni document get-bulk document-1,document-2 | jq '.results[].title'

# Vet a suspected duplicate before merging
geni profile compare profile-1 profile-2 | jq '.summary'

# Sandbox
geni -sandbox whoami
geni -sandbox profile get profile-1

# Scripted (no browser)
GENI_ACCESS_TOKEN=<token> geni whoami
```
