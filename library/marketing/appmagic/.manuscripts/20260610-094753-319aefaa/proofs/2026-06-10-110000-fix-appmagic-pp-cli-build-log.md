# appmagic-pp-cli Build Log

Manifest transcendence rows: 12 planned, 0 built. Phase 3 will not pass until all 12 ship.

(12 = 8 novel features + 4 approved web-tier commands, all hand-code. The 117 absorbed
official operations are generator-emitted and verified by the generated quality gates.)

## Phase 2 notes
- Generated from the official spec (converted Swagger 2.0 -> OpenAPI 3, host injected,
  BasicAuth enriched with x-auth-env-vars [APPMAGIC_LOGIN, APPMAGIC_PASSWORD]).
- Cloudflare MCP pattern auto-applied (117 endpoint tools > 50): code orchestration,
  hidden endpoint tools, [stdio,http] transport.
- 5 bare-array POST bodies fall back to --body-json (search-by-ids family) - expected.
- Headline shortened to 76 chars after observing truncation on root.go/SKILL/goreleaser;
  root.go + mcp/tools.go hand-synced to match (regen kept stale copies).
- Git identity set to personal noreply (the printer's personal GitHub noreply address (qazmataz))
  BEFORE first commit; re-done after --force regen wiped .git. No employer traces.

## Phase 3 infra (hand-authored, committed 07e5cc0)
- internal/store/appmagic_migrations.go: chart_snapshots, soft_launch_sightings,
  watchlist, entitlement_probes tables (lazy EnsureAppmagicTables).
- internal/source/webapi/client.go: unofficial web-surface client (Bearer
  APPMAGIC_WEB_TOKEN, AdaptiveLimiter, RateLimitError on 429 exhaustion,
  HTML-response detection, 401 token-expiry hint).
- internal/cli/appmagic_resolve.go: resolveUnitedApp / unitedAppsByIDs /
  parseUnitedApps / stripStorePrefix / resolveTagID.

## Key API facts discovered during design
- Official API has NO requests[] batch envelope (web surface only); fan-outs are
  parallel single calls with partial-failure accounting.
- GET /tops/united-applications rows carry only {place, united_application_id, value};
  names need search-by-ids enrichment.
- GET /history/united-applications returns daily downloads/revenue/rank rows - the
  workhorse for watchlist report, tag-rollup, liveops-overlay.
- POST /retention-v2 is single-app {store_application_id, store, country, dates}.

## Phase 3 completion (2026-06-10)
Manifest transcendence rows: 12 planned, 12 built. Per-row Cobra resolution: 14/14 leaf
commands resolve (incl. watchlist add/list). Dogfood novel_features_check: planned=12,
found=12, missing=[], skipped=false -> GATE PASS.
- 5 parallel agents implemented against PHASE3_DESIGN.md; all returned build+tests green
  with behavioral assertions (diff correctness, first-seen upsert, verdict classification,
  median/band-edge epsilon, mover deltas, calendar+catalog join, web response parsing).
- Spec-driven deviations (documented by agents): /history/united-applications takes a
  single date (per-day fan-out with batched ids); retention store enum is [1,2,3];
  /aso/by_app POST body {appIds, countries, count, dateFrom, dateTo}, position field
  medianPlace; live-ops event dates come from /live-ops/live-ops-calendar joined with
  /live-ops/live-ops labels.
- internal/source/webapi got a real test file (Bearer/params, 401-hint, HTML-shell
  detection, typed RateLimitError on 429 exhaustion).
