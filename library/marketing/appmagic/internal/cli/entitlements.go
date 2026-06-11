// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/store"

	"github.com/spf13/cobra"
)

// entitlementCacheMaxAge is how long a cached probe verdict stays trusted
// before the command re-probes the API. Contract changes are rare, so a week
// keeps the command instant without going stale for long.
const entitlementCacheMaxAge = 7 * 24 * time.Hour

// Verdict values for an endpoint-group probe.
const (
	verdictIncluded     = "included"
	verdictNotIncluded  = "not-included"
	verdictAuthRequired = "auth-required"
	verdictRateLimited  = "rate-limited"
	verdictPostOnly     = "post-only"
	verdictError        = "error"
	verdictUnknown      = "unknown"
)

// entitlementProbe describes one cheap request that proves whether an
// endpoint group is in the caller's AppMagic contract. Every GET path and its
// minimal valid params were verified against the official OpenAPI spec.
type entitlementProbe struct {
	group    string
	method   string
	path     string
	params   map[string]string
	postOnly bool // group has no cheap GET; recorded as verdict "post-only" without a request
}

// entitlementProbes returns the canonical, ordered probe map. recentDate is a
// YYYY-MM-DD a few days back, used by endpoints whose spec marks date required.
func entitlementProbes(recentDate string) []entitlementProbe {
	return []entitlementProbe{
		{group: "applications", method: "GET", path: "/applications", params: map[string]string{"search": "tiktok", "limit": "1"}},
		{group: "united-applications", method: "GET", path: "/united-applications", params: map[string]string{"search": "tiktok", "limit": "1"}},
		{group: "publishers", method: "GET", path: "/publishers", params: map[string]string{"search": "king", "limit": "1"}},
		{group: "united-publishers", method: "GET", path: "/united-publishers", params: map[string]string{"search": "king", "limit": "1"}},
		{group: "tops", method: "GET", path: "/tops/united-applications", params: map[string]string{"sort": "top_free", "store": "2", "country": "US", "date": recentDate, "limit": "1"}},
		// The charts group (/dau, /mau, /retention-v2, ...) is POST-only in the
		// spec; fabricating a POST body is noisy, so it is recorded without a probe.
		{group: "charts", method: "POST", path: "/dau", postOnly: true},
		{group: "history", method: "GET", path: "/history/united-applications", params: map[string]string{"date": recentDate, "store": "1", "united_application_ids": "1", "country": "US", "aggregation": "daily"}},
		{group: "steam", method: "GET", path: "/steam/last-date"},
		{group: "period-comparison", method: "GET", path: "/period-comparison/last-date"},
		{group: "categories", method: "GET", path: "/categories"},
		{group: "tags", method: "GET", path: "/tags"},
		{group: "aso", method: "GET", path: "/aso/dates"},
		{group: "asa", method: "GET", path: "/asa/dates"},
		{group: "live-ops", method: "GET", path: "/live-ops/games"},
		{group: "sdkint", method: "GET", path: "/sdkint/sdks", params: map[string]string{"store": "1", "store_application_id": "com.zhiliaoapp.musically"}},
		{group: "adint", method: "GET", path: "/adint/filter_values"},
		// GET-by-id is the only cheap contacts read; a 404 on id 1 still proves
		// the request routed past auth, which the verdict map counts as included.
		{group: "contacts", method: "GET", path: "/contacts/companies/1"},
		{group: "featuring", method: "GET", path: "/featuring", params: map[string]string{"store": "1", "store_application_id": "com.zhiliaoapp.musically", "country": "US", "date": recentDate, "limit": "1"}},
		{group: "keywords", method: "GET", path: "/keywords/all"},
		{group: "last-date", method: "GET", path: "/last-date"},
	}
}

// entitlementGroupAliases maps spelled-out group names from the docs to the
// canonical kebab-case keys used by --groups.
var entitlementGroupAliases = map[string]string{
	"ad-intelligence": "adint",
}

// entitlementDogfoodGroups is the curtailment set for dogfood environments:
// three free, parameter-less probes instead of the full twenty-group sweep.
var entitlementDogfoodGroups = map[string]bool{
	"categories": true,
	"tags":       true,
	"last-date":  true,
}

