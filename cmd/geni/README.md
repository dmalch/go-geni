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
| `geni profile get <id>` | Fetch a profile by id |
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

On macOS, reading Safari's cookies requires Full Disk Access for your
terminal in System Settings → Privacy & Security. If neither source yields
cookies, the error message tells you which step failed.

## Flags

- **`-sandbox`** — global flag, placed **before** the command
  (`geni -sandbox whoami`). Targets `sandbox.geni.com` instead of production;
  also enabled by `GENI_USE_SANDBOX=true`.
- Per-command flags go **after** the command —
  `geni profile search -page 2 Smith`, `geni tree ancestors -generations 3 <id>`.

## Output

Results are pretty-printed JSON on **stdout**; diagnostics and errors go to
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
