# AppMagic Absorb Manifest

### Absorbed (match or beat everything that exists)

Sources absorbed: muzerow/rappmagic (R wrapper, 35 fns, the ONLY existing AppMagic client), AppMagic's own vendor MCP at api.appmagic.rocks/mcp (endpoint mirror of all 117 ops), virusimmortal00/sensortower-mcp (43 tools; adjacent feature benchmark), appfigures/cli (`af`, the only vendor-official CLI in the category; ergonomics benchmark), AppMagic web app (operator-documented surface).

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Full official API coverage: applications (search, by-store-id, info, reviews, screenshots, release-notes, similarity, competitors, search-by-ids) | rappmagic (subset) / vendor MCP | (generated endpoint) applications * | typed flags, --json/--csv/--select, offline mirror |
| 2 | United (cross-store) app entities: search, by-id, search-by-app-ids, LTV, data-countries | rappmagic / vendor MCP | (generated endpoint) united-applications * | local cache of united-ID resolution |
| 3 | Publishers + united publishers: search, by-id, portfolio apps, lifetime-data, data-countries | rappmagic / vendor MCP | (generated endpoint) publishers *, united-publishers * | portfolio queries offline |
| 4 | Top charts: application/united, publishers, trending, soft-launches, LTV, advanced-search | rappmagic am_top_* / sensortower-mcp get_top_and_trending, get_top_publishers | (generated endpoint) tops * | snapshot-able into SQLite for diffing |
| 5 | Downloads/revenue estimates per app over time | sensortower-mcp get_download_estimates, get_revenue_estimates | (generated endpoint) history application(s), united-application(s) | CSV-native bulk via Accept header |
| 6 | DAU/MAU/WAU usage series | sensortower-mcp get_usage_active_users | (generated endpoint) charts dau/mau/united-dau/united-mau + history dau/mau | batch requests envelope |
| 7 | Retention curves (D1-D90+) | sensortower-mcp app_analysis_retention | (generated endpoint) charts retention-v2/united-retention-v2 + history retention | per-store vs united split |
| 8 | Session stats, ARPDAU, ad-spend, web-shop revenue series | AppMagic web app charts (operator docs) / official charts tag | (generated endpoint) charts session-stats/arpdau/ad-spend/web-shop-revenue | metrics the ST MCP does NOT have at SMB price |
| 9 | Ad Intelligence: creatives, series, stats, application-ads, ad-source values, filter values | sensortower-mcp get_creatives/get_impressions | (generated endpoint) adint * | network/country breakdowns |
| 10 | ASO keywords: by app, by keyword, series, dates, filter values | sensortower-mcp keywords tools / AppTweak API | (generated endpoint) aso * | |
| 11 | ASA (Apple Search Ads) intel: by app, by keyword, series, dates, filter values | MobileAction SearchAds API | (generated endpoint) asa * | |
| 12 | Live-ops calendar, events, games, updates, tag values | AppMagic web app (LiveOps calendar) | (generated endpoint) live-ops * | unique in category at API level |
| 13 | SDK intelligence: SDKs taxonomy, apps by SDK, publishers by SDK, changed apps | 42matters Search-by-SDK / sensortower get_app_metadata include_sdk_data | (generated endpoint) sdkint * | |
| 14 | Steam analytics: top, app info/united, metrics by country, charts (metrics/retention/user-activity/wishlist), categories/genres/tags, last-date | AppMagic web app Steam module | (generated endpoint) steam * | PC+mobile in ONE CLI; no competitor MCP/CLI has this |
| 15 | Period comparison: top apps, custom apps, last-date | AppMagic web app Period Comparison | (generated endpoint) period-comparison * | growth analysis without spreadsheet |
| 16 | Categories + tags taxonomy (500+ game tags) | rappmagic am_categories/am_tags | (generated endpoint) categories, tags + synced into SQLite | offline tag-ID resolution + FTS |
| 17 | B2B contacts: company by id, profiles search | AppMagic web app Contacts | (generated endpoint) contacts * | entitlement-gated; honest 403 UX |
| 18 | Featuring data + keywords/all + app-transfers + last_versions + apps_downloads + country_downloads | official spec misc | (generated endpoint) featuring, keywords, misc | |
| 19 | Data freshness watermark | rappmagic am_last_date | (generated endpoint) last-date | open endpoint; doctor liveness check without creds |
| 20 | JSON + CSV output everywhere | rappmagic (Accept: text/csv default) | (behavior in appmagic-pp-cli root flags) --csv maps to API-native Accept: text/csv on passthrough, framework CSV elsewhere | spreadsheet-ready bulk pulls |
| 21 | Local SQLite mirror + sync + FTS search + raw SQL | printing-press framework (no competitor has this for AppMagic) | (behavior in appmagic-pp-cli sync/search/sql) taxonomy, united apps/publishers, chart snapshots | offline; compounds over time |
| 22 | MCP server exposure of every command | vendor MCP (endpoint mirror) / sensortower-mcp | (behavior in appmagic-pp-cli mcp) Cobra-tree MCP; >50 endpoint tools so Cloudflare search+execute pattern | beats vendor MCP: local store + novel commands + bounded context |
| 23 | Agent-context/doctor/health ergonomics | appfigures cli (af doctor-style UX) | (behavior in appmagic-pp-cli doctor) auth check, entitlement summary, rate-limit headers, /last-date freshness | category-first |
| 24 | Rate-limit visibility + 429 Retry-After handling | sensor-tower-mcp-pro (multi-token failover) | (behavior in appmagic-pp-cli doctor + client) X-RateLimit-* surfaced; adaptive limiter on 429 | plan quotas are opaque; we make them visible |

