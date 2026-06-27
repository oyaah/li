# li — LinkedIn CLI — Requirements

**Date:** 2026-06-27
**Status:** Brainstorm complete → ready for `/ce-plan`
**Scope tier:** Deep — product (greenfield)

## Problem

LinkedIn from the terminal, for free, without weight. Every existing option fails at least one
constraint: official OAuth API is gated to near-uselessness (no people search, no arbitrary
profile read, no messaging on free tier); paid cloud wrappers (Linked API, Composio managed)
cost money; browser-automation tools (Selenium/Playwright stealth) drag in Chrome and are slow
and fat. Nothing is simultaneously **free, lightweight, fast, and actually working**.

## Solution

`li` — a single static Go binary that talks to LinkedIn's internal **Voyager API** using the
user's own session cookies. No browser, no LLM, no paid service, no MCP. Shaped after `gogcli`:
one binary, `--json` / `--plain` TSV output, data on stdout / human text on stderr, sysexit
exit codes, OS-keyring credential storage.

## Primary user & use

A single power user (the author) wanting full personal LinkedIn control from the terminal —
"swiss-army." Balanced read + write, not volume outreach.

## Goals

- Free of cost, forever — no API keys to buy, no cloud.
- Single static binary, instant startup, tiny memory. No runtime deps.
- Fast and scriptable — clean stdout for piping, predictable exit codes.
- Not token-hungry — terse default output; this is a human/script tool, not an agent feed.
- Simple and intuitive — `li <verb> <arg>`, ~8 commands, no config ceremony to start.

## Non-goals (product identity — explicitly NOT this)

- **No MCP mode.** Opposite of the requirement.
- **No browser automation.** No Chrome, no Playwright, no mouse emulation.
- **No paid cloud / no official OAuth.** Free Voyager path only.
- **No LLM in the loop.**
- **No volume-outreach tooling** (drip campaigns, lead lists, CRM sync).

## v1 command set (all ship in v1)

| Command | Action |
|---------|--------|
| `li login` | Paste `li_at` + `JSESSIONID` once → store in OS keyring. Capture real session headers. |
| `li who <publicId>` | View profile: name, headline, current role. |
| `li search <query>` | People search (`--title`, `--company` filters). |
| `li jobs <query>` | Job search (`--location`). |
| `li connect <id> [--note]` | Send connection invite. **Highest-risk action.** |
| `li msg <id> "text"` | Send a message. |
| `li post "text"` | Post an update to own feed. |
| `li inbox` | List recent conversations. |

## Ban-safety design (the core design decision)

**Chosen approach: A + one borrow from B.**

- **Spine (A):** every write action sleeps a randomized jitter (target 45–90s); a small local
  rate ledger (in the config/state dir) tracks per-window action counts; session headers and
  user-agent are cloned from the real browser session captured at `login`.
- **Borrow (B):** before the single riskiest action (`connect`), fire one "warm-up" GET that a
  real browser would make (load the target profile) so the invite isn't a cold isolated call.
- **Rejected — full B** (warm-up GETs before *every* action): too many calls, more drift
  surface, creeps toward the weight we're avoiding.
- **Rejected — C** (no in-tool pacing): timing density is exactly LinkedIn's ban trigger.

**Posture: warn + soft-block** (confirmed).
- Auto-jitter always on for writes.
- Warn on stderr as the user approaches a cap.
- Soft-block past hard caps (e.g. ~100 invites/week — LinkedIn's 2023 cap, still in effect)
  unless `--force`.
- Caps configurable; defaults set ~20% below known-safe thresholds.

## Success criteria

- `login` → `who` → `search` round-trips against a live account on first install, single binary,
  no extra setup.
- Cold-start to first output is sub-second.
- Write actions auto-pace and the ledger correctly warns/soft-blocks at caps.
- Output pipes cleanly (`li search x --json | jq`) with stable keys and correct exit codes.

## Key risks & assumptions

- **Voyager drift (durability bet):** endpoints are undocumented and LinkedIn changes/hardens
  them. The tool *will* break periodically. Mitigation: mirror the `tomquirk/linkedin-api`
  endpoint map as reference, pin/version the endpoint set, and **fail loud on schema drift**
  (no silent fallback, no fabricated output) so breakage is obvious, not silently wrong.
- **ToS / ban risk:** Voyager use violates LinkedIn ToS and risks account restriction. Docs must
  state this plainly and recommend a throwaway-tolerant account.
- **Header fidelity:** ban-safety leans on faithfully cloning the real session's headers/UA at
  login; if these go stale, detection risk rises.

## Deferred (not v1)

- `li schema --json` runtime command discovery.
- Multi-account named profiles / switching.
- Extra output formatters beyond `--json` / `--plain` (table, csv, yaml).

## Sources / prior art reviewed

- `bcharleson/linkedincli` — Voyager + cookie auth model (the engine; ignore its MCP half).
- `tomquirk/linkedin-api` — reference endpoint map for Voyager (mirror this).
- `gogcli` (openclaw/gogcli) — single-binary shape: `--json`/`--plain`, stdout/stderr split,
  keyring, sysexit codes, schema discovery.
- `jackwener/opencli` — polish ideas: sysexit codes, format multiplexing, named profiles
  (engine itself too heavy — rejected).
- Linked-API / Composio — paid/official, rejected as off-constraint.
- Stealth/automation repos (AmmarAR97, joeygoesgrey, stealthscraper, obscura) — browser-grade
  stealth is heavy; informed the "cheap 80% of safety" pacing approach instead.
