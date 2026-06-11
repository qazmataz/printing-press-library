// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/store"
	"github.com/spf13/cobra"
)

const snapshotDateLayout = "2006-01-02"

// chartDiffSortMap maps the user-facing --sort values to the official API's
// sort parameter values on GET /tops/united-applications.
var chartDiffSortMap = map[string]string{
	"free":      "top_free",
	"grossing":  "top_grossing",
	"featuring": "top_featuring",
}

// chartTopRow is one parsed row of GET /tops/united-applications:
// {place:int, united_application_id:int, value:int}. Raw preserves the
// original JSON element for the snapshot's raw column.
type chartTopRow struct {
	Place               int
	UnitedApplicationID string
	Value               int64
	Raw                 json.RawMessage
}

// chartSnapshotRow is one stored snapshot row used by the pure diff logic.
type chartSnapshotRow struct {
	Rank int
	ID   string
	Name string
}

type chartDiffEntry struct {
	Rank                int    `json:"rank"`
	UnitedApplicationID string `json:"united_application_id"`
	Name                string `json:"name,omitempty"`
}

type chartDiffMove struct {
	UnitedApplicationID string `json:"united_application_id"`
	Name                string `json:"name,omitempty"`
	FromRank            int    `json:"from_rank"`
	ToRank              int    `json:"to_rank"`
	// Delta = from_rank - to_rank; positive means the app climbed the chart.
	Delta int `json:"delta"`
}

type chartDiffView struct {
	Sort    string           `json:"sort"`
	Store   int              `json:"store"`
	Country string           `json:"country"`
	From    string           `json:"from,omitempty"`
	To      string           `json:"to,omitempty"`
	Entered []chartDiffEntry `json:"entered"`
	Dropped []chartDiffEntry `json:"dropped"`
	Moved   []chartDiffMove  `json:"moved"`
	Note    string           `json:"note,omitempty"`
}

// officialChartDate returns the latest date the official API is expected to
// have top-chart data for: today minus two days (data lags the calendar).
func officialChartDate(now time.Time) string {
	return now.AddDate(0, 0, -2).Format(snapshotDateLayout)
}

// parseChartTopRows defensively parses the bare-array response of
// GET /tops/united-applications. Tolerates a {data:[...]} envelope and
// missing fields so spec drift degrades to partial rows, not failures.
func parseChartTopRows(data json.RawMessage) []chartTopRow {
	arr := decodeObjectArray(data, "data", "items", "results")
	out := make([]chartTopRow, 0, len(arr))
	for _, m := range arr {
		row := chartTopRow{}
		if v, ok := m["place"].(float64); ok {
			row.Place = int(v)
		}
		switch v := m["united_application_id"].(type) {
		case float64:
			row.UnitedApplicationID = strconv.FormatInt(int64(v), 10)
		case string:
			row.UnitedApplicationID = v
		}
		if v, ok := m["value"].(float64); ok {
			row.Value = int64(v)
		}
		if row.UnitedApplicationID == "" {
			continue
		}
		if row.Place == 0 {
			row.Place = len(out) + 1
		}
		if raw, err := json.Marshal(m); err == nil {
			row.Raw = raw
		}
		out = append(out, row)
	}
	return out
}

// diffChartSnapshots computes entered (in curr, not prev), dropped (in prev,
// not curr), and moved (in both with a nonzero rank delta) between two
// snapshots. Delta = from_rank - to_rank, so positive means the app climbed.
// Movers are sorted by |delta| descending (ties: better current rank first)
// and capped at maxMovers when maxMovers > 0. Duplicate IDs within one
// snapshot keep their first (best-ranked) occurrence.
func diffChartSnapshots(prev, curr []chartSnapshotRow, maxMovers int) (entered, dropped []chartDiffEntry, moved []chartDiffMove) {
	entered = make([]chartDiffEntry, 0)
	dropped = make([]chartDiffEntry, 0)
	moved = make([]chartDiffMove, 0)
	prevByID := make(map[string]chartSnapshotRow, len(prev))
	for _, r := range prev {
		if _, ok := prevByID[r.ID]; !ok {
			prevByID[r.ID] = r
		}
	}
	currByID := make(map[string]chartSnapshotRow, len(curr))
	for _, r := range curr {
		if _, ok := currByID[r.ID]; !ok {
			currByID[r.ID] = r
		}
	}
	seen := make(map[string]bool, len(curr))
	for _, r := range curr {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		p, ok := prevByID[r.ID]
		if !ok {
			entered = append(entered, chartDiffEntry{Rank: r.Rank, UnitedApplicationID: r.ID, Name: r.Name})
			continue
		}
		delta := p.Rank - r.Rank
		if delta == 0 {
			continue
		}
		name := r.Name
		if name == "" {
			name = p.Name
		}
		moved = append(moved, chartDiffMove{
			UnitedApplicationID: r.ID,
			Name:                name,
			FromRank:            p.Rank,
			ToRank:              r.Rank,
			Delta:               delta,
		})
	}
	seenPrev := make(map[string]bool, len(prev))
	for _, r := range prev {
		if seenPrev[r.ID] {
			continue
		}
		seenPrev[r.ID] = true
		if _, ok := currByID[r.ID]; !ok {
			dropped = append(dropped, chartDiffEntry{Rank: r.Rank, UnitedApplicationID: r.ID, Name: r.Name})
		}
	}
	sort.Slice(entered, func(i, j int) bool { return entered[i].Rank < entered[j].Rank })
	sort.Slice(dropped, func(i, j int) bool { return dropped[i].Rank < dropped[j].Rank })
	sort.SliceStable(moved, func(i, j int) bool {
		ai, aj := chartAbsInt(moved[i].Delta), chartAbsInt(moved[j].Delta)
		if ai != aj {
			return ai > aj
		}
		return moved[i].ToRank < moved[j].ToRank
	})
	if maxMovers > 0 && len(moved) > maxMovers {
		moved = moved[:maxMovers]
	}
	return entered, dropped, moved
}

func chartAbsInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// chartDiffInsufficientNote explains the <2-snapshots case and names the
// exact command to run again tomorrow to make the diff possible.
func chartDiffInsufficientNote(sortFlag string, storeID int, country string, have int) string {
	return fmt.Sprintf(
		"only %d snapshot(s) captured for sort=%s store=%d country=%s; two distinct snapshot dates are needed to diff. Run 'appmagic-pp-cli chart-diff --sort %s --store %d --country %s' again tomorrow to capture the next snapshot.",
		have, sortFlag, storeID, country, sortFlag, storeID, country)
}

// replaceChartSnapshot replaces the stored snapshot for one
// (sort, store, country, date) tuple. Previously enriched app names for the
// same chart filter are carried forward so re-captures do not lose them.
func replaceChartSnapshot(ctx context.Context, db *store.Store, sortAPI string, storeID int, country, snapshotDate string, rows []chartTopRow) error {
	names := map[string][2]string{}
	nameRows, err := db.DB().QueryContext(ctx, `
		SELECT united_application_id, app_name, publisher_name FROM chart_snapshots
		WHERE sort_type = ? AND store = ? AND country = ? AND app_name IS NOT NULL`,
		sortAPI, storeID, country)
	if err == nil {
		defer nameRows.Close()
		for nameRows.Next() {
			var id string
			var appName, pubName sql.NullString
			if err := nameRows.Scan(&id, &appName, &pubName); err != nil {
				return fmt.Errorf("scanning enriched chart name row: %w", err)
			}
			names[id] = [2]string{appName.String, pubName.String}
		}
		if err := nameRows.Err(); err != nil {
			return fmt.Errorf("reading enriched chart names: %w", err)
		}
	}

	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting snapshot transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM chart_snapshots
		WHERE sort_type = ? AND store = ? AND country = ? AND snapshot_date = ?`,
		sortAPI, storeID, country, snapshotDate); err != nil {
		return fmt.Errorf("clearing prior snapshot rows: %w", err)
	}
	capturedAt := time.Now().UTC().Format(time.RFC3339)
	for _, r := range rows {
		var appName, pubName any
		if pair, ok := names[r.UnitedApplicationID]; ok {
			if pair[0] != "" {
				appName = pair[0]
			}
			if pair[1] != "" {
				pubName = pair[1]
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO chart_snapshots
				(sort_type, store, country, snapshot_date, rank, united_application_id, app_name, publisher_name, value, raw, captured_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sortAPI, storeID, country, snapshotDate, r.Place, r.UnitedApplicationID, appName, pubName, r.Value, string(r.Raw), capturedAt); err != nil {
			return fmt.Errorf("inserting snapshot row: %w", err)
		}
	}
	return tx.Commit()
}

// distinctSnapshotDates returns up to limit distinct snapshot dates for one
// chart filter, newest first.
func distinctSnapshotDates(ctx context.Context, db *store.Store, sortAPI string, storeID int, country string, limit int) ([]string, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT DISTINCT snapshot_date FROM chart_snapshots
		WHERE sort_type = ? AND store = ? AND country = ?
		ORDER BY snapshot_date DESC LIMIT ?`,
		sortAPI, storeID, country, limit)
	if err != nil {
		return nil, fmt.Errorf("listing snapshot dates: %w", err)
	}
	defer rows.Close()
	dates := make([]string, 0, limit)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dates, nil
}

