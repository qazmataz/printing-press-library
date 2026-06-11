# AppMagic CLI Brief

## API Identity
- Domain: Mobile + Steam app-market intelligence (downloads/revenue estimates, top charts, trending, soft launches, publisher rankings, app history time series, ASO/ASA keyword intel, ad-creative intel, live-ops calendars, SDK intel, B2B contacts). Vendor: AppMagic (appmagic.rocks), acquired by Sensor Tower (announced 2026-05-12), positioned as Sensor Tower's SMB market-intelligence product.
- Users: UA managers, game studio analysts, ASO specialists, market researchers, investors/analysts. Any paid AppMagic subscriber worldwide; API access is sales-led (no self-serve plan mapping published).
- Data profile: Estimates and rankings keyed by store (1=Google Play, 2=iOS iPhone, 3=iPad) + country (alpha-2 plus group codes WW/W1/E1...), date series from 2015-01-01, "united" cross-store entities (united applications / united publishers), 500+ tag genre taxonomy, categories, Steam apps. Responses negotiate JSON or CSV via Accept header.

## Canonical Spec Source
- **Official Swagger 2.0 spec, publicly downloadable without auth: `https://api.appmagic.rocks/swagger.json`** (fetched 2026-06-10, 339,254 bytes; title "Appmagic API" v1.0.0; basePath `/v1`; schemes https; NO `host` field — inject `api.appmagic.rocks`).
- 116 paths / 117 operations (72 GET, 45 POST). Tags: applications(16), charts(13), history(13), steam(12), tops(9), Ad Intelligence(6), united applications(6), united publishers(6), asa(5), aso(5), live-ops(5), sdkint(5), publishers(4), period comparison(3), categories(2), tags(2), contacts(2), featuring(1), keywords(1), last date(1).
- Human docs: `https://api.appmagic.rocks/v1/docs` (ReDoc) + `/v1/docs/interactive` (Swagger UI).
- Vendor also ships an official MCP endpoint at `https://api.appmagic.rocks/mcp` (JSON-RPC 2.0, Streamable HTTP) exposing every operation as a tool — documented inside the spec's info.description with a `claude mcp add` example.
- Marketing page says "50+ endpoints... tailored to your team's needs" → per-contract endpoint-group entitlement gating is expected.

## Auth
- **HTTP Basic only** (`securityDefinitions: BasicAuth`, applied globally). Credentials = AppMagic **account login:password** — no bearer token, no API key, no self-serve token issuance. Confirmed by spec text ("Authorization: Basic base64(login:password)") and the rappmagic wrapper's `am_auth(name, password)`.
- Canonical env vars for the CLI: `APPMAGIC_LOGIN` + `APPMAGIC_PASSWORD` (two-var Basic pair, Twilio-style `x-auth-env-vars`). No community convention exists; we set it.
- NOTE (this run): no credential is available — operator explicitly declined to use any company credential. All live verification phases run uncredentialed; Phase 5 live dogfood auto-skips per skill rules.

## Reachability Risk
- **None.** No WAF/bot protection anywhere on `api.appmagic.rocks`: unauthenticated sweep of all 72 GET paths (2026-06-10) returned clean API responses: 65× `401 {"message":"unauthorized"}`, 1× `200` (`/last-date`, returns `{"last_date":"2026-06-08"}` openly), 4× `403` (`/steam/top`, `/steam/app/info`, `/steam/app/united`, `/period-comparison/last-date` — entitlement-gated groups), 2× `422` (`/tops/applications`, `/applications/reviews` — validate params before auth). Sweep artifact: `research/unauth-probe-sweep.json`.
- Probe-safe endpoint used: `GET /last-date` (open, no auth) — natural doctor/liveness check.
- Tier/permission hints from 4xx body: 403 bodies observed on Steam + period-comparison groups even unauthenticated, consistent with "endpoints tailored to your team's needs" contract gating.
- Runtime transport: `standard_http`. No Surf, no clearance cookies, no browser anything.

