<h1 align="center">makeitMD</h1>

<p align="center"><strong>Send Markdown. Get native Telegram rich text.</strong><br/>A deliberately small Go bot built directly on Telegram Bot API 10.1.</p>

<p align="center">
  <a href="https://github.com/FreshLabDev/makeitMD/releases"><img src="https://img.shields.io/github/v/release/FreshLabDev/makeitMD?include_prereleases&sort=semver&style=for-the-badge&label=latest&labelColor=0f172a&color=26A5E4" alt="latest version"></a>
  <a href="docs/versioning.md"><img src="https://img.shields.io/badge/stable-v0.1.0-26A5E4?style=for-the-badge&labelColor=0f172a" alt="stable version"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/FreshLabDev/makeitMD?style=for-the-badge&logo=go&logoColor=white&label=go&labelColor=0f172a&color=00ADD8" alt="Go version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-334155?style=for-the-badge&labelColor=0f172a" alt="Apache-2.0 license"></a>
  <a href="https://t.me/makeitMD_bot"><img src="https://img.shields.io/badge/telegram-%40makeitMD__bot-26A5E4?style=for-the-badge&logo=telegram&logoColor=white&labelColor=0f172a" alt="Telegram bot"></a>
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> ·
  <a href="#using-makeitmd">Usage</a> ·
  <a href="#how-it-works">How It Works</a> ·
  <a href="#operations">Operations</a> ·
  <a href="#configuration">Configuration</a>
</p>

---

## The Idea

Telegram Bot API 10.1 can render structured Rich Markdown natively. makeitMD
keeps the product boundary as small as possible: send source text in a private
chat and receive Telegram's native rendering immediately.

| Principle | makeitMD approach |
|:--|:--|
| Exact input | Passes the submitted string unchanged as `rich_message.markdown` |
| Native rendering | Uses Telegram `sendRichMessage`, not a custom parser |
| Minimal interaction | English only, `/start` only, no buttons or settings |
| Private operations | Statistics and source audit stay operator-only |
| Direct integration | Calls the Bot API over HTTP without a Telegram library |

> **Result:** one text in, one native rich message out.

## Status

| Channel | Version | Meaning |
|:--|:--|:--|
| Release target | `v0.1.0` | First public release, prepared and awaiting publication |
| Production | `v0.1.0-rc.1` | Runs as `makeitmd-bot` with healthy DB and Telegram polling |

## Quick Start

You need Go 1.26.5+, Docker, a Telegram token from
[BotFather](https://t.me/BotFather), and access to the FreshLab shared
`core-postgres` database.

```sh
cp .env.example .env
$EDITOR .env
docker compose up --build
```

Before the first start, apply `core/migrations/003_makeitmd.sql`, provision the
`makeitmd_core` password, and ensure the external Docker network `core_net`
exists.

## Using makeitMD

1. Open [@makeitMD_bot](https://t.me/makeitMD_bot).
2. Send `/start`.
3. Send Markdown source text.
4. Receive the native Telegram Rich Markdown message.

The greeting is intentionally short:

> Send me Markdown. I’ll render it.

Rich Markdown supports headings, nested styles, lists, task lists, tables,
quotes, code blocks, details, links, formulas, and other structured content.
Telegram currently limits one rich message to 32,768 UTF-8 characters.

## How It Works

```text
Telegram getUpdates
        |
        v
core.touch identity/presence
        |
        v
store exact source as received
        |
        v
sendRichMessage(markdown: exact source)
        |
        +--> sent   -> lifetime statistics
        +--> failed -> short syntax error
```

The durable Telegram offset advances only after handling an update. Replayed
updates do not resend conversions already marked `sent`.

## Shared Core Database

Global Telegram identity and presence live in `core.person` and
`core.presence`. Private makeitMD data lives in the isolated `makeitmd` schema:

- `makeitmd.conversions` — exact source, Telegram identifiers, delivery state,
  character count, and byte count.
- `makeitmd.user_stats` — lifetime per-user conversion totals independent of
  raw-text retention.
- `makeitmd.runtime_state` — durable Telegram update offset.
- `makeitmd.schema_migrations` — applied domain migration ledger.

Raw `sent` and `failed` conversions expire after 90 days by default. Unresolved
`received` rows remain available for recovery. No statistics are shown to users.

## Operations

- `GET /healthz` checks PostgreSQL and Telegram polling freshness and returns
  version, commit, build date, and pending/failed conversion counts.
- `GET /metrics` exposes private-data-free Prometheus counters.
- Docker marks the container unhealthy when `/healthz` fails.
- Telegram GET requests retry transport, 429, and 5xx failures with jitter.
- POST requests retry only when Telegram explicitly returns `retry_after`,
  avoiding unsafe duplicate sends after an ambiguous transport failure.
- Transport errors are sanitized before logging so the bot token cannot appear
  through an embedded Bot API URL.

## Configuration

| Variable | Required | Default | Purpose |
|:--|:--:|:--|:--|
| `TELEGRAM_BOT_TOKEN` | yes | — | Telegram Bot API token |
| `DATABASE_URL` | yes | — | `makeitmd_core` connection to shared core-postgres |
| `AUTO_MIGRATE` | no | `true` | Apply makeitMD migrations on startup |
| `MIGRATIONS_DIR` | no | `./migrations` | SQL migration directory |
| `HTTP_ADDR` | no | `:8080` | Internal health and metrics listener |
| `CONVERSION_RETENTION` | no | `2160h` | Retention for terminal raw conversion rows |
| `LOG_LEVEL` | no | `info` | Reserved log level |
| `BUILD_VERSION` | no | `dev` | Version stamped into health and logs |
| `BUILD_COMMIT` | no | `none` | Source revision |
| `BUILD_DATE` | no | `unknown` | UTC build timestamp |

## Development

```sh
go mod verify
go test -race ./...
go vet ./...
govulncheck ./...
docker compose config
docker build -t makeitmd:local .
```

CI repeats module verification, build, race tests, vet, vulnerability scanning,
Docker build, and Compose validation. See [versioning](docs/versioning.md),
[release process](docs/releases.md), and the full [changelog](CHANGELOG.md).

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
