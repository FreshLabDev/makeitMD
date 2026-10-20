# Changelog

All notable makeitMD changes are documented here.

The `## <tag>` section of this file *is* the GitHub Release body: the release
workflow copies it verbatim and refuses a tag that has no section. Write it
for whoever has to decide whether to upgrade.

See [`docs/versioning.md`](docs/versioning.md) for what the numbers mean and
[`docs/releases.md`](docs/releases.md) for how a release is published.

## Unreleased

Use this section for changes that are merged but not released yet.

## v0.1.4 - 2026-10-20

makeitMD renders Markdown sent as a text file, renders formulas the way
ChatGPT, Gemini and Claude actually deliver them, and is built with a Go
release that closes ten advisories `v0.1.2` is exposed to.

This promotes `v0.1.4-alpha.5` unchanged. The five alphas ran on the
production bot, which has no test twin. Answers from ChatGPT, Gemini and
Claude were pasted from Telegram Desktop and from phones, and formula files
were sent; the replies were checked on Desktop, Android and iOS. The failure
path was exercised live too: four pastes that arrived as rich messages got the
explanation instead of silence. The details are in the alpha sections below.

There is no `v0.1.3`: that tag was pushed by mistake with unreviewed work,
nothing was published from it, and tags here cannot be deleted.

### Added

- Markdown sent as a UTF-8 text file of up to 128 KB is rendered like a paste.
- `\(...\)` and `\[...\]`, ChatGPT's delimiters, render as formulas, and so
  do the doubled ones its copy button produces.
- Display formulas that the ChatGPT and Gemini phone apps copy as bare LaTeX,
  Gemini's inside a code block, are recovered in pastes that delimit no
  formula. Inline formulas ChatGPT's app deletes while copying cannot be.
- A paste that lost its headings, lists and tables is followed by advice:
  paste without formatting, send a file, or on a phone send from a computer.

### Fixed

- A paste that arrives as a rich message, which Telegram Premium clients send,
  gets an explanation instead of no answer at all.
- A long paste from a phone is no longer stitched tail first.
- Inline code restored from client formatting keeps its edge backticks and
  spaces.

### Security

- Go 1.26.10 and `golang.org/x/text` 0.41.0: `govulncheck` found ten reachable
  advisories on `v0.1.2` and finds none here.

## v0.1.4-alpha.5 - 2026-10-20

Found by pasting chatbot answers from phones into alpha.4. Still on the
production bot.

### Fixed

- **A long paste from a phone is no longer stitched tail first.** A phone
  sends the parts of a split paste at once, and Telegram gave the 2637-character
  tail of a chatbot answer a smaller message id than its 4067-character start,
  so the reply began in the middle. When all parts arrive in the same second
  and exactly one is shorter than 3000 UTF-16 units without being last, it is
  the tail and moves to the end. Anything else keeps message id order, so a
  short note typed before a paste stays first.
- **Display formulas copied on a phone render.** Chatbot apps such as ChatGPT
  copy a display formula as bare LaTeX, with no delimiters; one app puts it in
  a code block. In a paste whose source delimits no formula, a line or a
  language-less code block that reads as a formula now becomes `$$...$$`: a
  LaTeX command next to math, or a sub/superscript with a relation, and no
  word of three or more letters once commands and braced arguments are
  removed. A `\begin{cases}` split over lines and a formula a phone broke in
  two are joined. On the test answers this recovered 7 of ChatGPT's display
  formulas and 5 of another app's, and changed nothing in 17 earlier pastes.
  ChatGPT's app deletes inline formulas while copying; those cannot come back.
- **The paste advice tells phone users what to do.** ⌘⇧V does not exist on a
  phone, and chatbot apps there lose headings, lists and tables while copying,
  so the advice now says to send from a computer.

## v0.1.4-alpha.4 - 2026-10-20

Found by pasting a Claude answer copied in the iPhone app: the bot did not
answer at all. Still on the production bot.

### Fixed

- **A message that arrives already formatted gets an answer.** Since Bot API
  10.1 a Telegram client can send a paste as a rich message: blocks, with no
  text field and no Markdown source. The bot ignored such messages without a
  word. It now explains that it cannot read them and suggests a .md file or a
  paste without formatting. Nothing is stored, and the log records only how
  many blocks of each type arrived, to find out what clients send before
  deciding whether to render them.

