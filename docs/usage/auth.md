# Auth

`li login` is the normal setup path.

It opens Chrome, lets LinkedIn handle login, validates `/voyager/api/me`, then stores the LinkedIn session in the OS keyring. Google SSO is fine as a way to log into LinkedIn, but `li` never stores or uses Google tokens.

For public installs, the expected path is:

```sh
brew tap oyaah/tap
brew install li
li login
li doctor
```

## Recommended Flow

```sh
li login
li who <publicId>
```

If Google says the controlled browser is not secure:

```sh
li login --system-browser
```

That opens normal Chrome for Google SSO, then imports and validates the fresh LinkedIn session from Chrome.

If cookie replay still returns `401`, use the real-profile CDP fallback:

1. Log into LinkedIn in normal Chrome.
2. Quit Chrome completely.
3. Run:

```sh
li login --real-chrome --browser-profile "Profile 1"
```

Use the Chrome profile directory, not the display name. On macOS these live under:

```text
~/Library/Application Support/Google/Chrome/
```

## Debug Paths

```sh
li login --from-browser
li login --manual --li-at '<value>' --jsessionid '"ajax:..."'
```

These exist for debugging and recovery. They are not the main product path.
