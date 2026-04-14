# withingy

`withingy` is a Go CLI for pulling Withings data from the terminal.

It was transplanted from the `whoopy` codebase, so it keeps the same overall shape:

- Cobra-based CLI
- XDG config and token storage
- `auth`, `diag`, `stats`, and resource subcommands
- JSON-first output with optional `--text`
- installable local binary via `make install`

## Install

```bash
brew tap totocaster/tap
brew install withingy
```

## Current command set

```bash
withingy auth login|status|logout
withingy activity list|today|view
withingy measures list
withingy sleep list|today|view
withingy weight list|today|latest
withingy workouts list|today|view|export
withingy stats daily --date YYYY-MM-DD
withingy diag [--text]
withingy version
```

## Output conventions

- `weight` and `measures` JSON timestamps are emitted as RFC3339 UTC values.
- Human-readable `--text` output renders timestamps in your local timezone.
- `weight list` defaults to the last 30 days when no explicit range is given.
- `today` shortcuts and `stats daily` with no `--date` use your local calendar day.

## Shell completions

Temporary session setup:

```bash
source <(withingy completion zsh)
```

Persisted setup examples:

```bash
withingy completion zsh > "${fpath[1]}/_withingy"
withingy completion bash > ~/.local/share/bash-completion/completions/withingy
withingy completion fish > ~/.config/fish/completions/withingy.fish
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
- `WITHINGY_DEBUG_AUTH_LOG` (`stderr` or a file path for redacted auth/debug events)

## Auth notes

The auth flow is Withings-specific, not WHOOP-compatible:

- browser authorization uses `account.withings.com/oauth2_user/authorize2`
- token exchange uses signed requests against `wbsapi.withings.net`
- refresh tokens rotate and are replaced on refresh
- auth/debug logging can be enabled with `WITHINGY_DEBUG_AUTH_LOG=/tmp/withingy-auth.log`

The current Withings docs reviewed during the transplant say loopback redirect URIs may be disallowed, but the CLI still preserves the same localhost callback/manual-paste workflow that `whoopy` used. If automatic callback auth fails, use manual mode and inspect [`docs/status.md`](docs/status.md) for the current caveats.

## Usage Overview

`withingy` defaults to JSON output. Add `--text` for readable tables and summaries. For everyday use, the main pattern is `today` for quick checks, `view YYYY-MM-DD` for a specific day, and `latest` for the most recent weigh-in.

### Common commands

| Command | Description |
| --- | --- |
| `withingy auth status` | Confirm tokens are present and see expiry/scopes. |
| `withingy weight latest [--text]` | Show the most recent weight entry. |
| `withingy weight today [--text]` | Show today's weight entries. |
| `withingy sleep today [--text]` | Show today's sleep summaries. |
| `withingy sleep view YYYY-MM-DD [--text]` | Show one sleep summary for a specific date. |
| `withingy activity today [--text]` | Show today's activity summary. |
| `withingy activity view YYYY-MM-DD [--text]` | Show one activity summary for a specific date. |
| `withingy stats daily --date YYYY-MM-DD [--text]` | Aggregate activity, sleep, and workouts for one day. |
| `withingy diag [--text]` | Show config, token, and API health information. |
| `withingy version` | Print the installed version/build info. |

For broader history and scripting:

- `list` commands accept `--start`, `--end`, `--limit`, and `--cursor`.
- Timestamps accept either RFC3339 or `YYYY-MM-DD`.
- `withingy completion <shell>` generates shell completions.

### Examples

```bash
# Check auth state
withingy auth status

# Latest weigh-in
withingy weight latest --text

# Today's weight entries
withingy weight today --text

# Today's sleep summary
withingy sleep today --text

# One sleep summary by date
withingy sleep view 2026-03-03 --text

# Today's activity
withingy activity today --text

# One activity summary by date
withingy activity view 2026-03-03 --text

# Quick sanity check when something looks wrong
withingy diag --text

# Daily dashboard
withingy stats daily --date 2026-03-03 --text
```

## Development

```bash
gofmt -w .
go test ./...
make install
```

The running migration ledger is in [`docs/status.md`](docs/status.md).
