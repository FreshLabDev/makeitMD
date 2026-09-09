# AGENTS.md

Keep makeitMD minimal, private by default, and production-minded.

## Product Boundaries

- Sixteen languages, one catalogue. Every user-facing string lives in
  `internal/i18n/translations.json`, keyed by screen; none of them is a Go
  constant. English is the fallback, so a language nobody has translated yet
  reads in English rather than blank -- which is what lets a locale land on its
  own. Add a language to the existing keys; never rename or restructure a key
  that translations are already hanging off. Log lines and identifiers stay
  English and stay out of the catalogue.
- The interface language belongs to the person, not to this bot. It is resolved
  per update from `core.effective_language`, with the language the Telegram
  client reports as the fallback. The language screen writes the choice back
  with `core.set_language('makeitmd', 'user', …, 'manual')` so every sibling bot
  honours it, and "Follow Telegram" clears that claim with
  `core.clear_language` rather than storing today's client language as a
  choice.
- `/start` is the only command. It is published per scope, never globally: the
  private list is the panel, the group list is one ephemeral redirect, and the
  default scope is cleared.
- `/start` opens one panel, one screen deep: **How it works** and **About** as
  two tabs, **Language** below them, each returning with `Back`. This is where
  the boundary moved twice -- it used to be "no buttons at all", then "two tabs
  and no state a button can change". A command that answers with a sentence and
  no way forward reads as an unfinished bot; and a bot that renders sixteen
  languages with no way to pick one leaves the choice to a Telegram setting a
  person may not want to change for one bot. What did not move: there are still
  no settings of makeitMD's own. The language is not one -- it is stored in the
  shared hub and read by every bot in the family. A fourth screen needs a reason
  as good as these.
- Every screen is built by `panelText`: bold title, italic one-line hint, the
  substance in a `<blockquote>`. The About card is the one exception, because
  the family states name, version and purpose the same way everywhere. A screen
  whose state lives on its buttons -- the language picker -- says nothing about
  that state in its body.
- No `Close` button in a private chat -- the conversation is the panel, so there
  is nothing covering anything. This bot has no group panel, so it has no Close
  at all. No emoji on button labels: in this family an emoji marks state, it is
  not an icon. The flags in the language picker are the content of those
  buttons, not decoration on them; `◉`/`◎` mark which option is chosen and sit
  on every option of the set, never only on the chosen one.
- Style the one button that matters on a screen (`tg.StylePrimary`) and nothing
  else; `tg.StyleDanger` stays unused because makeitMD destroys nothing.
- The About card takes its version and commit from the same `build.Info`
  `/healthz` reports. Never introduce a second source for either.
- No AI, and no custom Markdown parser.
- Render text only in private chats. Pass plain source unchanged; when a Telegram
  client has consumed formatting into `Message.entities`, deterministically
  restore only those entity ranges before calling Bot API 10.1 `sendRichMessage`.
- In a group the bot renders nothing. `/start` is registered there with
  `is_ephemeral`, so the command and its answer are visible only to the person
  who typed it; without an ephemeral id to answer, stay silent rather than post
  a redirect the whole group has to read. Group traffic reaches no database
  write at all.
- Panel text is HTML through `SendMessage`/`EditMessageText`; it is fixed,
  reviewed and carries no user input. User-facing failure strings stay plain
  text, and user Markdown stays `sendRichMessage`.
- Never add user-facing statistics unless explicitly requested.

## Data And Security

- Telegram goes through `github.com/FreshLabDev/tg`, the client shared by the
  bot family. makeitMD keeps no private HTTP client: transport, retries, token
  redaction and rich messages live there. Raw Telegram payloads that the
  conversion record keeps come from `Message.Raw` and `APIError.Response`.
- `Preflight` runs before the bot starts and requires `sendRichMessage`.
  Rendering Markdown is the whole bot: a server without that method would
  leave it answering nothing at all.
- Rich Markdown is sent with entity detection on (`tg.WithEntityDetection`).
  A person wrote this Markdown, and a bare URL in it is meant to become a link
  -- unlike the generated text the other bots send.
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

- Work on `dev`. Pre-releases (`-alpha.N`, `-beta.N`, `-rc.N`) are tagged on
  `dev`; stable versions are tagged on `main`, on the merge commit from `dev`.
  The test bot runs `dev`, the production bot runs `main`.
- Follow `docs/versioning.md` and `docs/releases.md`.
- Keep notable changes under `## Unreleased` in `CHANGELOG.md` until release preparation.
- Every production deployment must publish its matching alpha/beta/RC
  prerelease in the same delivery flow.
- Stable and otherwise full releases require explicit user approval.

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
- `/start` opens the panel in the language the person's client reports; How it
  works, About, Language and `Back` all edit the same message instead of
  stacking new ones.
- Picking a language repaints the panel in it, and the sibling bots agree the
  next time they answer. "Follow Telegram" puts the client's language back.
- The About card shows the version `/healthz` reports for the same process.
- `/start` in a group answers only the sender, and posts nothing publicly.
- Valid Rich Markdown is preserved byte-for-byte in the API request and renders successfully.
- Invalid Rich Markdown is stored as failed and receives the short error response.
- Telegram 429 honors `retry_after`; transport errors never reveal the bot token.
- Restart recovery does not resend an already `sent` conversion.
- `/healthz` reports DB and polling healthy; `/metrics` exposes no private content.
- Retention cleanup deletes only old terminal rows.

## Deploying

Do not invent a deploy. [`docs/releases.md`](docs/releases.md) has a **Deploying**
section describing this stack exactly: which host directory it lives in, which
env file names the image, which networks it needs, and how to roll back. Read it
before touching anything on the host.

Two rules that hold everywhere and are easy to get wrong:

- **Nothing is built on the host.** A production stack pulls the image the
  release workflow published. A `build:` section in a production manifest is a
  bug.
- **Pin the digest, not the tag.** A tag moves; a digest names one build that was
  tested, and a rollback becomes one line with nothing to rebuild.