## v0.1.4-alpha.3 - 2026-10-20

Found by pasting a ChatGPT answer into alpha.2 with ⌘⇧V. Still on the
production bot.

### Fixed

- **ChatGPT formulas pasted as plain text render.** ChatGPT's copy button
  escapes its Markdown once more, so the plain text holds `\\[ 1\\,000 \\]`
  and `\\(PV\\)`. alpha.2 read the doubled backslash as an escaped backslash
  and left every formula of the test answer as text. Doubled delimiters are
  now formulas too, with Markdown escapes such as `\_` and `\&` removed from
  the content. They are converted only when at least one holds LaTeX syntax,
  so a lone `C:\\(x86\\)` stays text.

## v0.1.4-alpha.2 - 2026-10-20

Found by pasting answers from three chatbots into alpha.1: ChatGPT's formulas
did not render, and an ordinary paste lost tables and lists before the bot saw
them. Still on the production bot.

### Added

- **Formulas written as `\(...\)` and `\[...\]` render.** That is how
  ChatGPT delimits them, and Markdown reads `\(` as an escaped parenthesis, so
  Telegram showed `(7^{-\frac{2}{3}})` as text instead of a formula. Outside
  code they now become `$...$` and `$$...$$`; a multi-line `\[...\]` becomes
  one line. An escaped backslash and a bracketed reference such as `\[1\]` keep
  their meaning. In a ChatGPT answer pasted on 20 October all fifteen formulas
  were affected.
- **Advice after a paste that lost its structure.** An ordinary paste of a
  chatbot's answer hands Telegram the clipboard's HTML, and the client drops
  headings, lists, tables and LaTeX backslashes before the bot sees the text.
  A long message that arrives with a quote or text link and no heading, table
  or bullet line left now gets one more message after the reply: paste without
  formatting (⌘⇧V, Ctrl+Shift+V) or send a file. How it works says the same.
  Of the three chatbot answers pasted on 20 October, all three would have got
  it; the same answer pasted with ⌘⇧V would not.

## v0.1.4-alpha.1 - 2026-10-20

makeitMD renders Markdown sent as a text file, and is built with a Go release
that closes ten advisories the running `v0.1.2` is exposed to.

makeitMD has no test bot, so this alpha runs on the production bot. It becomes
`v0.1.4` once formulas from `docs/examples/formulas.md` sent as a file render
correctly on Telegram Desktop, Android and iOS, or is rolled back to `v0.1.2`.

There is no `v0.1.3`. That tag was pushed by mistake on 20 October with work
that had not been reviewed, nothing was published from it, and tags here
cannot be deleted.

### Added

- **Markdown can be sent as a file.** A UTF-8 text file of up to 128 KB gets
  the same native rich reply as pasted Markdown, whatever its extension: `.md`,
  `.txt`, `.tex` or none. Text is recognised by its contents, not by the name or
  MIME type Telegram reports. Captions are ignored, and a file is never joined
  to a neighbouring message. An empty, binary or non-UTF-8 file, a PDF and a
  file over the limit each get a short explanation instead. The download has
  15 seconds including `getFile`, retries a transient failure up to three
  times within that budget, and goes to a private temporary directory that is
  removed afterwards. Accepted files follow the existing conversion retention;
  rejected ones are not stored. In groups a file is not downloaded at all.
- How it works explains files, formulas (`$...$`, `$$...$$` and fenced
  `math`) and Telegram's limits on a rich message. Formula source is passed to
  Telegram unchanged; `\(...\)` and `\[...\]` are not rewritten.

### Security

- Go moves from 1.26.6 to 1.26.10 and `golang.org/x/text` from 0.39.0 to
  0.41.0. On `v0.1.2` `govulncheck` finds ten advisories reachable from
  makeitMD's code, nine in the standard library (among them GO-2026-6611,
  -6612, -6613 and -6617 in `net/http` and HTTP/2) and GO-2026-6629 in
  `x/text`; on this version it finds none.

### Fixed

- Inline code restored from a client's formatting kept its edge backticks and
  surrounding spaces wrong: a span starting or ending with a backtick merged
  with the delimiter, and a span with a space at both ends lost them. Both now
  come back as the person typed them.

### Changed

- Rendering errors now mention Telegram’s rich-message limits as well as
  Markdown syntax. A byte download limit is not a rendered-character limit;
  documents are never silently truncated or split.
