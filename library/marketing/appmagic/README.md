# AppMagic CLI

**Full AppMagic API coverage plus chart history, deltas, and entitlement maps.**

AppMagic's official API is 117 stateless operations behind HTTP Basic auth; the only public client anywhere is a stale R wrapper. This CLI covers the full surface with typed commands and adds what statelessness cannot: snapshot history for chart-diff and soft-launch-radar, a persistent competitor watchlist, cohort-median retention benchmarking, cross-tag market rollups, and an entitlements probe that maps exactly which endpoint groups your contract includes.

## Install

The recommended path installs both the `appmagic-pp-cli` binary and the `pp-appmagic` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install appmagic
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install appmagic --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install appmagic --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install appmagic --agent claude-code
npx -y @mvanhorn/printing-press-library install appmagic --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/appmagic/cmd/appmagic-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/appmagic-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install appmagic --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-appmagic --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-appmagic --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install appmagic --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/appmagic-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in both `APPMAGIC_LOGIN` and `APPMAGIC_PASSWORD` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/appmagic/cmd/appmagic-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "appmagic": {
      "command": "appmagic-pp-mcp",
      "env": {
        "APPMAGIC_LOGIN": "<your-login>",
        "APPMAGIC_PASSWORD": "<your-password>"
      }
    }
  }
}
```

</details>

## Authentication

AppMagic has no separate API token: the API authenticates with your account login and password over HTTP Basic. Set APPMAGIC_LOGIN and APPMAGIC_PASSWORD in your environment. API access itself is a sales-led contract feature, and endpoint groups (Steam, ASA, contacts, period comparison) are entitlement-gated per contract; run 'entitlements' to map what your account includes. The optional 'web' command group uses a different credential: APPMAGIC_WEB_TOKEN, the Bearer token a logged-in appmagic.rocks browser session stores in localStorage under 'datamagic.token'. That surface is unofficial and the token expires periodically.

## Quick Start

```bash
# Health check: config and connectivity work before any credentials are set
appmagic-pp-cli doctor --dry-run

# Mirror the 500+ tag genre taxonomy locally so tag IDs resolve offline
appmagic-pp-cli sync --resources categories,tags

# Map which endpoint groups your contract includes before relying on any of them
appmagic-pp-cli entitlements --json

# After two runs on different days, see who entered or dropped in the chart
appmagic-pp-cli chart-diff --sort grossing --store 2 --country US

# The weekly competitor pull as one batched call instead of N tabs
appmagic-pp-cli watchlist report --country US --metrics downloads,revenue --since 7d --agent