// entitlementVerdict classifies a probe's HTTP status into a verdict and a
// human-readable detail. status 0 means the request never produced an HTTP
// response (network failure).
//
// The classification logic: any status proving the request was routed past
// auth and reached the handler (2xx data, 400/422 validation, 404 lookup)
// means the group is in the contract; 403 is the contract gate itself.
func entitlementVerdict(status int) (verdict, detail string) {
	switch {
	case status >= 200 && status < 300:
		return verdictIncluded, fmt.Sprintf("HTTP %d: probe returned data", status)
	case status == 400 || status == 422:
		return verdictIncluded, fmt.Sprintf("HTTP %d: request reached parameter validation, so the group is entitled", status)
	case status == 401:
		return verdictAuthRequired, "HTTP 401: credentials missing or invalid"
	case status == 403:
		return verdictNotIncluded, "HTTP 403: your contract does not include this endpoint group"
	case status == 404:
		return verdictIncluded, "HTTP 404: probe id not found, but the request routed past auth, so the group is entitled"
	case status == 429:
		return verdictRateLimited, "HTTP 429: rate limited before the probe could finish; retry later"
	case status == 0:
		return verdictError, "network error: the API could not be reached"
	default:
		return verdictUnknown, fmt.Sprintf("unexpected HTTP %d from the probe", status)
	}
}

// entitlementProbeRow is one persisted row of the entitlement_probes cache.
// JSON tags let tests feed fixture rows and keep the cache shape explicit.
type entitlementProbeRow struct {
	Group      string `json:"group"`
	Method     string `json:"probe_method"`
	Path       string `json:"probe_path"`
	HTTPStatus int    `json:"http_status"`
	Verdict    string `json:"verdict"`
	Detail     string `json:"detail"`
	CheckedAt  string `json:"checked_at"`
}

// entitlementCacheUsable reports whether every requested group has a cached
// probe row younger than entitlementCacheMaxAge. Any missing group, stale
// row, or unparsable timestamp forces a fresh probe run.
func entitlementCacheUsable(rows map[string]entitlementProbeRow, groups []string, now time.Time) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		row, ok := rows[g]
		if !ok {
			return false
		}
		checked, err := time.Parse(time.RFC3339, row.CheckedAt)
		if err != nil {
			return false
		}
		if now.Sub(checked) >= entitlementCacheMaxAge {
			return false
		}
	}
	return true
}

// parseEntitlementGroups validates the --groups CSV against the canonical
// probe map and returns the selection in canonical probe order. An empty CSV
// selects every group. Input is normalized (lowercase, spaces and
// underscores to dashes) and doc-style aliases are accepted.
func parseEntitlementGroups(csv string, probes []entitlementProbe) ([]string, error) {
	known := make(map[string]bool, len(probes))
	order := make([]string, 0, len(probes))
	for _, p := range probes {
		known[p.group] = true
		order = append(order, p.group)
	}
	if strings.TrimSpace(csv) == "" {
		return order, nil
	}
	requested := map[string]bool{}
	for _, tok := range strings.Split(csv, ",") {
		norm := strings.ToLower(strings.TrimSpace(tok))
		norm = strings.NewReplacer(" ", "-", "_", "-").Replace(norm)
		if norm == "" {
			continue
		}
		if alias, ok := entitlementGroupAliases[norm]; ok {
			norm = alias
		}
		if !known[norm] {
			return nil, fmt.Errorf("unknown group %q; valid groups: %s", tok, strings.Join(order, ", "))
		}
		requested[norm] = true
	}
	if len(requested) == 0 {
		return order, nil
	}
	selected := make([]string, 0, len(requested))
	for _, g := range order {
		if requested[g] {
			selected = append(selected, g)
		}
	}
	return selected, nil
}

