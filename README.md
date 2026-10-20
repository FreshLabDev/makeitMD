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
| Exact input | Preserves source; restores formatting consumed by Telegram clients |
| Native rendering | Uses Telegram `sendRichMessage`, not a custom parser |
| Minimal interaction | `/start` only, one panel: two tabs and the language, no settings |
| Private operations | Statistics and source audit stay operator-only |
| Shared integration | Uses the family’s small `github.com/FreshLabDev/tg` HTTP client |

> **Result:** one text message or text file in, one native rich message out.

## Status

| Channel | Version | Meaning |
|:--|:--|:--|
| Stable tag | `v0.1.2` | Latest stable version recorded in this repository |
| Development | `dev` | File input and clearer formula help, not released |

The running production version must be checked through `/healthz` or About;
this table is not a live deployment check.

## Quick Start

You need Go 1.26.10+, Docker, a Telegram token from
[BotFather](https://t.me/BotFather), and access to the Asterfield shared
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
3. Send Markdown source text, or attach a UTF-8 text file.
4. Receive the native Telegram Rich Markdown message.

The greeting is intentionally short:

> Send me Markdown as a message or a text file. I’ll render it.

Files such as `.md`, `.txt`, `.markdown`, `.mdown`, `.csv`, `.json`, `.tex`,
and extensionless text are accepted by content, regardless of the supplied MIME
type. The limit is 128 KiB (131,072 bytes), including an optional UTF-8 BOM.
The file contents are rendered as Rich Markdown, not converted from another
document format. PDF, Word, archives, invalid UTF-8, and binary data are rejected.
Captions are ignored. Each file is one conversion and is never joined to a
nearby text message or another file.

An ordinary paste of a chatbot's answer gives Telegram the clipboard's HTML,
and the client keeps only what a plain message can hold: headings, lists,
tables and LaTeX backslashes are gone before makeitMD receives the text. Paste
without formatting (⌘⇧V, Ctrl+Shift+V) or send a file instead. When a long
message arrives with a quote or text link and no heading, table or bullet line
left, the reply is followed by that advice.

Downloads use the shared Telegram client, have a 15-second budget including
`getFile`, and are removed from a private temporary directory after processing.
Accepted source and original document metadata follow the existing conversion
retention policy. Rejected or unsuccessfully downloaded files do not create a
conversion row. A download failure asks the sender to resend the file.

`/start` also opens the panel: **How it works**, **About** — the running
version, the rendering the bot uses, the repository, the licence and the admin —
and **Language**. `Back` returns to the greeting, and every screen edits the
same message rather than adding another one. There is nothing else to configure,
so there is no settings screen.

The panel speaks the sixteen languages the Asterfield bots share. It follows the
language your Telegram client reports until you pick one, and the language you
pick is stored once for the whole family: the other bots answer in it too.
*Follow Telegram* on that screen hands the decision back to your client.

Everything else the bot says is a rendered message. In a group makeitMD renders
nothing: `/start` there is registered as an ephemeral command, so the answer —
a link back to the private chat — is visible only to the person who sent it.

Rich Markdown supports headings, nested styles, lists, task lists, tables,
quotes, code blocks, details, links, formulas, and other structured content.
Telegram currently limits one rich message to 32,768 UTF-8 characters of rendered
text, including formula source and custom emoji alternative text. Source bytes
and rendered characters are different limits. Telegram also limits block count,
nesting, media and table width. Oversized documents are not truncated or split
automatically, because splitting could break tables, code or formulas.

### Formulas

Telegram Rich Markdown supports inline `$x^2 + y^2$`, block `$$E=mc^2$$`, and
fenced `math` blocks:

````markdown
Inline: $\frac{a}{b}$

$$E=mc^2$$

```math
\int_0^1 x^2\,dx = \frac{1}{3}
```
````

The formula source goes to Telegram unchanged. Telegram’s documented syntax is
not a promise of full KaTeX or a complete TeX document engine.

`\(...\)` and `\[...\]`, the delimiters ChatGPT writes, are the one rewrite:
Markdown reads `\(` as an escaped parenthesis, so Telegram would show
`(x^2)` instead of a formula. Outside code they become `$...$` and `$$...$$`.
An escaped backslash (`\\(`) is left alone, and `\[...\]` is converted only
when it stands on its own lines or contains LaTeX syntax, so `\[1\]` stays a
bracketed reference.
Putting `$...$` inside ordinary inline code or a non-`math` code block makes it
code, not a formula. Receiving clients must support rich messages.

See Telegram’s [Rich Markdown syntax and limits](https://core.telegram.org/bots/api#rich-message-formatting-options)
and [file download contract](https://core.telegram.org/bots/api#getfile).

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

- `makeitmd.conversions` — exact decoded source, raw input message and entities,
  every outbound Markdown/result attempt, final Telegram result, delivery state,
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

## Production

`deploy/ws04/compose.yaml` is the production stack. It pulls the image the
release workflow published to GHCR and never builds one:

```sh
# in the stack directory, alongside .env
MAKEITMD_IMAGE=ghcr.io/freshlabdev/makeitmd@sha256:<digest> docker compose up -d
```

Pin `MAKEITMD_IMAGE` by digest rather than tag. A digest names one exact build,
so a rollback is a one-line change with nothing to rebuild, and `docker inspect`
on the running container answers which commit it came from. `BUILD_VERSION` /
`BUILD_COMMIT` / `BUILD_DATE` are baked in by the release workflow; they only
need setting for a local build.

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
