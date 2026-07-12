# Versioning

makeitMD uses SemVer-style versions with pre-release tags before `v1.0.0`.

## Version Line

```text
v0.1.0-alpha.1  initial implementation
v0.1.0-beta.1   live-tested with limited users
v0.1.0-rc.1     public release candidate
v0.1.0          public MVP release
```

## Rules

- Use `alpha` until polling, Rich Markdown rendering, persistence, and restart behavior are proven with real Telegram credentials.
- Use `beta` after the complete bot works end to end for limited users.
- Use `rc` when only release-blocking fixes are expected.
- Use patch versions for fixes and minor versions for compatible behavior or operations improvements.
- Do not use `v1.0.0` until the production contract, shared-core migration,
  and data-retention policy are stable.
