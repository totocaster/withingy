# withingy

`withingy` is a Go CLI for pulling Withings data from the terminal.

It was transplanted from the `whoopy` codebase, so it keeps the same overall shape:

- Cobra-based CLI
- XDG config and token storage
- `auth`, `diag`, `stats`, and resource subcommands
- root-level `--hpx` export for Hypercontext NDJSON
- JSON-first output with optional `--text`
- installable local binary via `make install`

## Install

```bash
brew tap totocaster/tap
brew install --HEAD totocaster/tap/withingy
```

Until the first tagged release is published, the tap formula is HEAD-only and
builds from the current `main` branch. After the first release, plain
`brew install withingy` will work.

## Current command set

```bash
withingy auth login|status|logout
withingy --hpx [--since ... --until ... --last ... --limit ...]
withingy activity list|today|view
withingy measures list
withingy sleep list|today|view
withingy weight list|latest
withingy workouts list|today|view|export
withingy stats daily --date YYYY-MM-DD
withingy diag [--text]
withingy version
```

## Config

Config and tokens live under `${XDG_CONFIG_HOME:-~/.config}/withingy/`.

Example:

```toml
client_id = "..."
client_secret = "..."
api_base_url = "https://wbsapi.withings.net"
oauth_base_url = "https://account.withings.com"
redirect_uri = "http://127.0.0.1:8735/oauth/callback"
scopes = "user.metrics,user.activity"
```

Environment overrides:

- `WITHINGY_CLIENT_ID`
- `WITHINGY_CLIENT_SECRET`
- `WITHINGY_API_BASE_URL`
- `WITHINGY_OAUTH_BASE_URL`
- `WITHINGY_REDIRECT_URI`
- `WITHINGY_SCOPES`
- `WITHINGY_CONFIG_DIR`

## Auth notes

The auth flow is Withings-specific, not WHOOP-compatible:

- browser authorization uses `account.withings.com/oauth2_user/authorize2`
- token exchange uses signed requests against `wbsapi.withings.net`
- refresh tokens rotate and are replaced on refresh

The current Withings docs reviewed during the transplant say loopback redirect URIs may be disallowed, but the CLI still preserves the same localhost callback/manual-paste workflow that `whoopy` used. If automatic callback auth fails, use manual mode and inspect [`docs/status.md`](docs/status.md) for the current caveats.

## Hypercontext export

`withingy --hpx` emits canonical Hypercontext NDJSON to stdout.

Examples:

```bash
withingy --hpx | hpx import
withingy --hpx --last 30d | hpx import
withingy --hpx --since 2026-03-01 --until 2026-03-07 | hpx import
```

The exporter currently emits:

- body metrics for supported Withings measure groups
- sleep signposts plus session metrics
- workout signposts plus session metrics
- summary documents for daily activity and unsupported measurement groups

## Development

```bash
gofmt -w .
go test ./...
make install
```

The running migration ledger is in [`docs/status.md`](docs/status.md).