### Web-tier (APPROVED at gate 2026-06-10: secondary source, unofficial appmagic.rocks/api/v2 surface, Bearer APPMAGIC_WEB_TOKEN, shipping scope)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| W1 | Hourly top charts with rank-change diffs | AppMagic web app only | appmagic-pp-cli web hourly-tops | intraday movement; no official equivalent (hand-code) |
| W2 | Monetization Intelligence: in-game IAP offer library | AppMagic web app only | appmagic-pp-cli web offers (hand-code) | unique dataset; no API equivalent anywhere |
| W3 | Live-ops event counts by tag | AppMagic web app only | appmagic-pp-cli web liveops-tags (hand-code) | market-level live-ops benchmarking |
| W4 | App counts per tag | AppMagic web app only | appmagic-pp-cli web tag-counts (hand-code) | market sizing by niche |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Chart diff | chart-diff --sort grossing --store 2 --country US | hand-code | Diffs two top-chart snapshots in local SQLite; the API is stateless and keeps no chart history for you. (9/10, persona: Deniz) | Use this command to see rank movement, new entrants, and dropouts between two synced top-chart snapshots for any chart sort. Do NOT use it to track newly soft-launched titles across test markets; use 'soft-launch-radar' instead. Do NOT use it for market-size aggregates by tag; use 'tag-rollup' instead. |
| 2 | Soft-launch radar | soft-launch-radar --countries PH,CA,AU --since 30d | hand-code | First-seen index across test-market snapshots in local SQLite; the soft-launches endpoint only returns today's list. (9/10, persona: Deniz) | Use this command to find newly detected soft-launch titles with first-seen dates per test market, optionally filtered by publisher or tag. Do NOT use it for general top-chart rank movement; use 'chart-diff' instead. |
| 3 | Competitor watchlist report | watchlist report --country US --metrics downloads,revenue,retention --since 7d (with watchlist add/list) | hand-code | Persistent local competitor set + batched charts requests[] envelope replaces N per-app pulls and a hand-built spreadsheet. (8/10, persona: Mara) | Use this command to pull a side-by-side weekly metrics table for your saved competitor set. Do NOT use it to benchmark one app against its genre cohort median; use 'retention-benchmark' instead. |
| 4 | Entitlements probe | entitlements [--refresh] | hand-code | Synthesizes a per-endpoint-group access map (200/401/403/429) into SQLite; no API endpoint reports what your contract includes. (9/10, persona: Priya/all) | none |
| 5 | Tag rollup | tag-rollup --tags merge-2,match-3 --country JP --metric revenue --period 30d | hand-code | Joins synced tag taxonomy + tops + batched charts to sum market size per tag; the API has no cross-tag aggregation endpoint. (8/10, persona: Tomas) | Use this command for aggregate market sizing (summed downloads or revenue) across one or more tags. Do NOT use it to list which apps moved within a chart; use 'chart-diff' instead. |
| 6 | Retention benchmark | retention-benchmark "Royal Match" --tag match-3 --country US --top 20 | hand-code | Cohort selection from taxonomy + one batched retention pull + local median; answers the G2 "can I trust this retention number" complaint. (10/10, persona: Tomas/Mara) | Use this command to compare one app's retention curve against the median of its tag cohort. Do NOT use it for multi-metric comparisons across your saved competitor set; use 'watchlist report' instead. |
| 7 | ASO movers | aso-movers <app> --country US --store 2 --since 7d [--dataset asa] | hand-code | Computes gained/lost/moved keyword ranks between two dates; the API returns positions, never deltas. (8/10, persona: Priya) | none |
| 8 | Live-ops overlay | liveops-overlay "Royal Match" --country WW --since 90d | hand-code | Date-window join of live-ops events against revenue/downloads series; live-ops at API level is unique to AppMagic in this category. (8/10, persona: Mara/Deniz) | Use this command to correlate a single app's live-ops events with its revenue or download movement. Do NOT use it for plain cross-app metric comparison; use 'watchlist report' instead. |

### Killed candidates (audit trail in research/2026-06-10-novel-features-brainstorm.md)
similar-walk (episodic), quota (absorbed row 24 covers), freshness report (doctor owns watermarks), publisher-momentum (framework analytics covers), steam-crossover (entitlement-gated + episodic), sdk-watch (monthly cadence, thin wrapper), asa-movers (merged into aso-movers --dataset asa), resolve (already absorbed).