```

## Unique Features

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

Summed 30-day revenue for two genres in Japan without a spreadsheet.

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

## Usage

Run `appmagic-pp-cli --help` for the full command reference and flag list.

## Commands

### ad-spend

Manage ad spend

- **`appmagic-pp-cli ad-spend`** - Retrieves information about the amount of money (in dollars) spent on advertising for specified applications.

### adint

Manage adint

- **`appmagic-pp-cli adint get-ad-source-values`** - Retrieves a list of available values for adSource parameter. Use /adint/filter_values instead.
- **`appmagic-pp-cli adint get-ads-stats`** - Retrieves aggregated data over the specified time period with impressions breakdown by country/region, source, networks and platforms.
- **`appmagic-pp-cli adint get-application-ads`** - Retrieves links to ad creatives for the selected app over the selected period of time. Ad creatives can be sorted by date of introduction or by their impressions score.
- **`appmagic-pp-cli adint get-creative-stats`** - Retrieves all the data required to produce a popup window with the selected ad creative: the app it advertises, the actual creative with its videos and images, and its impressions history and breakdown by country/region, networks, and apps where the selected creative was shown.
- **`appmagic-pp-cli adint get-filter-values`** - Valid values for filtering in Ad Intelligence endpoints
- **`appmagic-pp-cli adint get-series`** - Retrieves Ad Intelligence series data (impressions over time) for selected countries and creative IDs.

### app-categories

Manage app categories

- **`appmagic-pp-cli app-categories`** - Internal usage only

### app-publishers

Manage app publishers

- **`appmagic-pp-cli app-publishers`** - Internal usage only

### app-tags

Manage app tags

- **`appmagic-pp-cli app-tags`** - Internal usage only

### app-transfers

Manage app transfers

- **`appmagic-pp-cli app-transfers`** - Retrieves the list of apps that have been assigned to the selected publisher's account.

### applications

Applicaitons and application info

- **`appmagic-pp-cli applications add`** - Submits an array of app IDs that are missing from our system. In 48 hours or less, their data will be added.
- **`appmagic-pp-cli applications get`** - Retrieves the App Info data for the apps searched by their names or name prefixes.
- **`appmagic-pp-cli applications get-app-info-by-country`** - Retrieves application info by country
- **`appmagic-pp-cli applications get-app-info-by-country-releases`** - Retrieves info on application releases by country
- **`appmagic-pp-cli applications get-by-ids`** - Retrieves the App Info data for the array of apps with selected IDs.
- **`appmagic-pp-cli applications get-release-notes`** - Retrieves information about releases on the selected date for an array of apps.
- **`appmagic-pp-cli applications get-reviews`** - Retrieves raw user reviews for a given application from app stores. Supports filtering by country, date range (YYYY-MM-DD), and exact star rating (1-5). Results are ordered by review date descending.
- **`appmagic-pp-cli applications get-store`** - Retrieves the App Info data by an app's store ID.

### apps-downloads

Manage apps downloads

- **`appmagic-pp-cli apps-downloads`** - Internal usage only

### arpdau

Manage arpdau

- **`appmagic-pp-cli arpdau`** - Returns Average Revenue Per Daily Active User over time.

### asa

Manage asa

- **`appmagic-pp-cli asa get-by-app`** - Retrieves a list of top terms by place for each of given apps.
- **`appmagic-pp-cli asa get-by-keyword`** - Retrieves a list of top apps by share ov voice for each of given keywords.
- **`appmagic-pp-cli asa get-dates`** - Retrieves the date range for which ASA data is available in our system.
- **`appmagic-pp-cli asa get-filter-values`** - Retrieves the list of countries that can be used to filter data in the /asa endpoints.
- **`appmagic-pp-cli asa get-series`** - Retrieves a list of share of voice series for given filter split by all possible filter values combinations.

### aso

Manage aso

- **`appmagic-pp-cli aso get-by-app`** - Retrieves a list of top terms by place for each of given apps.
- **`appmagic-pp-cli aso get-by-keyword`** - Retrieves a list of top apps by place for each of given keywords.
- **`appmagic-pp-cli aso get-dates`** - Retrieves the date range for which ASO data is available in our system.
- **`appmagic-pp-cli aso get-filter-values`** - Retrieves the list of countries that can be used to filter data in the /aso endpoints.
- **`appmagic-pp-cli aso get-series`** - Retrieves a list of place series for given filter split by all possible filter values combinations.

### categories

Application categories

- **`appmagic-pp-cli categories`** - Retrieves a list of categories with names and IDs for respective stores.

### contacts

Manage contacts

- **`appmagic-pp-cli contacts get-companies`** - Retrieves contact details of employees of the selected company.
- **`appmagic-pp-cli contacts get-profiles`** - Retrieves a list of employees of the selected company that can be further filtered to include or exclude specific positions.

### country-downloads

Manage country downloads

- **`appmagic-pp-cli country-downloads`** - Internal usage only

### dau

Manage dau

- **`appmagic-pp-cli dau`** - Retrieves the number of daily active unique users. If aggregation is set to 'week' or 'month', average DAU for this period will be returned. The API returns data as a list of pairs (arrays with two values). Each pair represents a point in time and a corresponding value.

### featuring

Manage featuring

- **`appmagic-pp-cli featuring`** - Retrieves data on featuring of an application in App Store or Google Play for a specific date

### history

Returns data set for the history of requested app for the entire time (starting from January 1, 2015).

- **`appmagic-pp-cli history get-application`** - Retrieves all data for one specific app for the selected period of time.
- **`appmagic-pp-cli history get-applications`** - Retrieves data for up to 100 apps for the selected date. The array of app IDs is transmitted to the body with information such as date, aggregation, category, and country/region. All parameters are compulsory except for aggregation and category. Aggregation is daily by default. For any other aggregation values, the date is rounded to the beginning of the time period.
- **`appmagic-pp-cli history get-dau`** - Retrieves dau for a list of applications for a specific date with daily aggregation.
- **`appmagic-pp-cli history get-dau-for-united-applications`** - Retrieves dau for a list of associated applications for a specific date. Data aggregation is daily.
- **`appmagic-pp-cli history get-mau`** - Retrieves mau for a list of applications for a specific date rounded to the beginning of the month.
- **`appmagic-pp-cli history get-mau-for-united-applications`** - Retrieves mau for a list of associated applications for a specific date. Data aggregation is monthly. The date from a request rounds to the start of month.
- **`appmagic-pp-cli history get-retention`** - Retrieves retention data for a list of applications for a specific date.
- **`appmagic-pp-cli history get-session-stats`** - Retrieves session stats for a list of applications for a specific date with daily aggregation.
- **`appmagic-pp-cli history get-session-stats-for-united-applications`** - Retrieves session stats for a list of associated applications for a specific date. Data aggregation is daily.
- **`appmagic-pp-cli history get-united-application`** - Retrieves all data for one specific associated app for the selected period of time.
- **`appmagic-pp-cli history get-united-applications`** - Retrieves data for up to 100 associated apps for the selected date. The set of app IDs is fed into the body with information such as date, aggregation, category, and country/region. All parameters are compulsory except for aggregation and category. Aggregation is daily by default. For any other aggregation values, the date is rounded to the beginning of the time period.
- **`appmagic-pp-cli history get-united-retention`** - Retrieves retention data for a list of associated applications for a specific date.

### keywords

Manage keywords

- **`appmagic-pp-cli keywords`** - Retrieves a list of users' keywords. Only available with access to Ad Intelligence.

### last-date

Manage last date

- **`appmagic-pp-cli last-date`** - Retrieves the most recent date for which all metrics (e.g., revenue, downloads) are available in our system.

### last-versions

Manage last versions

- **`appmagic-pp-cli last-versions`** - Get

### live-ops

Manage live ops

- **`appmagic-pp-cli live-ops get`** - Events info from LiveOps & Updates Calendar
- **`appmagic-pp-cli live-ops get-calendar`** - Dates of events and updates from LiveOps & Updates Calendar
- **`appmagic-pp-cli live-ops get-games`** - List of games covered in LiveOps & Updates Calendar
- **`appmagic-pp-cli live-ops get-tags-values`** - Retrieves a list of available tag values in /live-ops/live-ops response
- **`appmagic-pp-cli live-ops get-updates`** - Updates info from LiveOps & Updates Calendar

### market-segments

Manage market segments

- **`appmagic-pp-cli market-segments`** - Retrieves aggregated data for the selected market segment, such as store, country/region, tags, etc., as dynamic variables over time.

### mau

Manage mau

- **`appmagic-pp-cli mau`** - Retrieves the number of monthly active unique users. The API returns data as a list of pairs (arrays with two values). Each pair represents a point in time and a corresponding value.

### period-comparison

Manage period comparison

- **`appmagic-pp-cli period-comparison get-custom-apps`** - Returns period-over-period metrics for a specific list of apps identified by their united application IDs.
- **`appmagic-pp-cli period-comparison get-last-date`** - Returns the most recent date for which period comparison data is available.
- **`appmagic-pp-cli period-comparison get-top-apps`** - Returns a ranked list of apps with their metrics compared across two time periods.

### publishers

Publisher details

- **`appmagic-pp-cli publishers get`** - Retrieves publishers by their names or name prefixes.
- **`appmagic-pp-cli publishers get-store`** - Retrieves publishers by their store IDs.

### retention

Manage retention

- **`appmagic-pp-cli retention`** - Deprecated.Use /retention-v2 instead

### retention-v2

Manage retention v2

- **`appmagic-pp-cli retention-v2`** - Retrieves application retention data for 1, 7, 14, 30, 60, 90, 180 and 360 days periods. Retention reflects the classic retention metric (the share of users to run the app again the next day/week/2 weeks/month/...), estimated as an average for users from the weekly installation cohort.

### sdkint

Manage sdkint

- **`appmagic-pp-cli sdkint get-app-sdks`** - Retrieves a list of SDKs for the app by its ID as parameter.
- **`appmagic-pp-cli sdkint get-apps-by-sdk-changes`** - Retrieves a list of apps that added or removed the selected SDKs over the specified period of time. The search can be narrowed down to a pre-transmitted array of app IDs.
- **`appmagic-pp-cli sdkint get-apps-by-sdks`** - Retrieves a list of apps that do or do not have the selected SDKs. The search can be narrowed down to a pre-transmitted array of app IDs.
- **`appmagic-pp-cli sdkint get-pubs-by-sdks`** - Retrieves a list of publishers that do or do not have the selected SDKs. The search can be narrowed down to a pre-transmitted array of publisher IDs.
- **`appmagic-pp-cli sdkint get-sdks`** - Retrieves a list of all the SDKs in our system.

### session-stats

Manage session stats

- **`appmagic-pp-cli session-stats`** - Retrieves information about session length and average number of sessions for specified applications.

### steam

Manage steam

- **`appmagic-pp-cli steam get-app-info`** - Steam app info
- **`appmagic-pp-cli steam get-app-metrics-by-countries`** - Retrieves metrics (downloads and revenue) by countries for a specific Steam application.
- **`appmagic-pp-cli steam get-categories`** - Retrieves category values for steam applications
- **`appmagic-pp-cli steam get-genres`** - Retrieves genre values for steam applications
- **`appmagic-pp-cli steam get-last-date`** - Retrieves the most recent date for which steam data is available in our system.
- **`appmagic-pp-cli steam get-metrics-chart`** - Steam metrics chart
- **`appmagic-pp-cli steam get-retention-chart`** - Steam retention chart
- **`appmagic-pp-cli steam get-tags`** - Retrieves tag values for steam applications
- **`appmagic-pp-cli steam get-top`** - Retrieves the list of top free, top grossing and top wishlisted applications for the specified country and date.
- **`appmagic-pp-cli steam get-united-apps`** - Steam united apps
- **`appmagic-pp-cli steam get-user-activity-chart`** - Steam user activity chart
- **`appmagic-pp-cli steam get-wishlist-chart`** - Steam wishlist chart

### tags

Application tags

- **`appmagic-pp-cli tags`** - Retrieves a list of all our tags.

### tops

Returns top-placing apps sorted by specified metric for specified day, country, store and category.

- **`appmagic-pp-cli tops advanced-search`** - Retrieves search results by multiple criteria, including keywords in both app names and descriptions, release dates and periods, threshold metric values, etc.
- **`appmagic-pp-cli tops get-applications`** - Retrieves data on any app Top Chart (top free, top grossing, top featuring) for any country/region and date. Top Charts are comprised of native apps only (AppStore and Google Play); however, as opposed to native store charts, you can customize your aggregation dates.
- **`appmagic-pp-cli tops get-apps`** - Retrieves data on any app Top Chart (top free, top grossing, top featuring) for any country/region and date. Top Charts are comprised of native apps only (AppStore and Google Play); however, as opposed to native store charts, you can customize your aggregation dates. The value in the response represents the corresponding metric for each chart: revenue for Top Grossing, number of downloads for Top Free and featuring score for Top Featuring.
- **`appmagic-pp-cli tops get-ltv`** - Get ltv
- **`appmagic-pp-cli tops get-publishers`** - Retrieves data on any publisher Top Chart (top free, top grossing, top featuring) for any country/region and any date. Top Charts are comprised of native apps only (AppStore and Google Play); however, as opposed to native store charts, you can customize your aggregation dates.
- **`appmagic-pp-cli tops get-soft-launches`** - Retrieves a list of apps for the selected genre that are currently in their soft launch stage and were launched in the specified time period; you can add and remove apps. The 'only_global_publisher' parameter filters out apps launched by publishers with no other apps in their portfolios.
- **`appmagic-pp-cli tops get-trending`** - Retrieves a list of apps that have ranked up in the selected top chart for the selected country (or WW) over the specified time period. Use the 'new_only' parameter to additionally filter out apps that were released earlier or later than the specified time period.
- **`appmagic-pp-cli tops get-united-applications`** - Retrieves data on any associated app Top Chart (top free, top grossing, top featuring) for any country/region and date.You can customize your aggregation dates.
- **`appmagic-pp-cli tops get-united-apps`** - Retrieves data on any associated app Top Chart (top free, top grossing, top featuring) for any country/region and date.You can customize your aggregation dates.

### united-applications

United applications

- **`appmagic-pp-cli united-applications get`** - Retrieves the App Info data for the associated apps searched by their names or name prefixes.
- **`appmagic-pp-cli united-applications get-by-app-ids`** - Retrieves the App Info data for the associated apps identified by their store IDs.
- **`appmagic-pp-cli united-applications get-by-ids`** - Retrieves the App Info data for the array of associated apps with our internal IDs.
- **`appmagic-pp-cli united-applications get-unitedapplications`** - Retrieves the App Info data for the associated app with our internal ID.

### united-dau

Manage united dau

- **`appmagic-pp-cli united-dau`** - Retrieves the number of daily active unique users for a united application. If aggregation is set to 'week' or 'month', average DAU for this period will be returned. The API returns data as a list of pairs (arrays with two values). Each pair represents a point in time and a corresponding value.

### united-mau

Manage united mau

- **`appmagic-pp-cli united-mau`** - Retrieves the number of monthly active unique users for a united application. The API returns data as a list of pairs (arrays with two values). Each pair represents a point in time and a corresponding value.

### united-publishers

United publisher details

- **`appmagic-pp-cli united-publishers get`** - Retrieves associated publishers by their names or name prefixes.
- **`appmagic-pp-cli united-publishers get-by-ids`** - Retrieves a set of identified associated publishers based on the array of our internal IDs.
- **`appmagic-pp-cli united-publishers get-lifetime-data`** - Retrieves lifetime revenue and downloads for the selected associated publisher in the specified country/region (or WW). The search is carried out by their IDs and can include more than one publisher at the same time.
- **`appmagic-pp-cli united-publishers get-unitedpublishers`** - Retrieves an identified associated publisher based on our internal ID.

### united-retention

Manage united retention

- **`appmagic-pp-cli united-retention`** - Deprecated.Use /united-retention-v2 instead

### united-retention-v2

Manage united retention v2

- **`appmagic-pp-cli united-retention-v2`** - Retrieves retention data for 1, 7, 14, 30, 60, 90, 180 and 360 days periods for an associated app. Retention reflects the classic retention metric (the share of users to run the app again the next day/week/2 weeks/month), estimated as an average for users from the weekly installation cohort.

### web-shop-revenue

Manage web shop revenue

- **`appmagic-pp-cli web-shop-revenue`** - Returns D2C (web shop) revenue chart data for a single application. Same as D2C revenue in AppMagic.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
appmagic-pp-cli app-categories

# JSON for scripting and agents
appmagic-pp-cli app-categories --json

# Filter to specific fields
appmagic-pp-cli app-categories --json --select id,name,status

# Dry run — show the request without sending
appmagic-pp-cli app-categories --dry-run

# Agent mode — JSON + compact + no prompts in one flag
appmagic-pp-cli app-categories --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - `--idempotent`: treat already-exists (HTTP 409) responses as a successful no-op
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
appmagic-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/appmagic-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `APPMAGIC_LOGIN` | per_call | Yes | AppMagic account login (HTTP Basic, required). |
| `APPMAGIC_PASSWORD` | per_call | Yes | Set to your API credential. |
| `APPMAGIC_WEB_TOKEN` | per_call | No | Bearer token for the unofficial `web` commands; copy from a logged-in appmagic.rocks session, localStorage key 'datamagic.token' (optional). |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `appmagic-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `appmagic-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $APPMAGIC_LOGIN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the relevant group's list/search command (e.g. `united-applications get --search <name>`) to find valid IDs.

### API-specific
- **401 unauthorized on every call** — Set APPMAGIC_LOGIN and APPMAGIC_PASSWORD (your AppMagic account credentials; there is no separate API token). Confirm your contract includes API access.
- **403 forbidden on steam, asa, contacts, or period-comparison commands** — That endpoint group is not in your contract. Run `appmagic-pp-cli entitlements --refresh` to map your access; contact AppMagic to extend the contract.
- **429 too many requests** — You hit the plan's daily quota or concurrency cap. Back off and retry after the quota resets; quotas are plan-dependent (the API returns X-RateLimit-* headers and Retry-After on 429).
- **chart-diff or soft-launch-radar returns empty** — These read snapshot history that the commands capture themselves: run the command on two different days (each run stores a snapshot), or pass --from/--to to pick stored dates. `sync` does not populate these tables.
- **web commands fail with 401** — APPMAGIC_WEB_TOKEN expired. Log into appmagic.rocks in a browser and copy the token from localStorage key 'datamagic.token'.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sensortower-mcp**](https://github.com/virusimmortal00/sensortower-mcp) — Python (16 stars)
- [**rappmagic**](https://github.com/muzerow/rappmagic) — R (3 stars)
- [**appfigures-cli**](https://github.com/appfigures/cli) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
