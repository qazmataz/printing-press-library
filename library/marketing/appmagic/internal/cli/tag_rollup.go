// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/store"
	"github.com/spf13/cobra"
)

// tagRollupHistoryRow is the subset of a GET /history/united-applications row
// the rollup aggregation needs. Downloads and revenue are float64 so a future
// fractional revenue value degrades gracefully instead of failing the batch.
type tagRollupHistoryRow struct {
	UnitedApplicationID int64   `json:"united_application_id"`
	Date                string  `json:"date"`
	Downloads           float64 `json:"downloads"`
	Revenue             float64 `json:"revenue"`
}

type tagRollupTagView struct {
	Tag         string `json:"tag"`
	TagID       int64  `json:"tag_id"`
	AppsCounted int    `json:"apps_counted"`
	TopNTotal   int64  `json:"top_n_total"`
}

type tagRollupView struct {
	Period        string             `json:"period"`
	Country       string             `json:"country"`
	Store         int                `json:"store"`
	Metric        string             `json:"metric"`
	Aggregation   string             `json:"aggregation"`
	ScannedDates  int                `json:"scanned_dates"`
	Tags          []tagRollupTagView `json:"tags"`
	FetchFailures []string           `json:"fetch_failures,omitempty"`
	Note          string             `json:"note"`
}

// tagRollupDay normalizes a time to a UTC date (00:00).
func tagRollupDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// tagRollupWeekStart returns the Monday of t's ISO week.
func tagRollupWeekStart(t time.Time) time.Time {
	t = tagRollupDay(t)
	offset := (int(t.Weekday()) + 6) % 7 // Monday=0 ... Sunday=6
	return t.AddDate(0, 0, -offset)
}

// tagRollupDates picks an aggregation level for the window [start, end] and
// returns the per-bucket request dates for GET /history/united-applications
// (which takes one date per call; non-daily aggregations are rounded server
// side to the start of the bucket). Short windows stay daily for accuracy;
// longer windows step weekly/monthly to keep the fan-out polite. Week and
// month buckets that straddle the window edges are included whole, so the
// rollup is an estimate at the boundaries.
func tagRollupDates(start, end time.Time) (string, []string) {
	start, end = tagRollupDay(start), tagRollupDay(end)
	if end.Before(start) {
		start, end = end, start
	}
	days := int(end.Sub(start).Hours()/24) + 1
	dates := make([]string, 0, days)
	switch {
	case days <= 31:
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			dates = append(dates, d.Format("2006-01-02"))
		}
		return "daily", dates
	case days <= 182:
		for d := tagRollupWeekStart(start); !d.After(end); d = d.AddDate(0, 0, 7) {
			dates = append(dates, d.Format("2006-01-02"))
		}
		return "weekly", dates
	default:
		for d := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC); !d.After(end); d = d.AddDate(0, 1, 0) {
			dates = append(dates, d.Format("2006-01-02"))
		}
		return "monthly", dates
	}
}

// tagRollupSum aggregates one metric ("downloads" or "revenue") over history
// rows, deduplicating repeated (app, date) rows (overlapping fetches) and
// ignoring apps outside the cohort id set. Returns the summed total and the
// number of distinct cohort apps that contributed at least one row.
func tagRollupSum(rows []tagRollupHistoryRow, cohort map[int64]bool, metric string) (float64, int) {
	seen := make(map[string]bool, len(rows))
	apps := make(map[int64]bool)
	var total float64
	for _, r := range rows {
		if !cohort[r.UnitedApplicationID] {
			continue
		}
		key := strconv.FormatInt(r.UnitedApplicationID, 10) + "|" + r.Date
		if seen[key] {
			continue
		}
		seen[key] = true
		apps[r.UnitedApplicationID] = true
		if metric == "downloads" {
			total += r.Downloads
		} else {
			total += r.Revenue
		}
	}
	return total, len(apps)
}

