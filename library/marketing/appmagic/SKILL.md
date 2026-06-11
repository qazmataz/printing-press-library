---
name: pp-appmagic
description: "Full AppMagic API coverage plus chart history, deltas, and entitlement maps. Trigger phrases: `top grossing games in the US`, `who entered the top charts this week`, `soft launches in the Philippines`, `competitor downloads and revenue`, `how big is the match-3 market`, `use appmagic`, `run appmagic`."
author: "Hamza Qazi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - appmagic-pp-cli
    install:
      - kind: go
        bins: [appmagic-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/appmagic/cmd/appmagic-pp-cli
---

# AppMagic — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `appmagic-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install appmagic --cli-only
   ```
2. Verify: `appmagic-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/appmagic/cmd/appmagic-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

AppMagic's official API is 117 stateless operations behind HTTP Basic auth; the only public client anywhere is a stale R wrapper. This CLI covers the full surface with typed commands and adds what statelessness cannot: snapshot history for chart-diff and soft-launch-radar, a persistent competitor watchlist, cohort-median retention benchmarking, cross-tag market rollups, and an entitlements probe that maps exactly which endpoint groups your contract includes.

## When to Use This CLI

Use this CLI when an agent needs mobile or Steam market intelligence from AppMagic: competitor downloads/revenue/retention pulls, top-chart movement, soft-launch scouting, genre sizing across the tag taxonomy, ASO/ASA keyword deltas, live-ops correlation, or SDK adoption. It shines for recurring analyst rituals where local snapshot history and a persistent watchlist compound over time.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for first-party analytics of your own app (use App Store Connect, Google Play Console, or your MMP; AppMagic figures are model estimates)
- Do not use it to scrape app stores directly; it only queries AppMagic's API with your contract's entitlements
- Do not expect web-app-only modules (custom dashboards, Feature Library UI screens, App Tracker alerts) beyond the four documented 'web' commands
- Do not use estimate data as ground truth for financial reporting; estimates are modeled, not measured

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Trust and transparency
- **`retention-benchmark`** — See whether an app's retention curve is normal for its genre by comparing it against the cohort median of its tag.

  _Reach for this when a retention estimate needs context before it goes in a report; single-app numbers are widely distrusted without a cohort baseline._

  ```bash
  appmagic-pp-cli retention-benchmark "Royal Match" --tag match-3 --country US --top 20 --agent
  ```
- **`entitlements`** — Discover exactly which AppMagic endpoint groups your contract includes, before a bare 403 surprises you.

  _Run this first on a new account; AppMagic endpoint groups are contract-gated and tier limits are not published._

  ```bash
  appmagic-pp-cli entitlements --refresh --json
  ```

### Chart history that compounds
- **`chart-diff`** — See who entered, dropped, or moved in any top chart between two synced snapshots.

  _Use this for the Monday movers report instead of hand-diffing two CSV exports._

  ```bash
  appmagic-pp-cli chart-diff --sort grossing --store 2 --country US --agent
  ```
- **`soft-launch-radar`** — Find newly detected soft-launch titles with first-seen dates per test market.

  _Use this to scout new titles entering test markets without remembering what was there last week._

  ```bash
  appmagic-pp-cli soft-launch-radar --countries PH,CA,AU --since 30d --agent
  ```
- **`watchlist report`** — Pull downloads, revenue, and retention for your saved competitor set in one side-by-side table.

  _Use this for the weekly competitor deck; the watchlist remembers the set so the agent only asks once._

  ```bash
  appmagic-pp-cli watchlist report --country US --metrics downloads,revenue --since 7d --agent
  ```

### Computed market intelligence
- **`tag-rollup`** — Aggregate market size (summed downloads or revenue) across one or more genre tags.

  _Use this for genre sizing questions like 'how big is match-3 in Japan' that otherwise need a spreadsheet._

  ```bash
  appmagic-pp-cli tag-rollup --tags match-3,merge-2 --country JP --metric revenue --period 30d --agent
  ```
- **`aso-movers`** — See which keywords an app gained, lost, or moved on between two dates.

  _Use this for weekly keyword reports; pass --dataset asa for Apple Search Ads terms._

  ```bash
  appmagic-pp-cli aso-movers com.dreamgames.royalmatch --country US --store 1 --since 7d --agent
  ```
- **`liveops-overlay`** — Correlate a competitor's live-ops events with their revenue and download movement.

  _Use this to answer 'did that event spike their revenue' without eyeballing two dashboards._

  ```bash
  appmagic-pp-cli liveops-overlay "Royal Match" --country WW --since 90d --agent
  ```

### Unofficial web intelligence (optional)
- **`web offers`** — Browse AppMagic's in-game IAP offer library (structure, duration, pricing) from the web app's Monetization Intelligence module.

  _Use this only with APPMAGIC_WEB_TOKEN set; it is an unofficial surface that can change without notice._

  ```bash
  appmagic-pp-cli web offers --store 1 --country US --limit 20 --json
  ```
- **`web hourly-tops`** — Intraday top-chart rankings with rank-change diffs from the web app surface.

  _Use this when daily charts are too coarse, e.g. launch-day tracking; requires APPMAGIC_WEB_TOKEN._

  ```bash
  appmagic-pp-cli web hourly-tops --store 2 --country US --top-depth 10 --json
  ```
- **`web liveops-tags`** — Market-level counts of live-ops events grouped by tag for a store and country.

  _Use this to benchmark how common an event type (gacha, daily, competitive) is across the market; requires APPMAGIC_WEB_TOKEN._

  ```bash
  appmagic-pp-cli web liveops-tags --store 1 --country US --since 30d --json
  ```
- **`web tag-counts`** — Number of apps per taxonomy tag for market-niche sizing.

  _Use this to gauge how crowded a niche is before deeper sizing with tag-rollup; requires APPMAGIC_WEB_TOKEN._

  ```bash
  appmagic-pp-cli web tag-counts --store 1 --country WW --json
  ```

## Command Reference

**ad-spend** — Manage ad spend

- `appmagic-pp-cli ad-spend` — Retrieves information about the amount of money (in dollars) spent on advertising for specified applications.

**adint** — Manage adint

- `appmagic-pp-cli adint get-ad-source-values` — Retrieves a list of available values for adSource parameter. Use /adint/filter_values instead.
- `appmagic-pp-cli adint get-ads-stats` — Retrieves aggregated data over the specified time period with impressions breakdown by country/region, source
- `appmagic-pp-cli adint get-application-ads` — Retrieves links to ad creatives for the selected app over the selected period of time.
- `appmagic-pp-cli adint get-creative-stats` — Retrieves all the data required to produce a popup window with the selected ad creative: the app it advertises
- `appmagic-pp-cli adint get-filter-values` — Valid values for filtering in Ad Intelligence endpoints
- `appmagic-pp-cli adint get-series` — Retrieves Ad Intelligence series data (impressions over time) for selected countries and creative IDs.

**app-categories** — Manage app categories

- `appmagic-pp-cli app-categories` — Internal usage only

**app-publishers** — Manage app publishers

- `appmagic-pp-cli app-publishers` — Internal usage only

**app-tags** — Manage app tags

- `appmagic-pp-cli app-tags` — Internal usage only

**app-transfers** — Manage app transfers

- `appmagic-pp-cli app-transfers` — Retrieves the list of apps that have been assigned to the selected publisher's account.

**applications** — Applicaitons and application info

- `appmagic-pp-cli applications add` — Submits an array of app IDs that are missing from our system. In 48 hours or less, their data will be added.
- `appmagic-pp-cli applications get` — Retrieves the App Info data for the apps searched by their names or name prefixes.
- `appmagic-pp-cli applications get-app-info-by-country` — Retrieves application info by country
- `appmagic-pp-cli applications get-app-info-by-country-releases` — Retrieves info on application releases by country
- `appmagic-pp-cli applications get-by-ids` — Retrieves the App Info data for the array of apps with selected IDs.
- `appmagic-pp-cli applications get-release-notes` — Retrieves information about releases on the selected date for an array of apps.
- `appmagic-pp-cli applications get-reviews` — Retrieves raw user reviews for a given application from app stores.
- `appmagic-pp-cli applications get-store` — Retrieves the App Info data by an app's store ID.

**apps-downloads** — Manage apps downloads

- `appmagic-pp-cli apps-downloads` — Internal usage only

**arpdau** — Manage arpdau

- `appmagic-pp-cli arpdau` — Returns Average Revenue Per Daily Active User over time.

**asa** — Manage asa (entitlement-gated; run `entitlements` first)

- `appmagic-pp-cli asa get-by-app` — Retrieves a list of top terms by place for each of given apps.
- `appmagic-pp-cli asa get-by-keyword` — Retrieves a list of top apps by share ov voice for each of given keywords.
- `appmagic-pp-cli asa get-dates` — Retrieves the date range for which ASA data is available in our system.
- `appmagic-pp-cli asa get-filter-values` — Retrieves the list of countries that can be used to filter data in the /asa endpoints.
- `appmagic-pp-cli asa get-series` — Retrieves a list of share of voice series for given filter split by all possible filter values combinations.

**aso** — Manage aso

- `appmagic-pp-cli aso get-by-app` — Retrieves a list of top terms by place for each of given apps.
- `appmagic-pp-cli aso get-by-keyword` — Retrieves a list of top apps by place for each of given keywords.
- `appmagic-pp-cli aso get-dates` — Retrieves the date range for which ASO data is available in our system.
- `appmagic-pp-cli aso get-filter-values` — Retrieves the list of countries that can be used to filter data in the /aso endpoints.
- `appmagic-pp-cli aso get-series` — Retrieves a list of place series for given filter split by all possible filter values combinations.

**categories** — Application categories

- `appmagic-pp-cli categories` — Retrieves a list of categories with names and IDs for respective stores.

**contacts** — Manage contacts (entitlement-gated; run `entitlements` first)

- `appmagic-pp-cli contacts get-companies` — Retrieves contact details of employees of the selected company.
- `appmagic-pp-cli contacts get-profiles` — Retrieves a list of employees of the selected company that can be further filtered to include or exclude specific

**country-downloads** — Manage country downloads

- `appmagic-pp-cli country-downloads` — Internal usage only

**dau** — Manage dau

- `appmagic-pp-cli dau` — Retrieves the number of daily active unique users.

**featuring** — Manage featuring

- `appmagic-pp-cli featuring` — Retrieves data on featuring of an application in App Store or Google Play for a specific date

**history** — Returns data set for the history of requested app for the entire time (starting from January 1, 2015).

- `appmagic-pp-cli history get-application` — Retrieves all data for one specific app for the selected period of time.
- `appmagic-pp-cli history get-applications` — Retrieves data for up to 100 apps for the selected date.
- `appmagic-pp-cli history get-dau` — Retrieves dau for a list of applications for a specific date with daily aggregation.
- `appmagic-pp-cli history get-dau-for-united-applications` — Retrieves dau for a list of associated applications for a specific date. Data aggregation is daily.
- `appmagic-pp-cli history get-mau` — Retrieves mau for a list of applications for a specific date rounded to the beginning of the month.
- `appmagic-pp-cli history get-mau-for-united-applications` — Retrieves mau for a list of associated applications for a specific date. Data aggregation is monthly.
- `appmagic-pp-cli history get-retention` — Retrieves retention data for a list of applications for a specific date.
- `appmagic-pp-cli history get-session-stats` — Retrieves session stats for a list of applications for a specific date with daily aggregation.
- `appmagic-pp-cli history get-session-stats-for-united-applications` — Retrieves session stats for a list of associated applications for a specific date. Data aggregation is daily.
- `appmagic-pp-cli history get-united-application` — Retrieves all data for one specific associated app for the selected period of time.
- `appmagic-pp-cli history get-united-applications` — Retrieves data for up to 100 associated apps for the selected date.
- `appmagic-pp-cli history get-united-retention` — Retrieves retention data for a list of associated applications for a specific date.

**keywords** — Manage keywords

- `appmagic-pp-cli keywords` — Retrieves a list of users' keywords. Only available with access to Ad Intelligence.

**last-date** — Manage last date

- `appmagic-pp-cli last-date` — Retrieves the most recent date for which all metrics (e.g., revenue, downloads) are available in our system.

**last-versions** — Manage last versions

- `appmagic-pp-cli last-versions` — Get

**live-ops** — Manage live ops

- `appmagic-pp-cli live-ops get` — Events info from LiveOps & Updates Calendar
- `appmagic-pp-cli live-ops get-calendar` — Dates of events and updates from LiveOps & Updates Calendar
- `appmagic-pp-cli live-ops get-games` — List of games covered in LiveOps & Updates Calendar
- `appmagic-pp-cli live-ops get-tags-values` — Retrieves a list of available tag values in /live-ops/live-ops response
- `appmagic-pp-cli live-ops get-updates` — Updates info from LiveOps & Updates Calendar

**market-segments** — Manage market segments

- `appmagic-pp-cli market-segments` — Retrieves aggregated data for the selected market segment, such as store, country/region, tags, etc.

**mau** — Manage mau

- `appmagic-pp-cli mau` — Retrieves the number of monthly active unique users. The API returns data as a list of pairs (arrays with two values).

**period-comparison** — Manage period comparison (entitlement-gated; run `entitlements` first)

- `appmagic-pp-cli period-comparison get-custom-apps` — Returns period-over-period metrics for a specific list of apps identified by their united application IDs.
- `appmagic-pp-cli period-comparison get-last-date` — Returns the most recent date for which period comparison data is available.
- `appmagic-pp-cli period-comparison get-top-apps` — Returns a ranked list of apps with their metrics compared across two time periods.

**publishers** — Publisher details

- `appmagic-pp-cli publishers get` — Retrieves publishers by their names or name prefixes.
- `appmagic-pp-cli publishers get-store` — Retrieves publishers by their store IDs.

**retention** — Manage retention

- `appmagic-pp-cli retention` — Deprecated.Use /retention-v2 instead

**retention-v2** — Manage retention v2

- `appmagic-pp-cli retention-v2` — Retrieves application retention data for 1, 7, 14, 30, 60, 90, 180 and 360 days periods.

**sdkint** — Manage sdkint

- `appmagic-pp-cli sdkint get-app-sdks` — Retrieves a list of SDKs for the app by its ID as parameter.
- `appmagic-pp-cli sdkint get-apps-by-sdk-changes` — Retrieves a list of apps that added or removed the selected SDKs over the specified period of time.
- `appmagic-pp-cli sdkint get-apps-by-sdks` — Retrieves a list of apps that do or do not have the selected SDKs.
- `appmagic-pp-cli sdkint get-pubs-by-sdks` — Retrieves a list of publishers that do or do not have the selected SDKs.
- `appmagic-pp-cli sdkint get-sdks` — Retrieves a list of all the SDKs in our system.

**session-stats** — Manage session stats

- `appmagic-pp-cli session-stats` — Retrieves information about session length and average number of sessions for specified applications.

**steam** — Manage steam (entitlement-gated; run `entitlements` first)

- `appmagic-pp-cli steam get-app-info` — Steam app info
- `appmagic-pp-cli steam get-app-metrics-by-countries` — Retrieves metrics (downloads and revenue) by countries for a specific Steam application.
- `appmagic-pp-cli steam get-categories` — Retrieves category values for steam applications
- `appmagic-pp-cli steam get-genres` — Retrieves genre values for steam applications
- `appmagic-pp-cli steam get-last-date` — Retrieves the most recent date for which steam data is available in our system.
- `appmagic-pp-cli steam get-metrics-chart` — Steam metrics chart
- `appmagic-pp-cli steam get-retention-chart` — Steam retention chart
- `appmagic-pp-cli steam get-tags` — Retrieves tag values for steam applications
- `appmagic-pp-cli steam get-top` — Retrieves the list of top free, top grossing and top wishlisted applications for the specified country and date.
- `appmagic-pp-cli steam get-united-apps` — Steam united apps
- `appmagic-pp-cli steam get-user-activity-chart` — Steam user activity chart
- `appmagic-pp-cli steam get-wishlist-chart` — Steam wishlist chart

**tags** — Application tags

- `appmagic-pp-cli tags` — Retrieves a list of all our tags.

**tops** — Returns top-placing apps sorted by specified metric for specified day, country, store and category.

- `appmagic-pp-cli tops advanced-search` — Retrieves search results by multiple criteria, including keywords in both app names and descriptions
- `appmagic-pp-cli tops get-applications` — Retrieves data on any app Top Chart (top free, top grossing, top featuring) for any country/region and date.
- `appmagic-pp-cli tops get-apps` — Retrieves data on any app Top Chart (top free, top grossing, top featuring) for any country/region and date.
- `appmagic-pp-cli tops get-ltv` — Get ltv
- `appmagic-pp-cli tops get-publishers` — Retrieves data on any publisher Top Chart (top free, top grossing, top featuring) for any country/region and any date.
- `appmagic-pp-cli tops get-soft-launches` — Retrieves a list of apps for the selected genre that are currently in their soft launch stage and were launched in the
- `appmagic-pp-cli tops get-trending` — Retrieves a list of apps that have ranked up in the selected top chart for the selected country (or WW)
- `appmagic-pp-cli tops get-united-applications` — Retrieves data on any associated app Top Chart (top free, top grossing, top featuring) for any country/region and date.
- `appmagic-pp-cli tops get-united-apps` — Retrieves data on any associated app Top Chart (top free, top grossing, top featuring) for any country/region and date.

**united-applications** — United applications

- `appmagic-pp-cli united-applications get` — Retrieves the App Info data for the associated apps searched by their names or name prefixes.
- `appmagic-pp-cli united-applications get-by-app-ids` — Retrieves the App Info data for the associated apps identified by their store IDs.
- `appmagic-pp-cli united-applications get-by-ids` — Retrieves the App Info data for the array of associated apps with our internal IDs.
- `appmagic-pp-cli united-applications get-unitedapplications` — Retrieves the App Info data for the associated app with our internal ID.

**united-dau** — Manage united dau

- `appmagic-pp-cli united-dau` — Retrieves the number of daily active unique users for a united application.

**united-mau** — Manage united mau

- `appmagic-pp-cli united-mau` — Retrieves the number of monthly active unique users for a united application.

**united-publishers** — United publisher details

- `appmagic-pp-cli united-publishers get` — Retrieves associated publishers by their names or name prefixes.
- `appmagic-pp-cli united-publishers get-by-ids` — Retrieves a set of identified associated publishers based on the array of our internal IDs.
- `appmagic-pp-cli united-publishers get-lifetime-data` — Retrieves lifetime revenue and downloads for the selected associated publisher in the specified country/region (or WW).
- `appmagic-pp-cli united-publishers get-unitedpublishers` — Retrieves an identified associated publisher based on our internal ID.

**united-retention** — Manage united retention

- `appmagic-pp-cli united-retention` — Deprecated.Use /united-retention-v2 instead

**united-retention-v2** — Manage united retention v2

- `appmagic-pp-cli united-retention-v2` — Retrieves retention data for 1, 7, 14, 30, 60, 90, 180 and 360 days periods for an associated app.

**web-shop-revenue** — Manage web shop revenue

- `appmagic-pp-cli web-shop-revenue` — Returns D2C (web shop) revenue chart data for a single application. Same as D2C revenue in AppMagic.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
appmagic-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Under `--agent` (or `--json`) the CLI exits `0` even when nothing matches — check the `.matches` array length instead of relying on exit codes. Only in terminal (non-JSON) mode does a no-confidence result exit `2` — fall back to `--help` or use a narrower query.

## Recipes

### Monday movers report

```bash
appmagic-pp-cli chart-diff --sort grossing --store 2 --country US --agent
```

Entered/dropped/moved in the US iPhone grossing chart since the previous snapshot, agent-shaped for a standup summary.

### Scout test markets

```bash
appmagic-pp-cli soft-launch-radar --countries PH,CA,AU --since 30d --json
```

First-seen soft-launch titles across the classic test markets in the last 30 days.

### Genre heat check

```bash
appmagic-pp-cli tag-rollup --tags match-3,merge-2 --country JP --metric revenue --period 30d
```

Summed 30-day revenue for two genres in Japan without a spreadsheet. Totals cover the top N apps per tag (default --top 20), not the whole market.

### Bounded competitor pull for agents

```bash
appmagic-pp-cli watchlist report --country US --metrics downloads,revenue --since 7d --agent --select apps.name,apps.downloads,apps.revenue
```

Watchlist metrics narrowed to three dotted fields so a deeply nested batch response does not flood agent context.

### Sanity-check a retention number

```bash
appmagic-pp-cli retention-benchmark "Royal Match" --tag match-3 --country US --top 20
```

The app's curve against its genre cohort median at D1/D7/D30.

## Auth Setup

AppMagic has no separate API token: the API authenticates with your account login and password over HTTP Basic. Set APPMAGIC_LOGIN and APPMAGIC_PASSWORD in your environment. API access itself is a sales-led contract feature, and endpoint groups (Steam, ASA, contacts, period comparison) are entitlement-gated per contract; run 'entitlements' to map what your account includes. The optional 'web' command group uses a different credential: APPMAGIC_WEB_TOKEN, the Bearer token a logged-in appmagic.rocks browser session stores in localStorage under 'datamagic.token'. That surface is unofficial and the token expires periodically.

Run `appmagic-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  appmagic-pp-cli app-categories --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — `--idempotent`: treat already-exists (HTTP 409) responses as a successful no-op

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
appmagic-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
appmagic-pp-cli feedback --stdin < notes.txt
appmagic-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/appmagic-pp-cli/feedback.jsonl`. They are never POSTed unless `APPMAGIC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `APPMAGIC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
appmagic-pp-cli profile save briefing --json
appmagic-pp-cli --profile briefing app-categories
appmagic-pp-cli profile list --json
appmagic-pp-cli profile show briefing
appmagic-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `appmagic-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/appmagic/cmd/appmagic-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add appmagic-pp-mcp -- appmagic-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which appmagic-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   appmagic-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `appmagic-pp-cli <command> --help`.
