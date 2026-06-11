// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"

	"github.com/spf13/cobra"
)

// rbRetentionRequest is the POST /retention-v2 body (retention_request_v2 in
// the official spec). All five fields are required; store enum is 1|2|3.
type rbRetentionRequest struct {
	StoreApplicationID string `json:"store_application_id"`
	Store              int    `json:"store"`
	Country            string `json:"country"`
	DateStart          string `json:"date_start"`
	DateEnd            string `json:"date_end"`
}

// rbRetentionDays holds the latest non-null D1/D7/D30 values of one app's
// retention curves. A nil day means the curve had no usable points.
type rbRetentionDays struct {
	D1  *float64 `json:"d1"`
	D7  *float64 `json:"d7"`
	D30 *float64 `json:"d30"`
}

// rbExtractRetentionDays defensively pulls D1/D7/D30 from a /retention-v2
// response: {retention_1: [[ts, value], ...], retention_7: ..., ...}. Each
// curve is decoded independently so one malformed series cannot blank the
// others.
func rbExtractRetentionDays(raw json.RawMessage) rbRetentionDays {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(raw, &resp); err != nil {
		return rbRetentionDays{}
	}
	curve := func(key string) *float64 {
		var series [][]*float64
		if err := json.Unmarshal(resp[key], &series); err != nil {
			return nil
		}
		return rbLatestPoint(series)
	}
	return rbRetentionDays{
		D1:  curve("retention_1"),
		D7:  curve("retention_7"),
		D30: curve("retention_30"),
	}
}

// rbLatestPoint walks a retention chart backwards and returns the value of
// the most recent [timestamp, value] pair whose value is non-null.
// Single-element points are tolerated as bare values for spec drift.
func rbLatestPoint(series [][]*float64) *float64 {
	for i := len(series) - 1; i >= 0; i-- {
		p := series[i]
		switch {
		case len(p) >= 2 && p[1] != nil:
			v := *p[1]
			return &v
		case len(p) == 1 && p[0] != nil:
			v := *p[0]
			return &v
		}
	}
	return nil
}

// rbMedian returns the median of vals. Even-count inputs return the AVERAGE
// of the two middle values (documented choice; not the lower-middle). The
// second return is false for an empty input.
func rbMedian(vals []float64) (float64, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	s := append(make([]float64, 0, len(vals)), vals...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2], true
	}
	return (s[n/2-1] + s[n/2]) / 2, true
}

// rbVerdict classifies an app's day value against the cohort median: within
// +/-10% of the median (edges inclusive) is "near median"; otherwise
// "above median" or "below median". Missing data on either side yields
// "unknown". The epsilon keeps values sitting exactly on a band edge from
// flipping verdicts on float64 representation noise.
func rbVerdict(app, median *float64) string {
	if app == nil || median == nil {
		return "unknown"
	}
	const eps = 1e-9
	band := 0.10 * math.Abs(*median)
	switch {
	case *app > *median+band+eps:
		return "above median"
	case *app < *median-band-eps:
		return "below median"
	default:
		return "near median"
	}
}

// rbFirstStoreID returns the first store application id whose shape matches
// the requested store: store 1 (Google Play) wants reversed-domain package
// names; stores 2 and 3 (App Store) want all-digit numeric ids.
func rbFirstStoreID(ids []string, storeNum int) (string, bool) {
	for _, id := range ids {
		if id == "" {
			continue
		}
		numeric := storeForStoreAppID(id) == 2
		if (storeNum == 1 && !numeric) || (storeNum != 1 && numeric) {
			return id, true
		}
	}
	return "", false
}