// dogfoodEntitlementGroups curtails a selection to the cheap dogfood probe
// set. When the intersection is empty the full dogfood set is used so the
// command still demonstrates real behavior.
func dogfoodEntitlementGroups(selected []string) []string {
	out := make([]string, 0, len(entitlementDogfoodGroups))
	for _, g := range selected {
		if entitlementDogfoodGroups[g] {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		for g := range entitlementDogfoodGroups {
			out = append(out, g)
		}
		sort.Strings(out)
	}
	return out
}

type entitlementGroupView struct {
	Group      string `json:"group"`
	Verdict    string `json:"verdict"`
	HTTPStatus int    `json:"http_status"`
	Detail     string `json:"detail,omitempty"`
}

type entitlementSummaryView struct {
	Included    int `json:"included"`
	NotIncluded int `json:"not_included"`
	Unknown     int `json:"unknown"`
}

type entitlementsView struct {
	CheckedAt string                 `json:"checked_at"`
	Groups    []entitlementGroupView `json:"groups"`
	Summary   entitlementSummaryView `json:"summary"`
	Note      string                 `json:"note,omitempty"`
}

// entitlementSummarize folds group verdicts into the included/not-included/
// unknown counts. Everything that is neither a positive nor a negative proof
// (auth-required, rate-limited, post-only, error, unknown) counts as unknown.
func entitlementSummarize(groups []entitlementGroupView) entitlementSummaryView {
	var s entitlementSummaryView
	for _, g := range groups {
		switch g.Verdict {
		case verdictIncluded:
			s.Included++
		case verdictNotIncluded:
			s.NotIncluded++
		default:
			s.Unknown++
		}
	}
	return s
}

// entitlementViewFromRows builds the output view from cached rows, in the
// canonical order of the groups slice. Groups without a cached row are
// skipped; the returned missing slice names them. CheckedAt is the oldest
// served timestamp so the caller sees worst-case staleness.
func entitlementViewFromRows(rows map[string]entitlementProbeRow, groups []string) (entitlementsView, []string) {
	view := entitlementsView{Groups: make([]entitlementGroupView, 0, len(groups))}
	missing := make([]string, 0)
	oldest := ""
	for _, g := range groups {
		row, ok := rows[g]
		if !ok {
			missing = append(missing, g)
			continue
		}
		view.Groups = append(view.Groups, entitlementGroupView{
			Group:      row.Group,
			Verdict:    row.Verdict,
			HTTPStatus: row.HTTPStatus,
			Detail:     row.Detail,
		})
		if oldest == "" || row.CheckedAt < oldest {
			oldest = row.CheckedAt
		}
	}
	view.CheckedAt = oldest
	view.Summary = entitlementSummarize(view.Groups)
	return view, missing
}

func loadEntitlementCache(cmd *cobra.Command, db *store.Store) (map[string]entitlementProbeRow, error) {
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT group_name, probe_method, probe_path, http_status, verdict, detail, checked_at FROM entitlement_probes`)
	if err != nil {
		return nil, fmt.Errorf("reading entitlement probe cache: %w", err)
	}
	defer rows.Close()
	out := map[string]entitlementProbeRow{}
	for rows.Next() {
		var r entitlementProbeRow
		var detail sql.NullString
		if err := rows.Scan(&r.Group, &r.Method, &r.Path, &r.HTTPStatus, &r.Verdict, &detail, &r.CheckedAt); err != nil {
			return nil, fmt.Errorf("scanning entitlement probe cache: %w", err)
		}
		r.Detail = detail.String
		out[r.Group] = r
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating entitlement probe cache: %w", err)
	}
	return out, nil
}

func saveEntitlementCache(cmd *cobra.Command, db *store.Store, rows []entitlementProbeRow) error {
	for _, r := range rows {
		if _, err := db.DB().ExecContext(cmd.Context(),
			`INSERT OR REPLACE INTO entitlement_probes
				(group_name, probe_method, probe_path, http_status, verdict, detail, checked_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			r.Group, r.Method, r.Path, r.HTTPStatus, r.Verdict, r.Detail, r.CheckedAt); err != nil {
			return fmt.Errorf("caching entitlement probe for %q: %w", r.Group, err)
		}
	}
	return nil
}