## Two API Surfaces — keep strictly separate
- `api.appmagic.rocks/v1` (THIS CLI): official, documented, Basic auth, spec-driven.
- `appmagic.rocks/api/v2` (web app XHR): private SPA surface behind Google/FB/email OAuth sessions + anti-bot (2022 scraper evidence: browser automation was required). NOT replayable under Basic auth. Excluded from this CLI by design. Web-only modules (Monetization Intelligence offer library, Feature Library UI screens, App Tracker/Slack alerts, App Transfers, Success Meter, custom dashboards) are therefore out of scope.

## Top Workflows
1. Competitor tracking: resolve app → united app → download/revenue/DAU/MAU/retention history; compare countries/stores.
2. Top-charts monitoring: free/grossing/trending/soft-launch charts by store+country+category/tag; spot movers and new entries.
3. Market sizing: tops + history aggregated by tag/category/country; period comparison (vs-previous-period growth).
4. Publisher intelligence: publisher portfolios, united publisher rollups, top publishers by downloads/revenue.
5. ASO/ASA intel: keyword terms, store search rankings, Apple Search Ads terms (entitlement permitting).
6. Ad-creative intel: Ad Intelligence endpoints — creatives, impressions, networks (entitlement permitting).
7. Steam crossover: PC analog of the mobile views (12 endpoints; entitlement-gated).

## Table Stakes
- Full official endpoint coverage (117 ops) — the only existing wrapper (muzerow/rappmagic, R, stale 2024) covers ~34; we cover all, typed.
- JSON + CSV output (API-native CSV via Accept header — distinctive; most competitors are JSON-only).
- Search → resolve → fetch pipelines: by name, by store IDs (POST search-by-ids endpoints), united-entity resolution.
- Top charts + rank history + estimates + reviews + screenshots + release notes + similarity (per-app).
- Local taxonomy cache (categories + 500+ tags) for offline lookup of tag/category IDs.
- Rate-limit visibility: X-RateLimit-Limit/Remaining/Reset + X-Concurrent-Requests-Allowed + 429 Retry-After handling (numeric quotas are plan-dependent and unpublished).

## Data Layer
- Primary entities: united_applications, applications (per-store), publishers, united_publishers, categories, tags, top-chart snapshots (sort×store×country×date), history series per united app, steam apps (entitled accounts).
- Sync cursor: `/last-date` (open endpoint) = data freshness watermark; daily granularity.
- FTS/search: app names, publisher names, tag names; offline tag/category ID resolution.

