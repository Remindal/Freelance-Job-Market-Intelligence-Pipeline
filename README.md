# Scout

A personal job-hunting intelligence tool that runs entirely on your own machine. It monitors the public search pages of mainstream freelance platforms, filters new postings with rule-based pre-screening + LLM scoring, pushes high-score matches to Telegram, and presents everything in a native desktop dashboard built with Wails.

## Why it works this way

**The fetcher problem.** The target sites have mature anti-scraping protection, so naive approaches fail in various ways. Scout takes the compliant route: it attaches to **your own already-running browser** over the Chrome DevTools Protocol (`chromium.ConnectOverCDP`) and reads pages through your own session. Access is read-only and low-frequency (one round every ≥15 minutes), and the access pattern is identical to your own normal browsing — same browser, same profile, same session. You start the browser once via a shortcut that carries `--remote-debugging-port=9222` and a dedicated profile directory; Scout opens a tab per feed, reads, and closes just the tab.

**Extraction without extra requests.** Job cards are parsed from the DOM (all CSS selectors live in one file, `internal/fetcher/selectors.go`), while client-quality signals (payment verification, historical spend, rating, publish time, proposal tier) are read from the page's embedded framework state by an in-page script — no additional HTTP requests beyond the page loads you would make yourself.

**Progressive cost funnel.** Every stage is cheaper than the next, and nothing skips the queue:

```
fetch → dedupe (URL fingerprint; known jobs never reach the LLM)
      → keyword rules (word-boundary matching, so "logo" ≠ "go")
      → client-quality rules (unverified & zero-spend posters, stale posts)
      → LLM scoring (0-100 + rationale + tags, JSON contract with one retry)
      → activity re-check on the detail page for high scorers only (rate-limited; dead posts get killed before notifying)
      → notify (score ≥ threshold)
```

The scoring prompt encodes a fixed candidate profile plus explicit veto rules (non-development work, marketing-in-disguise, niche platforms requiring prior experience, seniority gates, budget mismatches, scam patterns), tuned for a new account optimizing for small, winnable first jobs. The profile, prompt template, keyword lists, and thresholds all live in `configs/config.yaml`.

**Operability.** Runs show live progress in the dashboard (per-feed and per-score counters) with a cancel button; manual runs share a single-flight lock with the cron schedule, and a second request while running is queued rather than rejected.

## Stack

- **Go** — pipeline, SQLite store (`modernc.org/sqlite`, pure Go), cron scheduler
- **Wails v2** — native window + system tray, Go↔TS bindings (no HTTP layer anywhere)
- **React + TypeScript + Tailwind + ECharts** — dashboard with stats, filters, tag filtering, live progress
- **OpenAI-compatible LLM client** — any endpoint works (DeepSeek, SiliconFlow, etc.)
- **playwright-go** — used only as a CDP client; it never launches a browser

## Layout

```
main.go                   composition root only (wires config, store, fetcher, pipeline, scheduler, app)
internal/
  domain/                 Job entity, statuses — zero dependencies
  fetcher/                Fetcher interface + CDP implementation; selectors.go (all selectors),
                          parse.go (pure parsers, unit-tested against real captured page fixtures)
  filter/                 keyword rules, client-quality rules, LLM scorer
  llm/                    OpenAI-compatible client + scoring prompt (profile injected from config)
  store/                  Store interface + SQLite impl (auto-migrating schema)
  notify/                 Telegram (plain HTTP, no SDK)
  pipeline/               the only orchestrator; single-flight + 1-deep queue + cancel support
  scheduler/              robfig/cron wrapper
  desktop/                Wails binding layer exposed to the frontend
frontend/                 React SPA (MemoryRouter, TanStack Query, event-driven refresh)
scripts/                  build.ps1 (one-command build) / start.ps1 (one-command launch)
```

Key invariants: `domain` depends on nothing; SQL lives only in `store/sqlite.go`; `pipeline` is the only place that knows the whole flow; everything tunable lives in `configs/config.yaml`.

## Running it

Prerequisites: a Chrome install you can dedicate a profile to.

```powershell
# one-time: playwright driver (CDN mirrors shown are for CN networks)
$env:PLAYWRIGHT_GO_NPM_REGISTRY="https://registry.npmmirror.com"
$env:NODE_MIRROR="https://npmmirror.com/mirrors/node/"
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6100.0 install chromium

# config
cp configs/config.example.yaml configs/config.yaml   # fill in LLM api_key, telegram (optional), feed URLs

scripts\build.ps1    # produces build/bin/scout.exe
scripts\start.ps1    # launches the CDP-enabled browser (if needed) + the desktop app
```

Config and database resolve relative to the executable, so `build/bin/` is portable as a folder.

## Tests

```powershell
go test ./...
```

Covers the store (dedupe, filters, pagination, stats), rule engine (word boundaries, budgets), scorer (JSON contract incl. `tags`, retry fallback), parsers (real captured fixtures: search cards and detail pages), client/activity rules, and the pipeline queue (single-flight + queued re-run).

## Notes

- `configs/config.yaml` is gitignored — keep your API keys out of git. Environment variables `SCOUT_LLM_API_KEY`, `SCOUT_TG_TOKEN`, `SCOUT_TG_CHAT_ID` override the file.
- Personal use only: low-frequency (≥15 min/round), read-only access through your own browser session.
