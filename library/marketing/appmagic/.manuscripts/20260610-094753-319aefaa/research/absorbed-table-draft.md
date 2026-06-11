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

### Web-tier candidates (optional secondary source; unofficial /api/v2 surface; pending user approval at gate)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| W1 | Hourly top charts with rank-change diffs | AppMagic web app only | appmagic-pp-cli web hourly-tops (hand-code, APPMAGIC_WEB_TOKEN) | intraday movement; no official equivalent |
| W2 | Monetization Intelligence: in-game IAP offer library | AppMagic web app only | appmagic-pp-cli web offers (hand-code) | unique dataset; no API equivalent anywhere |
| W3 | Live-ops event counts by tag | AppMagic web app only | appmagic-pp-cli web liveops-tags (hand-code) | market-level live-ops benchmarking |
| W4 | App counts per tag | AppMagic web app only | appmagic-pp-cli web tag-counts (hand-code) | market sizing by niche |