## Codebase Intelligence
- Source: muzerow/rappmagic (R, MIT, ~35 functions; STALE — 5 endpoints' methods changed from GET to POST since 2024; use only as tertiary cross-check for param semantics, never methods/shapes).
- Auth: `Basic base64(name:password)`; wrapper defaults `Accept: text/csv`, supports gzip.
- Rate limiting: headers documented per-op in spec; numbers plan-specific.
- Architecture insight: "united" entities are AppMagic's cross-store unification primitive — most analytical endpoints key on united IDs; store-level endpoints exist for resolution and store-specific detail.

## User Vision
- Operator's stated vision: "Build the complete printing-press CLI for AppMagic... it doesn't have [company]-only data, it is a global market-intelligence tool. Anyone with paid access of AppMagic, anywhere in the world, should be able to use it. Build without any internal skill content or internal tokens. Publishing to the printing-press library with no trace of the operator's employer."
- Implications: clean-room build from public sources only (this brief's sources are all public URLs); generic two-var Basic auth via env vars; no internal defaults; live-credentialed testing skipped this run.

## Product Thesis
- Name: `appmagic-pp-cli` (library slug `appmagic`).
- Why it should exist: AppMagic's official API is MCP-native but raw — 117 stateless operations, no local state, no diffing, no entitlement awareness, plan-gated groups that simply 403. No CLI exists anywhere (the only wrapper is a stale R package). A printed CLI adds: offline SQLite mirror + FTS over apps/publishers/taxonomy, chart-snapshot diffing ("who entered the top 50 since last week"), soft-launch watchlists, entitlement probing ("which of the 20 endpoint groups does MY contract include"), rate-limit budget awareness, CSV-native bulk export, and composable agent-native output — none of which the official MCP endpoint mirror can do.

## Community Signal (last30days run, 2026-06-10)
- Zero organic Reddit/HN chatter about AppMagic or the acquisition in the last 30 days (32 Reddit threads + 4 HN stories swept; all entity-misses). The conversation lives in trade press (PocketGamer.biz, Gamesforum) and LinkedIn. Implication: this CLI's users are practitioners, not hobbyists; docs and agent-ergonomics matter more than community memes.
- G2/OMR review complaints (fresh): (1) retention-estimate accuracy is doubted - users cross-check against other tools; (2) data updates lag for very recent dates; (3) valuable features are gated to higher tiers without upfront transparency. All three map directly to CLI features: cross-source sanity surface, freshness watermark (/last-date) surfaced everywhere, and entitlement probing that tells you WHAT your plan includes.
- PocketGamer.biz ("Why Sensor Tower acquired rival AppMagic"): AppMagic remains a standalone SMB product; no sunset planned. Lowers (not eliminates) API-churn risk.
- apimonster.io ships a "Datamagic (Appmagic) API" connector - external automation demand is real; no CLI exists anywhere.

## Web-App Surface Map (operator-authorized docs, scrubbed)
The operator authorized using their own endpoint documentation of the appmagic.rocks web XHR surface (`/api/v2`, Bearer token from the web session's `localStorage.datamagic.token`). Validated highlights NOT present in the official v1 API:
- `GET /top/hourly-apps` - hourly rank charts with rank-change diffs (official API has no hourly granularity)
- `POST /monetization-intelligence/offers` - in-game IAP offer library (80+ games; structure, duration, pricing)
- `POST /feature-intelligence/live-ops/count-by-tags` - live-ops event counts aggregated by tag
- `GET /tags/apps-count` - app counts per tag (market sizing)
- `GET /top/united-apps` - top charts with exact download/revenue figures per entry
- Known shapes: `requests[]` batch envelope on chart POSTs; `is_united` flag semantics; store 4=iOS+iPad, 5=All (web-only store codes); per-endpoint casing quirks (camelCase GET params, snake_case POST bodies, `dateStart` exceptions); ad-spend requires object-form app_ids
- Auth/lifecycle: Bearer token expires; refresh by re-grabbing from a logged-in browser session. Per-account paid-tier needed for exact figures (free tier returns rounded buckets).
- Caveats: several web endpoints are unvalidated or have unknown enum values (period-comparison sort fields, custom-apps ID type); ASO/ASA + creatives dmint POST shapes partially unknown; this surface is unofficial and can change without notice.
- Decision pending at Phase 1.5 gate: ship an optional `web` command group (secondary source, separate `APPMAGIC_WEB_TOKEN` env var, clearly marked unofficial) vs official-API-only v1.

## Build Priorities
1. Generate from official swagger.json (inject host; Swagger 2.0 → convert to OpenAPI 3 if the generator requires it; verify with `--validate`).
2. Foundation: store + sync for taxonomy (categories/tags), united apps/publishers; `/last-date` freshness watermark in doctor.
3. Absorb: full 117-op surface, JSON/CSV negotiation, search→resolve→history pipelines.
4. Transcend: entitlement probe, chart diffing, soft-launch watch, rate-limit budget, market rollups, similarity graph walk (final list via Phase 1.5 subagent + user gate).
5. Entitlement-aware UX: distinguish 401 (bad creds) / 403 (group not in contract) / 429 (quota) with actionable messages; never present a 403 as a bug.
6. Configurable base URL (`APPMAGIC_BASE_URL` or config) — Sensor Tower acquisition makes endpoint-host churn a real risk.
