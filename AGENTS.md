# AGENTS.md

Keep makeitMD minimal, private by default, and production-minded.

## Product Boundaries

- English only.
- `/start` is the only command.
- No buttons, menus, settings, AI, or custom Markdown parser.
- Accept text only in private chats. Pass plain source unchanged; when a Telegram
  client has consumed formatting into `Message.entities`, deterministically
  restore only those entity ranges before calling Bot API 10.1 `sendRichMessage`.
- Never add user-facing statistics unless explicitly requested.

## Data And Security

- Never log the bot token or full Telegram API URLs.
- Use the shared `core-postgres` database with `search_path=makeitmd,core`.
- Call `core.touch('makeitmd', ...)` before domain writes so `core.person` exists.
- Store source text and delivery state in `makeitmd.conversions`.
- Keep retention-bound Telegram input entities, rendered outbound Markdown, and
  the API result in the conversion row for operator-only transport debugging.
- Derive private per-user statistics from `makeitmd.user_stats`.
- Never put conversion text in application logs or shared `core.*` identity tables.
- Treat conversion rows as private user content and document retention changes explicitly.
- Sanitize every transport error before it reaches logs; net/http errors can embed the bot token in the URL.

## Database Boundaries

- `core.*` owns shared identity, presence, and language.
- `makeitmd.*` owns conversions, private analytics, and polling state.
- The service connects as the least-privilege `makeitmd_core` role.
- Add immutable numbered migrations; do not edit an applied migration.
- Terminal conversion rows expire according to `CONVERSION_RETENTION`; never delete unresolved `received` rows automatically.
- Lifetime `user_stats` must survive raw conversion retention and increment only on the first `received → sent` transition.

## Reliability

- Never advance the Telegram offset after a failed update until its retry budget is exhausted.
- Retry idempotent GET requests on transport failures, 429, and 5xx responses.
- Retry POST only when Telegram explicitly returns `retry_after`; generic POST retries can duplicate messages.
- `/healthz` must cover database availability and Telegram polling freshness.
- Keep `/metrics` free of user identifiers and submitted text.

## Code Style

- Use Go's standard library and direct Bot API HTTP calls.
- Keep protocol types small and explicit.
- Add dependencies only when they materially reduce complexity.

## Releases

- Follow `docs/versioning.md` and `docs/releases.md`.
- Keep notable changes under `## Unreleased` in `CHANGELOG.md` until release preparation.
- Do not publish a release without explicit user approval.

## Verification

```sh
go test ./...
go vet ./...
docker compose config
```

CI must also run `go mod verify`, `go test -race ./...`, Docker build, Compose
validation, and `govulncheck`.

## Release Checklist

- `core/migrations/003_makeitmd.sql` is applied and `makeitmd_core` can connect.
- `/start` replies in English without buttons.
- Valid Rich Markdown is preserved byte-for-byte in the API request and renders successfully.
- Invalid Rich Markdown is stored as failed and receives the short error response.
- Telegram 429 honors `retry_after`; transport errors never reveal the bot token.
- Restart recovery does not resend an already `sent` conversion.
- `/healthz` reports DB and polling healthy; `/metrics` exposes no private content.
- Retention cleanup deletes only old terminal rows.