// loadChartSnapshot loads one stored snapshot ordered by rank.
func loadChartSnapshot(ctx context.Context, db *store.Store, sortAPI string, storeID int, country, snapshotDate string) ([]chartSnapshotRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT rank, united_application_id, app_name FROM chart_snapshots
		WHERE sort_type = ? AND store = ? AND country = ? AND snapshot_date = ?
		ORDER BY rank ASC`,
		sortAPI, storeID, country, snapshotDate)
	if err != nil {
		return nil, fmt.Errorf("loading snapshot %s: %w", snapshotDate, err)
	}
	defer rows.Close()
	out := make([]chartSnapshotRow, 0)
	for rows.Next() {
		var r chartSnapshotRow
		var name sql.NullString
		if err := rows.Scan(&r.Rank, &r.ID, &name); err != nil {
			return nil, err
		}
		r.Name = name.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// updateChartSnapshotName persists an enriched app identity across all
// snapshot dates for the chart filter so future diffs render names offline.
func updateChartSnapshotName(ctx context.Context, db *store.Store, sortAPI string, storeID int, country, unitedID, name, publisher string) {
	_, _ = db.DB().ExecContext(ctx, `
		UPDATE chart_snapshots SET app_name = ?, publisher_name = ?
		WHERE sort_type = ? AND store = ? AND country = ? AND united_application_id = ?`,
		name, publisher, sortAPI, storeID, country, unitedID)
}

func newNovelChartDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSort      string
		flagStore     int
		flagCountry   string
		flagDate      string
		flagFrom      string
		flagTo        string
		flagTop       int
		flagDB        string
		flagNoCapture bool
	)

	// pp:data-source auto
	cmd := &cobra.Command{
		Use:   "chart-diff",
		Short: "See who entered, dropped, or moved in any top chart between two synced snapshots.",
		Long:  "Use this command to see rank movement, new entrants, and dropouts between two synced top-chart snapshots for any chart sort. Do NOT use it to track newly soft-launched titles across test markets; use 'soft-launch-radar' instead. Do NOT use it for market-size aggregates by tag; use 'tag-rollup' instead.",
		Example: strings.Trim(`
  appmagic-pp-cli chart-diff --sort grossing --store 1 --country US
  appmagic-pp-cli chart-diff --sort free --country US --from 2026-06-07 --to 2026-06-08 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would capture a top-chart snapshot and diff the two most recent snapshots")
				return nil
			}
			sortAPI, ok := chartDiffSortMap[flagSort]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sort is required and must be one of free, grossing, featuring (got %q)", flagSort))
			}
			if flagStore < 1 || flagStore > 5 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--store must be 1-5 (1 Google Play, 2 iPhone, 3 iPad, 4 iPhone+iPad, 5 all stores); got %d", flagStore))
			}
			if flagTop < 1 || flagTop > 1000 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--top must be between 1 and 1000; got %d", flagTop))
			}
			if (flagFrom == "") != (flagTo == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to must be provided together"))
			}
			for _, pair := range [][2]string{{"--date", flagDate}, {"--from", flagFrom}, {"--to", flagTo}} {
				if pair[1] == "" {
					continue
				}
				if _, err := time.Parse(snapshotDateLayout, pair[1]); err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("%s must be a YYYY-MM-DD date; got %q", pair[0], pair[1]))
				}
			}
			if flagFrom != "" && flagTo != "" && flagFrom > flagTo {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from %s is after --to %s; swap the values", flagFrom, flagTo))
			}

			ctx := cmd.Context()
			capture := !flagNoCapture && flags.dataSource != "local"
			if !capture && snapshotMirrorMissing(cmd, flags, flagDB,
				"re-run without --no-capture (and without --data-source local) to capture the first snapshot") {
				return nil
			}
			db, _, err := openSnapshotDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			captureDate := flagDate
			if captureDate == "" {
				captureDate = officialChartDate(time.Now().UTC())
			}
			var c *client.Client
			if capture {
				c, err = flags.newClient()
				if err != nil {
					return err
				}
				data, err := c.Get(ctx, "/tops/united-applications", map[string]string{
					"sort":        sortAPI,
					"store":       strconv.Itoa(flagStore),
					"country":     flagCountry,
					"date":        captureDate,
					"aggregation": "daily",
					"limit":       strconv.Itoa(flagTop),
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				rows := parseChartTopRows(data)
				if err := replaceChartSnapshot(ctx, db, sortAPI, flagStore, flagCountry, captureDate, rows); err != nil {
					return err
				}
			}

			fromDate, toDate := flagFrom, flagTo
			if fromDate == "" {
				dates, err := distinctSnapshotDates(ctx, db, sortAPI, flagStore, flagCountry, 2)
				if err != nil {
					return err
				}
				if len(dates) < 2 {
					view := chartDiffView{
						Sort: flagSort, Store: flagStore, Country: flagCountry,
						Entered: make([]chartDiffEntry, 0),
						Dropped: make([]chartDiffEntry, 0),
						Moved:   make([]chartDiffMove, 0),
						Note:    chartDiffInsufficientNote(flagSort, flagStore, flagCountry, len(dates)),
					}
					if len(dates) == 1 {
						view.From, view.To = dates[0], dates[0]
					}
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				toDate, fromDate = dates[0], dates[1]
			}

			prev, err := loadChartSnapshot(ctx, db, sortAPI, flagStore, flagCountry, fromDate)
			if err != nil {
				return err
			}
			curr, err := loadChartSnapshot(ctx, db, sortAPI, flagStore, flagCountry, toDate)
			if err != nil {
				return err
			}
			if len(prev) == 0 || len(curr) == 0 {
				missing := fromDate
				if len(curr) == 0 {
					missing = toDate
				}
				view := chartDiffView{
					Sort: flagSort, Store: flagStore, Country: flagCountry,
					From:    fromDate,
					To:      toDate,
					Entered: make([]chartDiffEntry, 0),
					Dropped: make([]chartDiffEntry, 0),
					Moved:   make([]chartDiffMove, 0),
					Note: fmt.Sprintf("no snapshot stored for %s (sort=%s store=%d country=%s); %s",
						missing, flagSort, flagStore, flagCountry,
						chartDiffInsufficientNote(flagSort, flagStore, flagCountry, 1)),
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			entered, dropped, moved := diffChartSnapshots(prev, curr, flagTop)

			// Enrich names for NEW and DROPPED apps only (cap 50 ids), and only
			// when network access is allowed for this invocation. Failures here
			// degrade to a warning; the diff itself is already computed.
			if capture {
				needs := make([]int64, 0, 50)
				seen := map[string]bool{}
				for _, e := range append(append([]chartDiffEntry{}, entered...), dropped...) {
					if e.Name != "" || seen[e.UnitedApplicationID] {
						continue
					}
					id, err := strconv.ParseInt(e.UnitedApplicationID, 10, 64)
					if err != nil {
						continue
					}
					seen[e.UnitedApplicationID] = true
					needs = append(needs, id)
					if len(needs) >= 50 {
						break
					}
				}
				if len(needs) > 0 {
					apps, err := unitedAppsByIDs(ctx, c, needs)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: name enrichment failed: %v\n", err)
					} else {
						byID := map[string]*unitedApp{}
						for _, a := range apps {
							byID[strconv.FormatInt(a.ID, 10)] = a
						}
						fill := func(list []chartDiffEntry) {
							for i := range list {
								if a, ok := byID[list[i].UnitedApplicationID]; ok && list[i].Name == "" {
									list[i].Name = a.Name
									updateChartSnapshotName(ctx, db, sortAPI, flagStore, flagCountry, list[i].UnitedApplicationID, a.Name, a.PublisherName)
								}
							}
						}
						fill(entered)
						fill(dropped)
					}
				}
			}

			view := chartDiffView{
				Sort:    flagSort,
				Store:   flagStore,
				Country: flagCountry,
				From:    fromDate,
				To:      toDate,
				Entered: entered,
				Dropped: dropped,
				Moved:   moved,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSort, "sort", "", "Chart sort to diff: free, grossing, or featuring (maps to AppMagic top_free/top_grossing/top_featuring)")
	cmd.Flags().IntVar(&flagStore, "store", 5, "Store id: 1 Google Play, 2 iPhone, 3 iPad, 4 iPhone+iPad, 5 all stores combined")
	cmd.Flags().StringVar(&flagCountry, "country", "WW", "Country code for the chart, e.g. US, GB, or WW for worldwide")
	cmd.Flags().StringVar(&flagDate, "date", "", "Capture date YYYY-MM-DD for today's snapshot; defaults to the latest official data date (today minus 2 days)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Explicit earlier snapshot date YYYY-MM-DD to diff from (requires --to)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Explicit later snapshot date YYYY-MM-DD to diff to (requires --from)")
	cmd.Flags().IntVar(&flagTop, "top", 100, "Chart depth to capture (limit param, 1-1000); also caps the reported movers list")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite snapshot store path (default: ~/.local/share/appmagic-pp-cli/data.db)")
	cmd.Flags().BoolVar(&flagNoCapture, "no-capture", false, "Skip fetching a fresh snapshot; diff only snapshots already stored locally")
	return cmd
}
