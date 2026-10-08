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
`~/.genealogy/geni_token.json` (`geni_sandbox_token.json` for sandbox), mode
`0600`. Read commands also trigger this flow automatically on a cache miss, so
an explicit `login` is optional. `geni logout` deletes the cached token.

Auth resolution order:

1. `GENI_ACCESS_TOKEN` — if set, used directly (no browser, good for CI).
2. The cached token file, if present and unexpired.
3. The cached refresh token, if there is one (see below) — no browser.
4. The interactive browser flow.

The token cache is shared with the `terraform-provider-genealogy` provider.

### Logging in once instead of daily

Geni's tokens last 24 hours, and the default flow cannot renew them. Storing
your application's client secret switches `geni` to Geni's server-side flow,
which returns a refresh token and renews in the background:

```bash
geni config client-secret <secret>   # or export GENI_CLIENT_SECRET
geni login
geni token status                    # "refreshable": true
```

Running against your own registered application instead of the built-in one
needs both halves — `geni config client-id <id>` and the matching secret. The
secret is stored in `~/.genealogy/config.json` (mode `0600`) and `geni config
show` prints it as `(set)`.

`geni logout` only deletes the local cache; it does not revoke the token,
because Geni issues one access token per application and user, so revoking
would also sign out the Terraform provider sharing this cache.

