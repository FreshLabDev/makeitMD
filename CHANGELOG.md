# Changelog

All notable makeitMD changes are documented here.

The `## <tag>` section of this file *is* the GitHub Release body: the release
workflow copies it verbatim and refuses a tag that has no section. Write it
for whoever has to decide whether to upgrade.

See [`docs/versioning.md`](docs/versioning.md) for what the numbers mean and
[`docs/releases.md`](docs/releases.md) for how a release is published.

## Unreleased

Use this section for changes that are merged but not released yet.

### Changed

- One versioning and release document for the whole family. `docs/versioning.md`
  and `docs/releases.md` are now byte-identical across every Asterfield
  repository apart from two clearly marked sections: this repository's own
  version line, and the surface where a change here breaks something. They spell
  out what each of the three numbers means, what the `-alpha.N` suffix counts,
  when alpha becomes beta and when it is legitimate to skip to rc or run a
  pre-release in production.
- **Pre-releases are now tagged on `dev`, not `main`.** Only stable versions are
  tagged on `main`, on the merge commit from `dev`. `release.yml` had no branch
  check at all before, so a tag pushed from any branch would publish; it now
  refuses a tag that is not on the branch its channel is published from.
  Earlier pre-releases were tagged on `main` under the previous rule; they are
  left as they are.

### Changed

- `github.com/FreshLabDev/tg` moves to `v0.0.1-alpha.7`. It carries one fix:
  a preflight probe is marked as a probe, so the `404 method not found` it
  expects stops being counted and logged as a transport failure. All four bots
  on the shared client now run the same version.

### Added

- `deploy/ws04/compose.yaml`, the production stack, pulling the image the
  release workflow publishes to GHCR. The stack on the host built its own image
  from a working copy, so what served users was not the artifact CI had tested,
  scanned and published, and nothing on the host could say which commit it came
  from. `MAKEITMD_IMAGE` has no default: an unset one stops the stack instead of
  quietly starting something else.

## v0.1.1-alpha.3 - 2026-09-08

Review fixes, all of them in how failures are handled.

### Fixed

- The bot's own fixed strings are sent with no parse mode. One of them is the
  message a person gets when their Markdown failed to render; as HTML, a stray
  angle bracket in that sentence would fail its own delivery, and the person
  would get nothing at all.
- `makeitmd_telegram_rate_limits_total` counts what it says again. It is
  documented as requests still limited after a retry, and a second counter had
  been added on every HTTP attempt, including `getUpdates` polls that never
  touched it before.
- Through the client: an `ok=false` answer carried on a 2xx is an API error
  again, so the retry with normalized Markdown, the failure record and the
  reply to the user all happen instead of the conversion silently stalling as
  `received`; the startup preflight waits through a booting server rather than
  exiting on its first 502; and the bot token can no longer reach a log line or
  the conversion record through an error body echoed by a proxy.

## v0.1.1-alpha.1 - 2026-09-08

An internal change with one visible consequence: makeitMD now refuses to start
against a Telegram server that cannot render Markdown for it.

### Changed

- Telegram goes through `github.com/FreshLabDev/tg`, the client shared by the
  bot family, and `internal/telegram` is gone. The transport was the same code
  three bots each carried, and the copies had drifted. What makeitMD needs and
  the others do not -- the raw payloads it stores, Telegram's own entity
  detection, message entities -- became part of the shared client rather than
  a reason to keep a private one.
- Domain types moved out of the transport package: `Result` and
  `DeliveryAttempt` now live in `internal/db`, which is what they describe.
  A paste stitched from several Telegram messages is its own type in
  `internal/bot`, instead of an extra field on a protocol struct.
- Conversion records store a message the way Telegram sends it. Re-encoding
  used to add a null for every field the client models and the message did not
  have; `telegram_input.combined` is now the message and nothing else.

### Security

- The Go floor moves to 1.26.6 (build image included) and `golang.org/x/text` to v0.39.0. Eight
  advisories were outstanding, two of them reachable: one from every polling
  loop through `net/http`, one from opening the database through pgx. Nothing
  had run govulncheck here since July, so the first CI run in two months found
  them all at once.

### Added

- A preflight at startup. makeitMD names `sendRichMessage` as the method it
  cannot work without and does not start when the Bot API server lacks it. A
  server behind the bot answers `404 method not found` to every rich message,
  which used to mean a bot that polled happily and answered nothing.

### Operations

- No configuration changes. The bot now fails fast on a stale Bot API server
  instead of running mute, so a first start after a server downgrade will
  crash-loop with the missing method named in the log.

## v0.1.0 - 2026-07-13

First stable public release, promoted after three production release candidates
and live verification on WS04.

### Highlights

- Renders the user's exact source text through Telegram Bot API 10.1 Rich
  Markdown without a custom parser, AI, buttons, or settings.
- Keeps `/start` as the only command and serves private chats in English.
- Uses the shared Asterfield `core-postgres` identity hub with an isolated
  `makeitmd` schema for conversion audit, lifetime statistics, and polling
  state.
- Includes bounded retries, token-safe errors, durable offsets, replay
  protection, `/healthz`, `/metrics`, retention cleanup, Docker health checks,
  CI, vulnerability scanning, and tag-driven releases.

### Fixed

- Consecutive Telegram messages produced by one large paste are debounced and
  joined into a single logical Rich Markdown conversion instead of rendering
  partial fragments separately.
- Inputs rejected because GitHub README layout HTML embeds badge images inside
  paragraphs receive one deterministic compatibility retry while the exact
  original source remains stored for audit.
- Restores formatting consumed by Telegram clients from UTF-16
  `Message.entities`, including entities across joined paste chunks.
- Preserves inline HTML formatting inside tables and converts unsupported linked
  badge images into ordinary links during compatibility fallback.
- Keeps retention-bound operator traces for raw input, outbound Markdown, and
  every Telegram delivery result.

### Operations

- Production stack: `/opt/stacks/makeitmd`.
- Container: `makeitmd-bot`.
- Database role/schema: `makeitmd_core` / `makeitmd`.
- Raw terminal conversion rows expire after 90 days by default; lifetime
  aggregate statistics remain available.

## v0.1.0-alpha.1 - 2026-07-13

### Added

- Minimal English-only Telegram bot that renders user text with Bot API 10.1 Rich Markdown.
- Shared `core-postgres` integration with `core.person`/`core.presence`, an
  isolated `makeitmd` schema, private conversion audit rows, and per-user
  statistics.
- Production health and metrics endpoints, build metadata, Docker healthcheck,
  configurable 90-day conversion retention, CI, tag-driven releases, and
  Apache-2.0 licensing.
- Go 1.26.5 minimum toolchain to avoid the standard-library TLS vulnerability
  reported as `GO-2026-5856`.
- Lifetime per-user aggregates that remain intact when old raw conversion text
  expires.

### Fixed

- Corrected `getUpdates` result decoding so long polling can receive updates.
- Redacted Telegram tokens from transport errors, honored rate-limit delays,
  added bounded outage/update retries, and stopped acknowledging failed
  `/start` replies.
- Prevented replayed updates from resending conversions already marked `sent`.
- Ignored macOS AppleDouble `._*` migration sidecars so snapshot archives can't
  cause a production restart loop.
