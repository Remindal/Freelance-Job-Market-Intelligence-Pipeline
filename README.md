# Upwork Scout

A personal Upwork job radar that runs on your own machine. It periodically reads Upwork search pages through your **real, logged-in Chrome** (CDP attach), filters jobs with rule-based pre-screening + LLM scoring, pushes high-score jobs to Telegram, and presents everything in a native desktop dashboard built with Wails.

## Why it works this way

**The fetcher problem.** Upwork sits behind Cloudflare. Plain HTTP (RSS) gets 403; headless browsers get killed at the TLS-fingerprint layer; even a fully patched Playwright gets stuck in "Just a moment" loops because Cloudflare detects the CDP automation channel itself. The only thing that reliably works is the browser you already use every day — real profile, real login, real trust history.

So instead of launching a browser, Scout **attaches to one** via `chromium.ConnectOverCDP`. You start Chrome once with `--remote-debugging-port=9222` and a dedicated profile directory (with your Upwork login persisted in it). Scout opens a tab per feed, reads the page, closes the tab. Your browser stays yours; no automation fingerprint is ever presented.

**Extraction without extra requests.** Job cards are parsed from the DOM (all CSS selectors live in one file, `selectors.go`), while client-quality signals (payment verified, total spent, rating, publish time, proposal tier) are lifted from the page's embedded Nuxt payload by an in-page script — zero additional HTTP requests, so nothing about the traffic pattern looks unusual.

**Progressive cost funnel.** Every stage is cheaper than the next and nothing skips the queue:

```
fetch → dedupe (URL fingerprint, old jobs never reach the LLM)
      → keyword rules (word-boundary matching, so "logo" ≠ "go")
      → client-quality rules (unverified+$0 spend, stale posts)
      → LLM scoring (0-100 + Chinese rationale + tags, JSON contract with one retry)
      → activity re-check on detail page for high scorers only (rate-limited, kill dead posts)
      → notify (score ≥ threshold)
```

The LLM prompt encodes a fixed candidate profile and nine hard-veto rules (marketing-in-disguise jobs, niche platforms requiring prior experience, anti-scraping work, seniority gates, budget mismatches, scam patterns...), tuned for a new freelancer account optimizing for small, winnable first jobs.

## Stack

- **Go** — pipeline, SQLite store (`modernc.org/sqlite`, pure Go), cron scheduler
- **Wails v2** — native window + system tray, Go↔TS bindings (no HTTP layer anywhere)
- **React + TypeScript + Tailwind + ECharts** — dashboard with stats, filters, tag filtering, live fetch progress with a cancellable run
- **OpenAI-compatible LLM client** — any endpoint works (DeepSeek, SiliconFlow, etc.)
- **playwright-go** — used only as a CDP client; it never launches a browser

## Layout

```
cmd → main.go             composition root only (wires config, store, fetcher, pipeline, scheduler, app)
internal/
  domain/                 Job entity, statuses — zero dependencies
  fetcher/                Fetcher interface + CDP implementation + selectors.go (all selectors) + parse.go (pure parsers, unit-tested against real captured fixtures)
  filter/                 keyword rules, client-quality rules, LLM scorer
  llm/                    OpenAI-compatible client + scoring prompt (profile injected from config)
  store/                  Store interface + SQLite impl (auto-migrating schema)
  notify/                 Telegram (plain HTTP, no SDK)
  pipeline/               the only orchestrator: dedupe → rules → client filter → score → activity check → notify; single-flight with a 1-deep queue and cancel support
  scheduler/              robfig/cron wrapper
  desktop/                Wails binding layer exposed to the frontend
frontend/                 React SPA (MemoryRouter, TanStack Query, event-driven refresh)
scripts/                  build.ps1 (one-command build) / start.ps1 (one-command launch)
```

Key invariants: `domain` depends on nothing; SQL lives only in `store/sqlite.go`; `pipeline` is the only place that knows the whole flow; everything tunable (profile, prompt, keyword lists, thresholds) lives in `configs/config.yaml`.

## Running it

Prerequisites: a Chrome install you can dedicate a profile to.

```powershell
# one-time: playwright driver (CDN mirrors shown are for CN networks)
$env:PLAYWRIGHT_GO_NPM_REGISTRY="https://registry.npmmirror.com"
$env:NODE_MIRROR="https://npmmirror.com/mirrors/node/"
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6100.0 install chromium

# config
cp configs/config.example.yaml configs/config.yaml   # fill in LLM api_key, telegram (optional)

scripts\build.ps1    # produces build/bin/upwork-scout.exe
scripts\start.ps1    # launches the CDP Chrome (if needed) + the desktop app
```

Config and database resolve relative to the executable, so `build/bin/` is portable as a folder.

## Tests

```powershell
go test ./...
```

Covers the store (dedupe, filters, pagination, stats), rule engine (word boundaries, budgets), scorer (JSON contract incl. `tags`, retry fallback), parsers (real captured fixtures: search cards and detail pages), client/activity rules, and the pipeline queue (single-flight + queued re-run).

## Notes

- `configs/config.yaml` is gitignored — keep your API keys out of git. Environment variables `UPWORK_SCOUT_LLM_API_KEY`, `UPWORK_SCOUT_TG_TOKEN`, `UPWORK_SCOUT_TG_CHAT_ID` override the file.
- Personal tool, low-frequency polling (≥15 min/round). It reads public search pages through your own browser session; respect Upwork's ToS.
