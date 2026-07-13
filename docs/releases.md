# Release Process

`CHANGELOG.md` is the source of truth for release notes.

1. Finish code and documentation.
2. Run `go mod verify`, `go test -race ./...`, `go vet ./...`, Docker build,
   and `docker compose config`.
3. Smoke-test the bot with real Telegram credentials for beta, RC, and public releases.
4. Move `Unreleased` notes to `## vX.Y.Z - YYYY-MM-DD`.
5. Create an annotated tag and matching GitHub Release.

Production deployment and its matching alpha/beta/RC prerelease are one
operation: never leave production running an unpublished build. Stable and
otherwise full releases are published only on an explicit user command.

Release notes use this shape:

```text
makeitMD v0.1.0-alpha.1

Summary:
- Short release purpose.

Highlights:
- Important shipped behavior.

Operations:
- Data or deployment notes.

Verification:
- go test ./...
- go test -race ./...
- go vet ./...
- Docker build
- docker compose config
- /healthz and /metrics
- smoke test status

Known limitations:
- What is intentionally not done yet.
```

Mark `alpha`, `beta`, and `rc` GitHub Releases as pre-release. Do not publish a release until the tag, notes, and verification match.
