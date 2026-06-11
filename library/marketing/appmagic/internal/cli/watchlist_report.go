// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"

	"github.com/spf13/cobra"
)

// wlAllowedMetrics is the closed set accepted by --metrics.
var wlAllowedMetrics = map[string]bool{"downloads": true, "revenue": true, "retention": true}

// wlParseMetrics validates the --metrics CSV against the allowed set and
// returns the requested metrics as a lookup map. Empty entries are skipped;
// an unknown metric is a usage error naming the offending token.
func wlParseMetrics(csv string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, m := range strings.Split(csv, ",") {
		m = strings.ToLower(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if !wlAllowedMetrics[m] {
			return nil, fmt.Errorf("unknown metric %q: allowed values are downloads, revenue, retention", m)
		}
		out[m] = true
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--metrics must name at least one of downloads, revenue, retention")
	}
	return out, nil
}

// wlWindowDays converts a loose duration into a whole number of daily report
// dates, rounding partial days up and flooring at one day.
func wlWindowDays(since time.Duration) int {
	days := int((since + 24*time.Hour - 1) / (24 * time.Hour))
	if days < 1 {
		days = 1
	}
	return days
}

// wlWindowDates returns the daily YYYY-MM-DD dates of the report window,
// oldest first, ending at end inclusive. GET /history/united-applications
// takes a single date per request, so the window is one fetch per date.
func wlWindowDates(end time.Time, days int) []string {
	if days < 1 {
		days = 1
	}
	dates := make([]string, 0, days)
	for i := days - 1; i >= 0; i-- {
		dates = append(dates, end.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return dates
}

// wlHistoryRecord is the subset of a united_record row the report aggregates.
// Downloads and revenue are nullable in the spec, so both are pointers.
type wlHistoryRecord struct {
	UnitedApplicationID int64  `json:"united_application_id"`
	Date                string `json:"date"`
	Downloads           *int64 `json:"downloads"`
	Revenue             *int64 `json:"revenue"`
}

// wlParseHistoryRecords defensively decodes a /history/united-applications
// response (bare array per spec; {data:[...]} tolerated for drift). Any other
// shape is an error so the caller can record the date as a fetch failure.
func wlParseHistoryRecords(raw json.RawMessage) ([]wlHistoryRecord, error) {
	var rows []wlHistoryRecord
	if err := json.Unmarshal(raw, &rows); err == nil {
		return rows, nil
	}
	var env struct {
		Data []wlHistoryRecord `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Data != nil {
		return env.Data, nil
	}
	return nil, fmt.Errorf("unrecognized /history/united-applications response shape (expected a bare array or a {data:[...]} envelope)")
}

// wlMetricSums accumulates window totals for one united application.
type wlMetricSums struct {
	Downloads int64
	Revenue   int64
}

// wlAggregateHistory sums downloads and revenue per united application over
// the fetched history rows. Only ids present in want are aggregated; NULL
// metric values count as zero rather than poisoning the sum.
func wlAggregateHistory(rows []wlHistoryRecord, want map[int64]bool) map[int64]*wlMetricSums {
	out := map[int64]*wlMetricSums{}
	for _, r := range rows {
		if !want[r.UnitedApplicationID] {
			continue
		}
		agg := out[r.UnitedApplicationID]
		if agg == nil {
			agg = &wlMetricSums{}
			out[r.UnitedApplicationID] = agg
		}
		if r.Downloads != nil {
			agg.Downloads += *r.Downloads
		}
		if r.Revenue != nil {
			agg.Revenue += *r.Revenue
		}
	}
	return out
}

// wlReportApp is one side-by-side row of the watchlist report.
type wlReportApp struct {
	Name                string   `json:"name"`
	UnitedApplicationID string   `json:"united_application_id"`
	Downloads           *int64   `json:"downloads,omitempty"`
	Revenue             *int64   `json:"revenue,omitempty"`
	RetentionD1         *float64 `json:"retention_d1,omitempty"`
	RetentionD7         *float64 `json:"retention_d7,omitempty"`
	RetentionD30        *float64 `json:"retention_d30,omitempty"`
}

// wlReportView is the report envelope (rule-15 fan-out shape).
type wlReportView struct {
	Window        string        `json:"window"`
	Country       string        `json:"country"`
	Apps          []wlReportApp `json:"apps"`
	FetchFailures []string      `json:"fetch_failures,omitempty"`
	Note          string        `json:"note,omitempty"`
}

// wlFetchResult carries one parallel fetch outcome; id names the unit
// (a window date for history, an app name for retention).
type wlFetchResult struct {
	id   string
	rows []wlHistoryRecord
	days rbRetentionDays
	err  error
}

func newNovelWatchlistReportCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string
	var flagMetrics string
	var flagSince string
	var flagDB string

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Pull downloads, revenue, and retention for your saved competitor set in one side-by-side table.",
		Long: "Use this command to pull a side-by-side weekly metrics table for your saved competitor set. " +
			"Do NOT use it to benchmark one app against its genre cohort median; use 'retention-benchmark' instead. " +
			"Downloads and revenue are summed across all stores (store 5) from one /history/united-applications " +
			"fetch per day in the window; retention (D1/D7/D30) is pulled per app from /retention-v2 using each " +
			"app's first store application id, with the store inferred from the id shape.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  appmagic-pp-cli watchlist report --country US
  appmagic-pp-cli watchlist report --metrics downloads,revenue,retention --since 7d --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would pull a side-by-side metrics table for every app on the local watchlist")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			metrics, err := wlParseMetrics(flagMetrics)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			since, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			if since > 366*24*time.Hour {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since capped at 366d to bound the per-day fetch fan-out"))
			}

			ctx := cmd.Context()
			end := time.Now().UTC().AddDate(0, 0, -2) // latest date with official data
			days := wlWindowDays(since)
			dates := wlWindowDates(end, days)
			view := wlReportView{
				Window:        fmt.Sprintf("%s..%s", dates[0], dates[len(dates)-1]),
				Country:       flagCountry,
				Apps:          make([]wlReportApp, 0),
				FetchFailures: make([]string, 0),
			}

			db, _, err := openSnapshotDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()
			saved, err := watchlistReadRows(ctx, db)
			if err != nil {
				return err
			}
			if len(saved) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: the watchlist is empty; run 'appmagic-pp-cli watchlist add <app>' to save competitors")
				view.Note = "watchlist is empty; run 'appmagic-pp-cli watchlist add <app>' to save competitors"
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			if cliutil.IsDogfoodEnv() {
				if len(saved) > 3 {
					saved = saved[:3]
				}
				if len(dates) > 3 {
					dates = dates[len(dates)-3:]
				}
				view.Note = "dogfood environment: curtailed to 3 apps and 3 window dates"
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Stable report order: alphabetical by name.
			sort.Slice(saved, func(i, j int) bool { return saved[i].Name < saved[j].Name })
			wantIDs := map[int64]bool{}
			idTokens := make([]string, 0, len(saved))
			for _, r := range saved {
				id, parseErr := strconv.ParseInt(r.UnitedApplicationID, 10, 64)
				if parseErr != nil {
					view.FetchFailures = append(view.FetchFailures, fmt.Sprintf("%s: non-numeric united application id %q", r.Name, r.UnitedApplicationID))
					continue
				}
				wantIDs[id] = true
				idTokens = append(idTokens, r.UnitedApplicationID)
			}

			totalFetches := 0
			failedFetches := 0

			// History fan-out: one fetch per window date per id chunk. The
			// united_application_ids parameter takes at most 100 ids per call
			// (spec maxItems:100), so larger watchlists are chunked and the
			// chunk results are aggregated per date.
			sums := map[int64]*wlMetricSums{}
			if (metrics["downloads"] || metrics["revenue"]) && len(idTokens) > 0 {
				const maxIDsPerCall = 100
				chunks := make([][]string, 0, (len(idTokens)+maxIDsPerCall-1)/maxIDsPerCall)
				for start := 0; start < len(idTokens); start += maxIDsPerCall {
					stop := start + maxIDsPerCall
					if stop > len(idTokens) {
						stop = len(idTokens)
					}
					chunks = append(chunks, idTokens[start:stop])
				}
				results := make([]wlFetchResult, len(dates)*len(chunks))
				var wg sync.WaitGroup
				for di, d := range dates {
					for ci, chunk := range chunks {
						label := d
						if len(chunks) > 1 {
							label = fmt.Sprintf("%s (id chunk %d/%d)", d, ci+1, len(chunks))
						}
						wg.Add(1)
						go func(slot int, d, label string, chunk []string) {
							defer wg.Done()
							raw, fetchErr := c.Get(ctx, "/history/united-applications", map[string]string{
								"date":                   d,
								"store":                  "5",
								"country":                flagCountry,
								"aggregation":            "daily",
								"united_application_ids": strings.Join(chunk, ","),
							})
							if fetchErr != nil {
								results[slot] = wlFetchResult{id: label, err: fetchErr}
								return
							}
							rows, parseErr := wlParseHistoryRecords(raw)
							if parseErr != nil {
								results[slot] = wlFetchResult{id: label, err: parseErr}
								return
							}
							results[slot] = wlFetchResult{id: label, rows: rows}
						}(di*len(chunks)+ci, d, label, chunk)
					}
				}
				wg.Wait()

				allRows := make([]wlHistoryRecord, 0)
				okFetches := 0
				var firstErr error
				for _, res := range results {
					totalFetches++
					if res.err != nil {
						failedFetches++
						if firstErr == nil {
							firstErr = res.err
						}
						view.FetchFailures = append(view.FetchFailures, fmt.Sprintf("history %s: %v", res.id, res.err))
						continue
					}
					okFetches++
					allRows = append(allRows, res.rows...)
				}
				if okFetches == 0 && firstErr != nil {
					return classifyAPIError(firstErr, flags)
				}
				sums = wlAggregateHistory(allRows, wantIDs)
			}

			// Retention fan-out: one /retention-v2 POST per app, using the first
			// store application id with the store inferred from the id shape.
			retention := map[string]rbRetentionDays{}
			if metrics["retention"] {
				results := make([]wlFetchResult, len(saved))
				var wg sync.WaitGroup
				for i, r := range saved {
					if len(r.StoreApplicationIDs) == 0 {
						results[i] = wlFetchResult{id: r.Name, err: fmt.Errorf("no store application id saved; re-run 'watchlist add'")}
						continue
					}
					wg.Add(1)
					go func(i int, name, storeAppID string) {
						defer wg.Done()
						body := rbRetentionRequest{
							StoreApplicationID: storeAppID,
							Store:              storeForStoreAppID(storeAppID),
							Country:            flagCountry,
							DateStart:          dates[0],
							DateEnd:            dates[len(dates)-1],
						}
						raw, _, postErr := c.Post(ctx, "/retention-v2", body)
						if postErr != nil {
							results[i] = wlFetchResult{id: name, err: postErr}
							return
						}
						results[i] = wlFetchResult{id: name, days: rbExtractRetentionDays(raw)}
					}(i, r.Name, r.StoreApplicationIDs[0])
				}
				wg.Wait()
				for i, res := range results {
					totalFetches++
					if res.err != nil {
						failedFetches++
						view.FetchFailures = append(view.FetchFailures, fmt.Sprintf("retention %s: %v", res.id, res.err))
						continue
					}
					retention[saved[i].UnitedApplicationID] = res.days
				}
			}

			if failedFetches > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed; computed over %d successful fetches\n",
					failedFetches, totalFetches, totalFetches-failedFetches)
			}

			for _, r := range saved {
				row := wlReportApp{Name: r.Name, UnitedApplicationID: r.UnitedApplicationID}
				if id, parseErr := strconv.ParseInt(r.UnitedApplicationID, 10, 64); parseErr == nil {
					if metrics["downloads"] || metrics["revenue"] {
						var d, rev int64
						if agg := sums[id]; agg != nil {
							d, rev = agg.Downloads, agg.Revenue
						}
						if metrics["downloads"] {
							row.Downloads = &d
						}
						if metrics["revenue"] {
							row.Revenue = &rev
						}
					}
				}
				if days, ok := retention[r.UnitedApplicationID]; ok {
					row.RetentionD1, row.RetentionD7, row.RetentionD30 = days.D1, days.D7, days.D30
				}
				view.Apps = append(view.Apps, row)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Country or region code scoping downloads, revenue, and retention (for example US, GB, WW)")
	cmd.Flags().StringVar(&flagMetrics, "metrics", "downloads,revenue", "Comma-separated metrics to pull for each saved app: downloads, revenue, retention")
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Lookback window for the report; accepts loose durations like 7d, 2w, 24h")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite watchlist database (default: ~/.local/share/appmagic-pp-cli/data.db)")
	return cmd
}