`geni login -port N` moves the callback listener, but it is not a free choice:
it must match the Callback URL registered with your Geni application. That one
URL is the only thing deciding where Geni redirects — an application may
register exactly one, and the authorization request cannot carry a
`redirect_uri` of its own (Geni's WAF blocks any query parameter holding a
URL, and Geni rejects anything that gets past it as pointing to "a different
server"). So changing the port means editing the registration, and `-port` is
only useful to someone running their own.

## Commands

Run `geni help` for the full list.

| Command | Description |
| --- | --- |
| `geni login [-port N]` | Authenticate and cache an OAuth token |
| `geni logout` | Delete the cached OAuth token |
| `geni token print` | Print the access token, refreshing it if possible |
| `geni token status` | Report on the cached token without revealing it |
| `geni whoami` | Show the authenticated user |
| `geni stats` | Show platform-wide statistics |
| `geni help` | Show usage |
| `geni api [flags] <endpoint>` | Call any API endpoint — or, with `-web`, any geni.com path — and print the raw response, like `gh api` (see [Raw calls](#raw-calls-geni-api)) |
| `geni config show` | Print the persisted CLI config (`~/.genealogy/config.json`) as JSON, secrets redacted |
| `geni config browser <name\|"">` | Set or clear the persisted default for `-browser` (see [Cookie source](#cookie-source)) |
| `geni config client-id <id\|"">` | Set or clear the OAuth client id of your own Geni application |
| `geni config client-secret <secret\|"">` | Set or clear the OAuth client secret, enabling refreshable logins |
| `geni profile get <id>` / `geni profile get -guid <guid>` | Fetch a profile by `profile-NNN` id, or by bare guid with `-guid` (rewritten to the `profile-g<guid>` immutable-id form; the only API shape that resolves a guid). Single-get only — the bulk endpoint can't resolve guids. |
| `geni profile get-bulk <id...>` | Fetch multiple profiles by id |
| `geni profile search <name...>` | Search profiles by name (`-page N`) |
| `geni profile open <id\|guid>` | Open the profile's web page in the browser |
| `geni profile compare <id1> <id2>` | Field-by-field diff of two profiles |
| `geni profile merge [-yes] <keep-id> <dup-id>` | Merge one profile into another (destructive; prompts for confirmation) |
| `geni profile detach-union [-yes] <id\|guid> <union-id…>` | Detach a profile from one or more unions (AJAX, mutating — see [Web (AJAX) commands](#web-ajax-commands)) |
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
the prompt in scripts. `geni api` sends whatever request you give it, so a
non-GET call can change data too, and it does not ask first.

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
| `geni matches list [-collection X] [-filter Y] [-order Z] [-direction D] [-page N \| -all] [-limit N]` | List the merge-center matches (the OAuth API has no equivalent). Output is a JSON array of `{profile_guid, name, profile_url, lifespan_text, deceased, privacy, relationship, manager_name, manager_profile_url, updated_at_text, tree_match_count, record_match_count, smart_match_count, smart_match_value, tree_match_url, record_match_url, smart_match_url}`. The three `*_url` fields are the review links behind the row's match buttons, present only for the types with a non-zero count. `tree_match_url` stays on geni.com; `record_match_url` and `smart_match_url` are `/fwd/myheritage` hand-offs — those two match types are computed and reviewed by MyHeritage, and the count plus that link is everything Geni holds, so open them in a browser. `-collection` is one of `managed,relatives,followed,collaborators` (default: `managed`); `-filter` one of `tree,record,smart,free-record`; `-order` one of `name,relationship,manager,updated_at,matches`; `-direction` `asc\|desc`. `-all` paginates until exhausted; `-limit` caps total rows. Pipe through `jq` to filter further (e.g. `\| jq '.[] \| select(.tree_match_count + .record_match_count + .smart_match_count > 0)'`). |
| `geni matches for-profile [-group {new,requested,removed}] <id\|guid>` | Tree-match candidates for one profile (drills into a single row of `matches list`). **Tree matches only** — a profile whose pending matches are record or smart ones returns an empty `matches` array, which means "no tree matches", not "no matches"; use `matches list -filter=record` and its `record_match_url` for those. Output is JSON `{source, matches, total_text}`. `source` carries the looked-up profile's name, place, lifespan, immediate family, and manager. Each `matches[]` entry adds `compare_url` (the `/merge/compare/…` link for the merge UI) and `similar_profiles_count` (how many further candidates that match itself has). `-group` defaults to `new`; `requested` and `removed` show confirmed/dismissed matches respectively. |
| `geni matches reject [-yes] <source-id\|guid> <match-id\|guid>` | **Mutating.** Reject the pending match between two profiles (the "remove match" action in the merge center). The pair is symmetric — order does not matter. JSON output `{"status":"rejected","source":"…","match":"…"}`. Reversible: rejected matches move to the `removed` group, viewable via `geni matches for-profile -group removed <source>`. Prompts for a `y/N` confirmation unless `-yes` is passed. |
| `geni conflicts list [-page N \| -all] [-limit N]` | List profiles that still carry an **unresolved merge data conflict** — the field disagreements (names, dates, residence) Geni leaves after merging two profiles (the OAuth API has no equivalent). Output is a JSON array of `{profile_guid, name, profile_url, resolve_url, manager_name, updated_at_text}`. `-all` paginates until exhausted; `-limit` caps total rows. |
| `geni conflicts show <id\|guid>` | Show the conflicting fields for one profile. JSON `{profile_guid, has_conflict, fields}`; each `fields[]` entry is `{field, subject, primary_value, other_values}` — the surviving (primary) profile's value vs. the merged-in profiles' values. A profile with no outstanding conflict prints `has_conflict:false`. |
| `geni tree-conflicts list [-collection C] [-page N \| -all] [-limit N]` | List profiles with an **unresolved tree conflict** — the Merge Center's sibling of data conflicts, flagging profiles that may have gained duplicate close relatives after a merge (the OAuth API has no equivalent). Output is a JSON array of `{profile_id, name, profile_url, updated_by_name, updated_at_text, manager_name, tree_url}`. Read-only: tree conflicts have no programmatic resolution — `tree_url` is the "Open tree" link to review and fix the conflict by hand. `-collection` selects the viewing mode (`managed` (default)\|`relatives`\|`followed`\|`collaborators`); `-all` paginates until exhausted; `-limit` caps total rows. |
| `geni tree-conflicts show <profile-id>` | Inspect **one** profile's tree conflict (the `profile_id` from `tree-conflicts list`) and get the information to resolve it. Reads Geni's own tree-view analysis (`/flash/fetch_immediate_family`) and outputs JSON `{profile_id, focus, conflict_types, parent_union_count, partner_conflict, parent_unions[], duplicate_candidates[], suggested_actions[], has_conflict}`. The common case is **duplicate parents**: two parent unions whose fathers/mothers are the same person duplicated by a merge. `duplicate_candidates[]` groups those by role (`father`/`mother`/`spouse`) with each profile's `subtree_size` (how much tree hangs off it, from prune counts), and `suggested_actions[]` are ready-to-run `geni profile compare`/`merge` commands that keep the profile with the larger subtree. |
| `geni conflicts resolve [-yes] [-prefer-nonempty] [-pick field=col]… [-dry-run] <id\|guid>` | **Mutating.** Clear a profile's merge data conflict. The default keeps the surviving (primary) profile's value for every field (correct when the survivor is canonical). `-prefer-nonempty` instead keeps a merged-in value for any field the survivor left **blank** (preserves data an external contributor added). `-pick field=col` (repeatable) resolves a named field to an explicit column (`0` = primary, `1+` = a merged profile's value). `-dry-run` prints the choices that would be submitted without changing anything. JSON output `{"status":"resolved","profile":"…"}`. Prompts for a `y/N` confirmation unless `-yes` (or `-dry-run`) is passed; resolving an already-clean profile is a no-op. |
| `geni profile detach-union [-yes] <profile-id\|guid> <union-id> [<union-id>…]` | **Mutating.** Detach a profile from one or more unions (the "Удалить связь" / "remove relationships" action on the profile's edit_relationships page) by POSTing to `/profile_actions/delete_relationships`. Each `<union-id>` is a Geni **web union id** — the digits in a `remove_connection_<id>` checkbox on that page — accepted bare or `union-` prefixed. This detaches the profile; it does **not** delete the union (an emptied union becomes a harmless orphan), and re-attaching requires the web UI, so it is only weakly reversible. JSON output `{"status":"detached","profile":"…","unions":["…"]}`. Prompts for a `y/N` confirmation unless `-yes` is passed. |

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

## Raw calls (`geni api`)

`geni api <endpoint>` sends an arbitrary request and prints the response, for
the endpoints no command wraps yet or to see exactly what Geni returns. It
goes through the same layer as every other command: the cached token, the
rate limiter that re-tunes itself from Geni's `X-API-Rate-*` headers, and the
retries on 429, 401, transient 5xx and Incapsula blocks. The flags are
`gh api`'s.

| Flag | Meaning |
| --- | --- |
| `-X`, `-method <verb>` | HTTP method. Defaults to `GET`, or `POST` when fields or `-input` are given |
| `-f`, `-raw-field key=value` | A string parameter (repeatable) |
| `-F`, `-field key=value` | A typed parameter (repeatable): `true`, `false`, `null` and integers become JSON literals, `@file` reads a file, `@-` reads stdin |
| `-H`, `-header 'Name: value'` | A request header (repeatable) |
| `-input <file>` | Read the request body from a file, `-` for stdin. The fields then go in the query string |
| `-i`, `-include` | Print the status line and the response headers before the body |
| `-paginate` | Follow `next_page` and print every page, one JSON document after another (API `GET`s only) |
| `-web` | Call a geni.com path with the browser session instead of the OAuth API (see below) |

Flags may come before or after the endpoint.

**Endpoint.** Any of `profile-123`, `/profile-123/immediate-family`,
`api/profile/search?names=Smith`, or a full URL as Geni prints it —
`https://www.geni.com/api/…`, `https://sandbox.geni.com/api/…`, or the
`https://api.sandbox.geni.com/…` form of the sandbox's `next_page` links. A
URL on any other host is refused before anything is sent, so the token never
leaves geni.com; so is a production URL under `-sandbox` and the reverse,
rather than switching environments silently. An `access_token` already in the
URL is dropped and the current one added. `api_version=1` and `only_ids=true`
are added unless the endpoint sets them — `?only_ids=false` returns full URLs
instead of ids.

**Fields.** On a `GET`, or when `-input` supplies the body, fields go in the
query string as given — `names[first_name]=Ivan` is nested by Rails on Geni's
side. Otherwise they form a JSON object: `birth[date][year]=1900` nests into
objects and `nicknames[]=Vanya` appends to an array. A JSON body, from fields
or from `-input`, has every non-ASCII character written as a `\uXXXX` escape,
because Geni mishandles raw UTF-8 in request bodies.

**Output.** The body is printed as is, re-indented when it is JSON. A status
other than 200 prints the body too (Geni's error message is in it), then
`geni api: HTTP 404 Not Found` on stderr and exit code `1`.

### `-web`

With `-web` the endpoint is a geni.com path — `/list/data_conflicts`,
`merge/resolve/<guid>`, or a full `https://www.geni.com/…` URL — sent with the
browser session, exactly as the [Web (AJAX) commands](#web-ajax-commands)
are: the same [one-time consent](#one-time-consent), the same
[cookie source](#cookie-source), the same 1 request per second. The caveats
there apply in full.

- A non-GET request carries the page's CSRF token, both as the
  `authenticity_token` form field (unless you pass your own) and as the
  `X-CSRF-Token` header. Fields form a URL-encoded form, keys as given
  (`-f 'resolve[name]=__unchanged__'`).
- Redirects are not followed. A 3xx is success — geni.com answers a form POST
  with one — and the target is printed on stderr. A redirect to `/login` means
  the session has expired.
- Pages are HTML and are printed as is. Endpoints the site calls over AJAX
  often answer JSON only to `-H 'X-Requested-With: XMLHttpRequest'`.
- `-paginate` is not available: web pages have no `next_page`.

### Examples

```bash
geni api user                                   # who am I, raw
geni api -i profile-g6000000012102785219        # with status and rate-limit headers
geni api -X GET profile/search -f names="John Smith" -paginate | jq -s 'map(.results[]) | length'
geni api 'profile-1?only_ids=false'             # full URLs instead of ids
geni api profile-1/update -f occupation=Blacksmith -F 'birth[date][year]=1850'
geni api -web /list/data_conflicts | head       # an HTML page, with the browser session
```

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

Results are pretty-printed JSON on **stdout**, with two exceptions:
`geni document text get` prints the document's raw text body (it is the
artifact requested, not a record about it), and `geni api` prints whatever the
server sent — HTML for most `-web` paths, and the status line and headers
first with `-i`. Diagnostics and errors go to
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
