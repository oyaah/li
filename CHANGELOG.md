# Changelog

All notable changes to `li` are documented here.

## Unreleased

## v0.1.1

- Fixed Homebrew cask installs on macOS by clearing downloaded binary metadata and ad-hoc signing during cask postflight.

## v0.1.0

- Added browser-assisted LinkedIn login through Chrome and OS keyring session storage.
- Added native Voyager HTTP reads with browser-context fallback when native replay is rejected.
- Added `doctor` endpoint drift checks.
- Added write safety for `connect`, `msg`, and `post`.
- Added `version` command for release metadata.
- Added GitHub/Homebrew release-readiness docs and packaging plan.