// rbParseTopRows extracts united application ids from a
// /tops/united-applications response in rank order (bare array of
// {place, united_application_id, value} records per the official spec).
func rbParseTopRows(raw json.RawMessage) []int64 {
	var rows []struct {
		Place               int64 `json:"place"`
		UnitedApplicationID int64 `json:"united_application_id"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil
	}
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		if r.UnitedApplicationID != 0 {
			out = append(out, r.UnitedApplicationID)
		}
	}
	return out
}

// rbBenchmarkView is the retention-benchmark output envelope.
type rbBenchmarkView struct {
	App struct {
		Name                string   `json:"name"`
		UnitedApplicationID string   `json:"united_application_id"`
		StoreApplicationID  string   `json:"store_application_id"`
		D1                  *float64 `json:"d1"`
		D7                  *float64 `json:"d7"`
		D30                 *float64 `json:"d30"`
	} `json:"app"`
	Cohort struct {
		Tag       string   `json:"tag"`
		TagID     int64    `json:"tag_id"`
		Size      int      `json:"size"`
		MedianD1  *float64 `json:"median_d1"`
		MedianD7  *float64 `json:"median_d7"`
		MedianD30 *float64 `json:"median_d30"`
	} `json:"cohort"`
	Verdict struct {
		D1  string `json:"d1"`
		D7  string `json:"d7"`
		D30 string `json:"d30"`
	} `json:"verdict"`
	FetchFailures []string `json:"fetch_failures,omitempty"`
	Note          string   `json:"note,omitempty"`
}

func newNovelRetentionBenchmarkCmd(flags *rootFlags) *cobra.Command {
	var flagTag string
	var flagCountry string
	var flagStore int
	var flagTop int
	var flagDB string

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "retention-benchmark <app-name-or-united-id>",
		Short: "See whether an app's retention curve is normal for its genre by comparing it against the cohort median of its tag.",
		Long: "Use this command to compare one app's retention curve against the median of its tag cohort. " +
			"Do NOT use it for multi-metric comparisons across your saved competitor set; use 'watchlist report' instead. " +
			"The cohort is the tag's current top-grossing chart (size set by --top); each member's D1/D7/D30 is the " +
			"latest non-null point of its /retention-v2 curve over the past 180 days, and the cohort median uses the " +
			"average of the two middle values when the member count is even. The tag is resolved against the locally " +
			"synced taxonomy (run 'appmagic-pp-cli sync --resources tags' first).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  appmagic-pp-cli retention-benchmark "Royal Match" --tag match-3
  appmagic-pp-cli retention-benchmark com.dreamgames.royalmatch --tag match-3 --country US --store 1 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would benchmark the app's D1/D7/D30 retention against its tag cohort median")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an app name or united application id is required"))
			}
			if strings.TrimSpace(flagTag) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--tag is required: it defines the cohort to benchmark against"))
			}
			if flagStore < 1 || flagStore > 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --store %d: /retention-v2 supports 1 (Google Play), 2 (iPhone App Store), 3 (iPad App Store)", flagStore))
			}
			if flagTop < 1 || flagTop > 100 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --top %d: cohort size must be between 1 and 100", flagTop))
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ua, err := resolveUnitedApp(ctx, c, strings.Join(args, " "))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			targetStoreID, ok := rbFirstStoreID(ua.StoreApplicationIDs, flagStore)
			if !ok {
				return notFoundErr(fmt.Errorf("%q has no store-%d application id; try a different --store", ua.Name, flagStore))
			}

			// Tag resolution needs the locally synced taxonomy.
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("appmagic-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return notFoundErr(fmt.Errorf("tag taxonomy is not synced locally; run: appmagic-pp-cli sync --resources tags"))
			}
			db, _, err := openSnapshotDB(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			tagID, tagName, err := resolveTagID(ctx, db, flagTag)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			recentDate := now.AddDate(0, 0, -2).Format("2006-01-02")
			view := rbBenchmarkView{FetchFailures: make([]string, 0)}

			// Cohort: the tag's current top-grossing chart.
			topRaw, err := c.Get(ctx, "/tops/united-applications", map[string]string{
				"sort":    "top_grossing",
				"store":   strconv.Itoa(flagStore),
				"country": flagCountry,
				"date":    recentDate,
				"tag_id":  strconv.FormatInt(tagID, 10),
				"limit":   strconv.Itoa(flagTop),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cohortIDs := rbParseTopRows(topRaw)
			if len(cohortIDs) == 0 {
				return notFoundErr(fmt.Errorf("no top-grossing apps found for tag %q (id %d) in %s on store %d", tagName, tagID, flagCountry, flagStore))
			}
			if cliutil.IsDogfoodEnv() && len(cohortIDs) > 3 {
				cohortIDs = cohortIDs[:3]
				view.Note = "dogfood environment: cohort curtailed to 3 apps"
			}

			cohortApps, err := unitedAppsByIDs(ctx, c, cohortIDs)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Build the retention fetch list: the target first, then each cohort
			// member with a store id matching the requested store kind.
			type rbMember struct {
				name    string
				storeID string
			}
			members := make([]rbMember, 0, len(cohortApps))
			for _, app := range cohortApps {
				label := app.Name
				if label == "" {
					label = fmt.Sprintf("united app %d", app.ID)
				}
				sid, found := rbFirstStoreID(app.StoreApplicationIDs, flagStore)
				if !found {
					view.FetchFailures = append(view.FetchFailures, fmt.Sprintf("cohort %s: no store-%d application id", label, flagStore))
					continue
				}
				members = append(members, rbMember{name: label, storeID: sid})
			}

			dateStart := now.AddDate(0, 0, -180).Format("2006-01-02")
			dateEnd := now.Format("2006-01-02")
			fetchRetention := func(storeAppID string) (json.RawMessage, error) {
				raw, _, postErr := c.Post(ctx, "/retention-v2", rbRetentionRequest{
					StoreApplicationID: storeAppID,
					Store:              flagStore,
					Country:            flagCountry,
					DateStart:          dateStart,
					DateEnd:            dateEnd,
				})
				return raw, postErr
			}

			// Parallel fan-out: index 0 is the target, the rest the cohort.
			results := make([]wlFetchResult, len(members)+1)
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				raw, fetchErr := fetchRetention(targetStoreID)
				if fetchErr != nil {
					results[0] = wlFetchResult{id: ua.Name, err: fetchErr}
					return
				}
				results[0] = wlFetchResult{id: ua.Name, days: rbExtractRetentionDays(raw)}
			}()
			for i, m := range members {
				wg.Add(1)
				go func(i int, m rbMember) {
					defer wg.Done()
					raw, fetchErr := fetchRetention(m.storeID)
					if fetchErr != nil {
						results[i+1] = wlFetchResult{id: m.name, err: fetchErr}
						return
					}
					results[i+1] = wlFetchResult{id: m.name, days: rbExtractRetentionDays(raw)}
				}(i, m)
			}
			wg.Wait()

			if results[0].err != nil {
				return classifyAPIError(fmt.Errorf("fetching retention for %q: %w", ua.Name, results[0].err), flags)
			}
			target := results[0].days

			failed := 0
			var d1s, d7s, d30s []float64
			cohortCounted := 0
			for _, res := range results[1:] {
				if res.err != nil {
					failed++
					view.FetchFailures = append(view.FetchFailures, fmt.Sprintf("cohort %s: %v", res.id, res.err))
					continue
				}
				cohortCounted++
				if res.days.D1 != nil {
					d1s = append(d1s, *res.days.D1)
				}
				if res.days.D7 != nil {
					d7s = append(d7s, *res.days.D7)
				}
				if res.days.D30 != nil {
					d30s = append(d30s, *res.days.D30)
				}
			}
			if failed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d cohort fetches failed; computed over %d cohort apps\n",
					failed, len(members), cohortCounted)
			}

			view.App.Name = ua.Name
			view.App.UnitedApplicationID = strconv.FormatInt(ua.ID, 10)
			view.App.StoreApplicationID = targetStoreID
			view.App.D1, view.App.D7, view.App.D30 = target.D1, target.D7, target.D30
			view.Cohort.Tag = tagName
			view.Cohort.TagID = tagID
			view.Cohort.Size = cohortCounted
			if m, ok := rbMedian(d1s); ok {
				view.Cohort.MedianD1 = &m
			}
			if m, ok := rbMedian(d7s); ok {
				view.Cohort.MedianD7 = &m
			}
			if m, ok := rbMedian(d30s); ok {
				view.Cohort.MedianD30 = &m
			}
			view.Verdict.D1 = rbVerdict(target.D1, view.Cohort.MedianD1)
			view.Verdict.D7 = rbVerdict(target.D7, view.Cohort.MedianD7)
			view.Verdict.D30 = rbVerdict(target.D30, view.Cohort.MedianD30)
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagTag, "tag", "", "Tag (genre) name or numeric tag id defining the cohort, resolved against the locally synced taxonomy")
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Country code scoping the cohort chart and retention curves (for example US, GB)")
	cmd.Flags().IntVar(&flagStore, "store", 1, "Store for cohort and retention: 1 Google Play, 2 iPhone App Store, 3 iPad App Store")
	cmd.Flags().IntVar(&flagTop, "top", 10, "Cohort size: how many of the tag's top-grossing apps to benchmark against")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite database holding the synced tag taxonomy (default: ~/.local/share/appmagic-pp-cli/data.db)")
	return cmd
}