- **A release waits for CI.** `release.yml` runs `ci.yml` on the tagged commit
  and publishes nothing unless it passed. When CI cannot run, `release.yml` can
  be started by hand with the tag and a reason; that run skips CI and prints the
  reason at the top of the release notes. See `docs/releases.md`.
- The release notes end with the image digest to pin, which `docs/releases.md`
  already said they did.
- The image carries `org.opencontainers.image.version` and `.revision` labels,
  so `docker inspect` names the build without starting it.
- CI also runs weekly, so a new advisory against a dependency or the standard
  library is noticed while nothing is being pushed.
- Dependabot opens grouped version updates against `dev`.

## v0.1.2 - 2026-09-10

makeitMD speaks sixteen languages.

Every word it says moved out of the Go source into a translation catalogue, the
language is read from the shared hub so a choice made in any sibling bot is
honoured here, and there is a picker for changing it. Nobody has to do anything:
a person whose Telegram is Russian gets Russian without touching a setting.

### Changed

- `docs/releases.md` names the real cause of the `403` a digest pull used to
  answer: the account running the deploy could not read the private package,
  not anything about the digest. Pulling the tag first was a workaround for
  that, and is no longer required.

## v0.1.2-alpha.1 - 2026-09-09

### Added

- **The panel speaks the languages the family shares, and lets you pick one.**
  Every sentence makeitMD says now comes from one embedded catalogue instead of
  a Go constant, the language is resolved per person from the shared hub with
  the Telegram client's own language as the fallback, and a *Language* screen
  offers the same sixteen options, in the same order, that the sibling bots
  offer. The choice is stored once for the whole family, so making it here makes
  it everywhere; *Follow Telegram* withdraws it and lets the client decide
  again. Only English is written so far -- the other fifteen locales read in
  English until they are translated, which is why they can arrive one at a time.
  Telegram's command menu is published per language for the same reason: a
  translated panel opened from an English menu speaks two languages at one
  person.

### Changed

- **Every screen opens with a title.** The root screen used to start with bare
  prose while every other panel in the family started with a bold title and a
  one-line hint, which made makeitMD read as a different product from the bot
  next to it. The greeting itself is unchanged, and the About card keeps the
  shape it already had.

## v0.1.1 - 2026-09-09

makeitMD gets a panel. The product boundary moved deliberately: /start now opens
two tabs instead of answering with a sentence and no way forward. Still no
settings.


### Changed

- **`/start` now opens a panel instead of answering with one sentence.** Two
  tabs and no more: *How it works*, and an *About* card carrying the version and
  commit the running process reports on `/healthz`, the rendering it uses, the
  repository, the licence and the admin. `Back` is the only navigation, every
  tab edits the same message rather than posting another one, and there is still
  nothing to configure. The greeting itself is unchanged.
- **Commands are published per scope instead of globally.** The private list
  is `/start — Open the makeitMD panel`; the group list is one `/start`
  registered as ephemeral, so a person on a Bot API 10.3 client who types it in
  a group gets a private link back to the direct chat and the group sees nothing
  at all. An older client cannot send an ephemeral command, so the bot has no
  message it is entitled to answer privately and stays silent rather than
  replying in front of everyone -- which reads as a dead bot, and will be the
  common case until clients catch up. The default
  scope, where the old global `Start the bot` entry lived, is cleared. Nothing
  is offered in a chat where it would do nothing.
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
- `github.com/FreshLabDev/tg` moves to `v0.0.1-alpha.7`. It carries one fix:
  a preflight probe is marked as a probe, so the `404 method not found` it
  expects stops being counted and logged as a transport failure. All four bots
  on the shared client now run the same version.

### Added

- `docs/releases.md` gained a **Deploying** section, and `AGENTS.md` points at it.
  Releasing was documented; deploying was not, in any repository in the family —
  the process stopped at "deploy it" and never said how. That gap mattered more
  after the stacks moved from building on the host to pulling a published image,
  because the procedure changed on the same day. The section names this stack's
  host directory, its env file, the variable that selects the image, the networks
  it needs, and what a rollback actually is.

- `internal/build` holds the version, commit and build date the linker stamped
  in. `/healthz` and the About card read the same value, so the panel cannot
  report a build that is not the one answering.
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
