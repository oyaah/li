# li

`li` is a free, lightweight LinkedIn CLI for personal terminal use. It talks to LinkedIn's internal Voyager web API with your own logged-in LinkedIn session, stores that session in the OS keyring, and keeps command output script-friendly.

This is unofficial software. Voyager is not a public API, LinkedIn can break it at any time, and using it may violate LinkedIn's Terms of Service or restrict your account. Use an account you can afford to risk.

## Install

Homebrew is the intended public install path:

```sh
brew tap oyaah/tap
brew install li
```

Until the first public release is tagged, you can build from source:

```sh
go install github.com/oyaah/li@latest
```

Or clone and build locally:

```sh
go test ./...
go build -o li .
```

## Quickstart

```sh
li login
li doctor
li who yash-bansal-b506bb302
```

`li login` opens Chrome, lets LinkedIn handle normal login, validates `/voyager/api/me`, and stores the LinkedIn session in your OS keyring. "Continue with Google" is fine as a way to log into LinkedIn, but `li` never stores Google tokens.

## Commands

```sh
li login
li doctor
li who <publicId-or-profile-url>
li search "founder" --title CEO --company OpenAI
li jobs "backend engineer" --location "San Francisco"
li inbox
li msg <publicId-or-profile-url> "hello"
li connect <publicId-or-profile-url> --note "short note"
li post "shipping from the terminal"
li version
```

Write commands are auto-paced. `connect`, `msg`, and `post` use a local safety ledger, jitter, and soft-blocks so accidental bursts are harder to trigger. `--force` overrides a soft-block, but that does not make the action safe.

## Output

Data goes to stdout. Human text, progress, warnings, and errors go to stderr.

```sh
li search "founder" --json | jq
li inbox --plain | cut -f1
```

`--json` emits stable JSON for scripts. `--plain` emits TSV rows without headers.

## Auth and Recovery

Normal users should only need:

```sh
li login
```

If Google rejects the controlled Chrome window, use:

```sh
li login --system-browser
```

If you already have a logged-in Chrome profile and need a recovery path:

```sh
li login --real-chrome --browser-profile "Profile 1"
```

Cookie import and manual cookie flags exist for debugging only. See [docs/usage/auth.md](docs/usage/auth.md).

## Doctor

`li doctor` checks whether the stored session and pinned Voyager endpoints still work:

```sh
li doctor --json
```

Run it before filing bugs. If `doctor` says an endpoint drifted, LinkedIn probably changed the web API and the pinned schema needs an update.

## Privacy

`li` stores LinkedIn session material in your OS keyring and uses a controlled Chrome profile under your app config directory for browser-assisted auth. It does not store Google tokens, and it does not require any LLM, agent, cloud service, or paid API key.

## Release Status

The first public release is macOS/Homebrew-first. Linux and Windows binaries may be published as GitHub artifacts, but the Chrome/keyring auth path is currently tested hardest on macOS.

Maintainer release steps live in [docs/usage/release.md](docs/usage/release.md).

## License

MIT. See [LICENSE](LICENSE).
