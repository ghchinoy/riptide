# Releasing Riptide

## Overview

Releases are fully automated via GoReleaser and GitHub Actions. When you push a
semver tag, the `release` workflow builds cross-platform binaries, creates a
GitHub release with checksums, and (if configured) updates the Homebrew tap.

---

## Semver conventions

| Bump   | When to use |
|--------|-------------|
| Patch  | Bug fixes, dependency updates, docs, minor refactors with no behaviour change |
| Minor  | New features, new CLI flags or subcommands, backwards-compatible changes |
| Major  | Breaking CLI interface changes, removed flags/commands, incompatible config changes |

---

## Cutting a release

1. **Ensure `main` is green** — CI must pass before tagging.

2. **Create and push the tag:**

   ```sh
   git checkout main
   git pull origin main
   git tag -a v1.2.3 -m "Release v1.2.3"
   git push origin v1.2.3
   ```

3. **GitHub Actions takes over** — the `release` workflow:
   - Runs `go test ./pkg/...`
   - Builds binaries for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64
   - Creates a GitHub release with the archives and `checksums.txt`
   - Pushes a Homebrew formula to `ghchinoy/homebrew-tap` (requires `HOMEBREW_TAP_GITHUB_TOKEN` secret)

4. **Verify the release** on https://github.com/ghchinoy/riptide/releases.

---

## Verifying a release locally

Run GoReleaser in snapshot mode — it builds all targets without publishing:

```sh
make release-dry-run
```

This is equivalent to:

```sh
goreleaser release --snapshot --clean
```

Artifacts land in `dist/`. Inspect them before pushing a real tag.

---

## Setting up the Homebrew tap secret

The release workflow needs a PAT to push the formula to `ghchinoy/homebrew-tap`:

1. Create a [classic PAT](https://github.com/settings/tokens) with `repo` scope.
2. Add it as a repository secret named `HOMEBREW_TAP_GITHUB_TOKEN` in the
   Riptide repo settings (`Settings → Secrets and variables → Actions`).

If the secret is absent, the release still succeeds — only the formula push is
skipped (GoReleaser will log a warning).

---

## How users install Riptide

### Curl installer (all platforms)

```sh
curl -fsSL https://raw.githubusercontent.com/ghchinoy/riptide/main/scripts/install.sh | sh
```

The script detects OS/arch, downloads the correct tarball, verifies the SHA-256
checksum, and installs to `/usr/local/bin` (or `~/.local/bin` as a fallback).

### Homebrew (when the tap is set up)

```sh
brew install ghchinoy/tap/riptide
```

### Manual

Download the appropriate archive from the
[Releases page](https://github.com/ghchinoy/riptide/releases), verify the
checksum in `checksums.txt`, and copy the `riptide` binary to a directory on
your `PATH`.

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| GoReleaser not found locally | `go install github.com/goreleaser/goreleaser/v2@latest` |
| `git describe` returns empty | Ensure at least one tag exists: `git tag v0.0.0` |
| Homebrew formula not updated | Check `HOMEBREW_TAP_GITHUB_TOKEN` secret is set |
| Release workflow fails on tests | Fix the failing tests; do NOT skip |
