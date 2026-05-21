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
| `geni union get <id>` | Fetch a union |
| `geni union get-bulk <id...>` | Fetch multiple unions by id |
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
| `geni revision get <id>` | Fetch a revision |
| `geni revision get-bulk <id...>` | Fetch multiple revisions by id |
| `geni tree family <id>` | Immediate family of a profile |
| `geni tree ancestors <id>` | Ancestors of a profile (`-generations N`) |

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

# Sandbox
geni -sandbox whoami
geni -sandbox profile get profile-1

# Scripted (no browser)
GENI_ACCESS_TOKEN=<token> geni whoami
```
