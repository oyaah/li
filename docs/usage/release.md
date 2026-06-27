# Release Runbook

This runbook is for maintainers cutting a public `li` release.

## Public Release Shape

First public releases are macOS/Homebrew-first:

```sh
brew tap oyaah/tap
brew install --cask oyaah/tap/li
```

GitHub release artifacts may include Linux and Windows binaries, but do not call those auth paths fully supported until they have fresh live validation.

## Prerequisites

- GitHub repo: `github.com/oyaah/li`
- Homebrew tap repo: `github.com/oyaah/homebrew-tap`
- Go toolchain matching `go.mod`
- GoReleaser installed locally for snapshot checks
- GitHub token with permission to publish releases and update the tap
- Chrome installed for live auth smoke tests

## Local Preflight

Run these before tagging:

```sh
go test ./...
go vet ./...
go build -o ./li .
./li version
./li --help
```

Run a snapshot release:

```sh
goreleaser release --snapshot --clean
```

Inspect `dist/` for archives and checksums. Do not commit `dist/`.

## Live Smoke Test

Use a real account you can afford to risk.

```sh
./li login
./li doctor --json
./li who yash-bansal-b506bb302 --json
./li search "founder" --json
```

Record:

- Date
- Platform and architecture
- Chrome version
- `li version`
- `doctor` probe result
- Any endpoint drift or auth recovery path used

Write-command smoke tests are optional and side-effecting. Only run them when the target account and recipient are approved for testing.

### Smoke Record Template

```text
Date:
Release candidate:
Install method:
Platform:
Architecture:
Chrome version:
li version:
doctor result:
who result:
search result:
Auth recovery path used, if any:
Side-effecting write checks run:
Decision: ship / do not ship
Notes:
```

## Tag Release

Use semver tags:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The release workflow runs GoReleaser, publishes GitHub artifacts, and updates the Homebrew cask in the tap.

## Post-Release Verification

From a clean shell:

```sh
brew update
brew install --cask oyaah/tap/li
li version
li doctor
```

Check the GitHub release page for archives, checksums, and release notes. Check the tap repository for the updated Homebrew cask.

## Rollback

If the release is broken:

1. Mark the GitHub release as pre-release or delete the bad release if no users should consume it.
2. Revert or fix the tap cask.
3. Publish a patch tag.
4. Call out the broken version in `CHANGELOG.md`.
