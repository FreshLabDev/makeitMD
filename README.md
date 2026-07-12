<h1 align="center">makeitMD</h1>

<p align="center"><strong>Send Markdown. Get native Telegram rich text.</strong><br/>One small Go bot built directly on Telegram Bot API 10.1.</p>

---

## What It Does

Send the bot text in a private chat. It passes that exact text to Telegram's `sendRichMessage` method as Rich Markdown and immediately returns Telegram's native rendering.

There are no buttons, settings, extra commands, AI, or custom parser. `/start` only says:

> Send me Markdown. I’ll render it.

Rich Markdown supports headings, nested styles, lists and task lists, tables, quotes, code blocks, details, links, formulas, and more. Telegram currently limits one rich message to 32,768 UTF-8 characters.

## Quick Start

```sh
cp .env.example .env
$EDITOR .env
docker compose up --build
```

Configure `TELEGRAM_BOT_TOKEN` from [BotFather](https://t.me/BotFather) and the
shared-core `DATABASE_URL` before starting the service.

## Shared Core Database

makeitMD is a member of the FreshLab bot family and uses the shared WS04
PostgreSQL database. Global Telegram identity and presence live in `core.*`;
private makeitMD data lives in the isolated `makeitmd` schema.

- `makeitmd.conversions` stores the exact submitted text, delivery status,
  character and byte counts, and Telegram identifiers.
- `makeitmd.user_stats` keeps lifetime per-user totals for successful
  conversions, independently of raw-text retention.
- `makeitmd.runtime_state` stores the durable Telegram update offset.
- `core.person` and `core.presence` provide shared identity and who-was-where
  data through `core.touch('makeitmd', ...)`.

The conversion table contains private user content. Successful and rejected
conversions are deleted after `CONVERSION_RETENTION` (90 days by default);
unresolved `received` rows are retained for operational recovery. Nothing about
statistics is shown to users.

## Operations

- `GET /healthz` checks PostgreSQL and Telegram polling freshness and reports
  the exact build version, commit, and build date.
- `GET /metrics` exposes minimal Prometheus counters for polling, processed
  updates, conversions, failures, and Telegram rate limits.
- Docker marks the container unhealthy if `/healthz` fails.
- Telegram polling backs off during outages, honors `retry_after`, and keeps a
  failed update unconfirmed for retry before dropping a persistent poison update.

## Configuration

| Variable | Required | Default | Purpose |
|:--|:--:|:--|:--|
| `TELEGRAM_BOT_TOKEN` | yes | — | Bot API token |
| `DATABASE_URL` | yes | — | `makeitmd_core` connection to shared core-postgres |
| `AUTO_MIGRATE` | no | `true` | Apply makeitMD schema migrations on startup |
| `MIGRATIONS_DIR` | no | `./migrations` | SQL migration directory |
| `HTTP_ADDR` | no | `:8080` | Internal health and metrics listener |
| `CONVERSION_RETENTION` | no | `2160h` | Retention for terminal conversion rows |
| `LOG_LEVEL` | no | `info` | Reserved log level |
| `BUILD_VERSION` | no | `dev` | Version stamped into production health and logs |
| `BUILD_COMMIT` | no | `none` | Source revision or archive hash |
| `BUILD_DATE` | no | `unknown` | UTC build timestamp |

Before the first bot start, apply `core/migrations/003_makeitmd.sql` through
the core migrator and configure `MAKEITMD_CORE_PASSWORD`. The bot stack must
join the external Docker network `core_net`.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go mod verify
docker compose config
```

See [versioning](docs/versioning.md) and [releases](docs/releases.md) for the
Branchy-style tag-driven release process. The project is licensed under
[Apache-2.0](LICENSE).
