# Changelog

All notable makeitMD changes are documented here.

## Unreleased

Use this section for changes that are merged but not released yet.

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
