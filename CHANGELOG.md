# Changelog

All notable makeitMD changes are documented here.

## Unreleased

Use this section for changes that are merged but not released yet.

## v0.1.0 - 2026-07-13

First stable public release, promoted after three production release candidates
and live verification on WS04.

### Highlights

- Renders the user's exact source text through Telegram Bot API 10.1 Rich
  Markdown without a custom parser, AI, buttons, or settings.
- Keeps `/start` as the only command and serves private chats in English.
- Uses the shared FreshLab `core-postgres` identity hub with an isolated
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