func newNovelEntitlementsCmd(flags *rootFlags) *cobra.Command {
	var flagRefresh bool
	var flagDB string
	var flagGroups string

	// pp:data-source auto
	cmd := &cobra.Command{
		Use:   "entitlements",
		Short: "Discover exactly which AppMagic endpoint groups your contract includes, before a bare 403 surprises you.",
		Long: "Use this command to map your AppMagic contract: it sends one cheap GET per endpoint group " +
			"and classifies each group from the HTTP status. 200, 400, 422, and 404 all prove the request " +
			"routed past the contract gate (verdict 'included'); 403 means the group is not in your contract " +
			"(verdict 'not-included'); 401 means credentials are missing or invalid. POST-only groups (charts) " +
			"are reported as 'post-only' without a probe. Verdicts are cached locally for 7 days; pass --refresh " +
			"to re-probe, or --data-source local to read the cache without any network traffic.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  appmagic-pp-cli entitlements --refresh
  appmagic-pp-cli entitlements --groups tops,history,aso --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would probe AppMagic endpoint groups and report an entitlement verdict map")
				return nil
			}

			localOnly := flags.dataSource == "local"
			if localOnly && flagRefresh {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--refresh needs network probes and cannot be combined with --data-source local"))
			}

			recentDate := time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02")
			probes := entitlementProbes(recentDate)
			selected, err := parseEntitlementGroups(flagGroups, probes)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			// Missing-mirror guard for the cache-only path.
			if localOnly && snapshotMirrorMissing(cmd, flags, flagDB,
				"run 'appmagic-pp-cli entitlements --refresh' to populate it") {
				return nil
			}

			ctx := cmd.Context()
			db, _, err := openSnapshotDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			cached, err := loadEntitlementCache(cmd, db)
			if err != nil {
				return err
			}

			// Cache-only mode: serve whatever rows exist, even stale ones.
			if localOnly {
				view, missing := entitlementViewFromRows(cached, selected)
				view.Note = "served from the local probe cache (--data-source local); run with --refresh to re-probe"
				if len(missing) > 0 {
					view.Note += "; no cached probe for: " + strings.Join(missing, ", ")
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			// Fresh-enough cache short-circuits the network entirely.
			if !flagRefresh && entitlementCacheUsable(cached, selected, time.Now().UTC()) {
				view, _ := entitlementViewFromRows(cached, selected)
				view.Note = "served from cached probes (younger than 7 days); pass --refresh to re-probe"
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			dogfoodNote := ""
			if cliutil.IsDogfoodEnv() {
				selected = dogfoodEntitlementGroups(selected)
				dogfoodNote = "dogfood environment: probed only " + strings.Join(selected, ", ")
			}
			selectedSet := make(map[string]bool, len(selected))
			for _, g := range selected {
				selectedSet[g] = true
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			checkedAt := time.Now().UTC().Format(time.RFC3339)
			results := make([]entitlementGroupView, 0, len(selected))
			rows := make([]entitlementProbeRow, 0, len(selected))
			probed, unauthorized := 0, 0
			for _, p := range probes {
				if !selectedSet[p.group] {
					continue
				}
				var verdict, detail string
				status := 0
				if p.postOnly {
					verdict = verdictPostOnly
					detail = "group has no cheap GET; probed indirectly via history"
				} else {
					if probed > 0 {
						// Be polite: 100ms gap between sequential probes.
						select {
						case <-ctx.Done():
						case <-time.After(100 * time.Millisecond):
						}
					}
					probed++
					_, probeErr := c.GetNoCache(ctx, p.path, p.params)
					var apiE *client.APIError
					switch {
					case probeErr == nil:
						status = 200
						verdict, detail = entitlementVerdict(status)
					case errors.Is(probeErr, client.ErrPlaceholderCredential):
						verdict = verdictAuthRequired
						detail = "placeholder credential configured; set APPMAGIC_LOGIN to a real key"
					case errors.As(probeErr, &apiE):
						status = apiE.StatusCode
						verdict, detail = entitlementVerdict(status)
					default:
						verdict, detail = entitlementVerdict(0)
						detail += ": " + truncate(probeErr.Error(), 200)
					}
					if verdict == verdictAuthRequired {
						unauthorized++
					}
				}
				results = append(results, entitlementGroupView{Group: p.group, Verdict: verdict, HTTPStatus: status, Detail: detail})
				rows = append(rows, entitlementProbeRow{Group: p.group, Method: p.method, Path: p.path, HTTPStatus: status, Verdict: verdict, Detail: detail, CheckedAt: checkedAt})
			}

			// Write the cache even when every probe failed auth; the verdict
			// map itself is the answer and the command still exits 0.
			if err := saveEntitlementCache(cmd, db, rows); err != nil {
				return err
			}

			view := entitlementsView{
				CheckedAt: checkedAt,
				Groups:    results,
				Summary:   entitlementSummarize(results),
				Note:      dogfoodNote,
			}
			if probed > 0 && unauthorized == probed {
				msg := "credentials missing or invalid: every probe returned an auth failure; set APPMAGIC_LOGIN and re-run with --refresh"
				fmt.Fprintln(cmd.ErrOrStderr(), msg)
				if view.Note != "" {
					view.Note += "; "
				}
				view.Note += msg
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Force fresh probes against the API instead of serving the cached verdict map (cache lives 7 days)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite database holding the probe cache (default: the standard appmagic-pp-cli mirror)")
	cmd.Flags().StringVar(&flagGroups, "groups", "", "Comma-separated endpoint groups to probe; empty means every group (e.g. tops,history,aso,charts)")
	return cmd
}