// parseTagRollupHistoryRows decodes the /history/united-applications response,
// tolerating a {data:[...]} envelope exactly like wlParseHistoryRecords does
// for the same endpoint.
func parseTagRollupHistoryRows(data json.RawMessage) ([]tagRollupHistoryRow, error) {
	var rows []tagRollupHistoryRow
	if err := json.Unmarshal(data, &rows); err == nil {
		return rows, nil
	}
	var envelope struct {
		Data []tagRollupHistoryRow `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Data != nil {
		return envelope.Data, nil
	}
	return nil, fmt.Errorf("unrecognized /history/united-applications response shape (expected a bare array or a {data:[...]} envelope)")
}

// parseTagRollupTopIDs extracts united application ids from the
// GET /tops/united-applications response (bare array or {data:[...]}
// envelope), capped at max entries.
func parseTagRollupTopIDs(data json.RawMessage, max int) []int64 {
	// decodeObjectArray tolerates the {data:[...]} envelope, matching
	// parseChartTopRows on the same /tops/united-applications endpoint.
	rows := decodeObjectArray(data, "data", "items", "results")
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		var id int64
		switch v := r["united_application_id"].(type) {
		case float64:
			id = int64(v)
		case string:
			id, _ = strconv.ParseInt(v, 10, 64)
		}
		if id == 0 {
			continue
		}
		ids = append(ids, id)
		if len(ids) >= max {
			break
		}
	}
	return ids
}

func newNovelTagRollupCmd(flags *rootFlags) *cobra.Command {
	var flagTags []string
	var flagCountry string
	var flagStore int
	var flagMetric string
	var flagPeriod string
	var flagTop int
	var flagDB string

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "tag-rollup",
		Short: "Aggregate market size (summed downloads or revenue) across one or more genre tags.",
		Long: "Use this command for aggregate market sizing (summed downloads or revenue) across one or more tags. " +
			"Do NOT use it to list which apps moved within a chart; use 'chart-diff' instead.",
		Example: strings.Trim(`
  appmagic-pp-cli tag-rollup --tags match-3 --metric revenue --country US
  appmagic-pp-cli tag-rollup --tags match-3,puzzle --period 90d --top 50 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sum top-app downloads or revenue per tag over the period")
				return nil
			}
			if len(flagTags) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--tags is required: one or more tag names or numeric tag ids"))
			}
			if flagMetric != "downloads" && flagMetric != "revenue" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --metric %q: must be downloads or revenue", flagMetric))
			}
			if flagStore < 1 || flagStore > 5 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --store %d: must be 1-5 (5 = all stores)", flagStore))
			}
			if flagTop < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --top %d: must be at least 1", flagTop))
			}
			// The tops/history rollup has no local mirror; an explicit
			// --data-source local cannot be honored.
			if flags.dataSource == "local" {
				return usageErr(unsupportedDataSourceError("live", flags.dataSource))
			}
			period, err := cliutil.ParseDurationLoose(flagPeriod)
			if err != nil || period <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --period %q: use loose durations like 30d, 12w", flagPeriod))
			}

			ctx := cmd.Context()

			top := flagTop
			if cliutil.IsDogfoodEnv() && top > 3 {
				top = 3
			}

			// Resolve tag names against the locally synced taxonomy. Pure
			// numeric ids skip the database entirely.
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("appmagic-pp-cli")
			}
			needDB := false
			for _, t := range flagTags {
				if _, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err != nil {
					needDB = true
					break
				}
			}
			var db *store.Store
			if needDB {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					return notFoundErr(fmt.Errorf("tag names need the local taxonomy and %s does not exist; run: appmagic-pp-cli sync --resources tags (or pass numeric tag ids)", dbPath))
				}
				db, _, err = openSnapshotDB(ctx, dbPath)
				if err != nil {
					return err
				}
				defer db.Close()
				hintIfUnsynced(cmd, db, "tags")
			}
			type resolvedTag struct {
				name string
				id   int64
			}
			resolved := make([]resolvedTag, 0, len(flagTags))
			for _, t := range flagTags {
				id, name, err := resolveTagID(ctx, db, strings.TrimSpace(t))
				if err != nil {
					return err
				}
				resolved = append(resolved, resolvedTag{name: name, id: id})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Latest reliable official data trails today by ~2 days.
			end := tagRollupDay(time.Now().UTC().AddDate(0, 0, -2))
			days := int(period.Hours() / 24)
			if days < 1 {
				days = 1
			}
			start := end.AddDate(0, 0, -(days - 1))
			agg, dates := tagRollupDates(start, end)
			if cliutil.IsDogfoodEnv() && len(dates) > 3 {
				dates = dates[len(dates)-3:]
			}

			sortParam := "top_grossing"
			if flagMetric == "downloads" {
				sortParam = "top_free"
			}

			tagViews := make([]tagRollupTagView, 0, len(resolved))
			failures := make([]string, 0)
			attempted, succeeded := 0, 0
			zeroApps := false
			for _, rt := range resolved {
				attempted++
				topsData, err := c.Get(ctx, "/tops/united-applications", map[string]string{
					"sort":    sortParam,
					"store":   strconv.Itoa(flagStore),
					"country": flagCountry,
					"date":    end.Format("2006-01-02"),
					"limit":   strconv.Itoa(top),
					"tag_id":  strconv.FormatInt(rt.id, 10),
				})
				if err != nil {
					failures = append(failures, fmt.Sprintf("tag %s tops: %v", rt.name, err))
					tagViews = append(tagViews, tagRollupTagView{Tag: rt.name, TagID: rt.id})
					zeroApps = true
					continue
				}
				succeeded++
				ids := parseTagRollupTopIDs(topsData, top)
				if len(ids) == 0 {
					tagViews = append(tagViews, tagRollupTagView{Tag: rt.name, TagID: rt.id})
					zeroApps = true
					continue
				}
				cohort := make(map[int64]bool, len(ids))
				idStrs := make([]string, 0, len(ids))
				for _, id := range ids {
					cohort[id] = true
					idStrs = append(idStrs, strconv.FormatInt(id, 10))
				}
				idCSV := strings.Join(idStrs, ",")

				rows := make([]tagRollupHistoryRow, 0, len(dates)*len(ids))
				for _, date := range dates {
					attempted++
					histData, err := c.Get(ctx, "/history/united-applications", map[string]string{
						"date":                   date,
						"store":                  strconv.Itoa(flagStore),
						"country":                flagCountry,
						"aggregation":            agg,
						"united_application_ids": idCSV,
					})
					if err != nil {
						failures = append(failures, fmt.Sprintf("tag %s history %s: %v", rt.name, date, err))
						continue
					}
					parsed, err := parseTagRollupHistoryRows(histData)
					if err != nil {
						failures = append(failures, fmt.Sprintf("tag %s history %s: %v", rt.name, date, err))
						continue
					}
					succeeded++
					rows = append(rows, parsed...)
				}
				total, counted := tagRollupSum(rows, cohort, flagMetric)
				if counted == 0 {
					zeroApps = true
				}
				tagViews = append(tagViews, tagRollupTagView{
					Tag:         rt.name,
					TagID:       rt.id,
					AppsCounted: counted,
					TopNTotal:   int64(math.Round(total)),
				})
			}

			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed; totals computed over %d successful fetches\n",
					len(failures), attempted, succeeded)
			}

			note := fmt.Sprintf("top_n_total sums only the top %d apps per tag (--top), not the whole market; treat it as a floor estimate", top)
			if zeroApps {
				note += "; at least one tag counted zero apps - check the tag name or widen --top"
			}

			view := tagRollupView{
				Period:        flagPeriod,
				Country:       flagCountry,
				Store:         flagStore,
				Metric:        flagMetric,
				Aggregation:   agg,
				ScannedDates:  len(dates),
				Tags:          tagViews,
				FetchFailures: failures,
				Note:          note,
			}
			if len(view.FetchFailures) == 0 {
				view.FetchFailures = nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagTags, "tags", nil, "Comma-separated tag names or numeric tag ids to roll up (e.g. match-3,puzzle)")
	cmd.Flags().StringVar(&flagCountry, "country", "WW", "Country or region code for the charts and history (e.g. WW, US, GB)")
	cmd.Flags().IntVar(&flagStore, "store", 5, "Store id: 1 Google Play, 2 iPhone, 3 iPad, 4 iPhone+iPad, 5 all stores")
	cmd.Flags().StringVar(&flagMetric, "metric", "revenue", "Metric to sum across each tag cohort: downloads or revenue")
	cmd.Flags().StringVar(&flagPeriod, "period", "30d", "Look-back window for the rollup; loose durations like 30d or 12w")
	cmd.Flags().IntVar(&flagTop, "top", 20, "Apps per tag included in the rollup; this is the scan cap, so totals cover the top N only")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite mirror used for tag-name resolution (default: the synced appmagic-pp-cli database)")
	return cmd
}
