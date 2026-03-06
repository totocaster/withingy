# withingy Transplant Status

Last updated: 2026-03-07

## Goal

Transplant the existing `whoopy` Go CLI into `withingy`, preserving the same project shape and operator UX where possible while swapping WHOOP-specific auth and data access for Withings equivalents.

## Completed

- Copied the `whoopy` repository structure into `withingy`.
- Renamed the project identity to `withingy`:
  - module path: `github.com/toto/withingy`
  - binary: `withingy`
  - env vars: `WITHINGY_*`
  - XDG config path: `${XDG_CONFIG_HOME:-~/.config}/withingy`
- Removed the stale copied build artifact from `bin/`.
- Replaced the auth flow foundation:
  - browser auth URL now targets `https://account.withings.com/oauth2_user/authorize2`
  - token exchange and refresh now use signed requests against `https://wbsapi.withings.net/v2/oauth2`
  - nonce acquisition uses `https://wbsapi.withings.net/v2/signature`
  - refresh token rotation is handled by overwriting the stored token file
  - logout currently clears the local token cache only
- Added Withings config defaults:
  - `api_base_url = "https://wbsapi.withings.net"`
  - `oauth_base_url = "https://account.withings.com"`
  - `scopes = "user.metrics,user.activity"`
- Added Withings-native CLI/resource layers:
  - `activity list|today|view`
  - `measures list`
  - `sleep list|today|view`
  - `weight list|latest`
  - `workouts list|today|view|export`
  - `stats daily`
  - `diag`
- Added root-level Hypercontext provider export:
  - `withingy --hpx`
  - bounded export flags: `--since`, `--until`, `--last`, `--limit`
  - emits canonical Hypercontext NDJSON for supported body, sleep, and workout data
  - emits `summary` documents for daily activity and unsupported measurement groups so richer Withings data still imports cleanly
- Reworked diagnostics to probe the Withings activity endpoint instead of the old WHOOP profile endpoint.
- Rewrote the root `README.md` to describe the current Withings-based CLI.
- Wrote the local machine config at `/Users/toto/.config/withingy/config.toml` using the credentials provided by the user.
- Ran formatting and verification:
  - `gofmt -w .`
  - `go test ./...`
  - `make install`
  - `/Users/toto/.local/bin/withingy version`
  - `/Users/toto/.local/bin/withingy diag --text`
  - `/Users/toto/.local/bin/withingy auth login --manual --no-browser` (smoke-tested through authorization URL generation only)

## Current command set

```text
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

## Important implementation notes

- This repo no longer exposes the transplanted `profile`, `cycles`, or `recovery` CLI commands because they do not map cleanly to the public Withings API reviewed during this pass.
- The old `internal/cycles`, `internal/profile`, and `internal/recovery` packages still exist in the tree as transplant leftovers, but they are no longer part of the active CLI surface.
- The new `workouts view` command treats the workout ID as the workout `startdate` Unix timestamp because the reviewed Withings docs did not expose a clean WHOOP-style single-workout endpoint.
- The new `sleep view` and `activity view` commands use `YYYY-MM-DD` identifiers.
- The `--hpx` exporter maps only data that exists in the current Hypercontext schema registry:
  - body metrics: `body.weight_kg`, `body.fat_pct`
  - sleep signposts and session metrics
  - workout signposts and session metrics
  - daily activity and unsupported body measurements fall back to `summary` documents with structured `meta`
- The `--hpx` exporter emits UTC timestamps and deterministic `origin_id` values for repeated imports.

## Known risks / gaps

- The current Withings docs reviewed during the transplant say loopback redirect URIs may be disallowed, but the CLI still preserves the localhost callback/manual-paste flow used by `whoopy`.
- No live OAuth completion was run in this pass because that requires interactive user authorization.
- The manual auth path successfully emits a Withings authorization URL from the installed binary, but the full callback/token-exchange path is still unverified.
- Some new Withings response models were inferred from the official docs and historical endpoint behavior rather than generated from a first-party OpenAPI schema.
- The current `--hpx` exporter does not expose `--updated-since`; bounded exports are time-window based rather than source-native update-cursor based.
- The current Hypercontext schema does not register daily activity keys such as steps or distance, so that data is exported as `summary` documents instead of metrics.
- The README and `docs/status.md` are current, but the older transplant carry-over docs under `docs/` are still historical and should be treated as non-authoritative unless updated.

## Recommended next steps

- Run `withingy auth login` and complete a real token exchange.
- Once authenticated, capture a few real payloads for:
  - `activity list --text`
  - `sleep list --text`
  - `workouts list --text`
- Validate `withingy --hpx` against a live Hypercontext importer run:
  - `withingy --hpx --last 30d | hpx import --dry-run`
  - `withingy --hpx --last 30d | hpx import`
  - repeat the same import to confirm idempotency
- Tighten the data models and add targeted tests based on real Withings payloads.
- Decide whether to keep the current Withings-native command surface or add aliases for users coming from `whoopy`.

## Sources used

- Source project:
  - `/Users/toto/Developer/whoopy`
- Official Withings docs reviewed:
  - https://developer.withings.com/developer-guide/v3/integration-guide/public-health-data-api/get-access/oauth-authorization-url
  - https://developer.withings.com/developer-guide/v3/integration-guide/public-health-data-api/get-access/sign-your-requests
  - https://developer.withings.com/developer-guide/v3/integration-guide/public-health-data-api/get-access/access-and-refresh-tokens-no-recover
  - https://developer.withings.com/developer-guide/v3/integration-guide/advanced-research-api/faq/
  - https://developer.withings.com/developer-guide/v3/integration-guide/advanced-research-api/glossary/glossary-page
  - https://developer.withings.com/developer-guide/v3/integration-guide/public-health-data-api/data-api/fetch-data-example
- Official sample repository cross-check:
  - https://github.com/withings-sas/api-oauth2-python
