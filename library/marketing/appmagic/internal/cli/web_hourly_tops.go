// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/source/webapi"
	"github.com/spf13/cobra"
)

// webHourlyTopRow is one chart position from the hourly web-surface charts.
type webHourlyTopRow struct {
	Chart         string `json:"chart"`
	Rank          int    `json:"rank"`
	Diff          int    `json:"diff"`
	App           string `json:"app,omitempty"`
	ApplicationID int64  `json:"application_id,omitempty"`
	Publisher     string `json:"publisher,omitempty"`
}

// webHourlyTopsView is the typed output envelope for `web hourly-tops`.
type webHourlyTopsView struct {
	Date     string            `json:"date"`
	Store    int               `json:"store"`
	Country  string            `json:"country"`
	TopDepth int               `json:"top_depth"`
	Rows     []webHourlyTopRow `json:"rows"`
}

// webSurfaceErr maps web-surface failures to the CLI's typed exit codes.
// It intentionally does NOT route through classifyAPIError: that helper
// appends APPMAGIC_LOGIN hints that are wrong for the web surface, whose
// credential is APPMAGIC_WEB_TOKEN (the webapi.APIError 401 message already
// carries the correct re-copy-the-token hint).
func webSurfaceErr(err error) error {
	if err == nil {
		return nil
	}
	var rateErr *cliutil.RateLimitError
	if errors.As(err, &rateErr) {
		return rateLimitErr(err)
	}
	var apiE *webapi.APIError
	if errors.As(err, &apiE) {
		switch apiE.StatusCode {
		case 401, 403:
			return authErr(err)
		case 404:
			return notFoundErr(err)
		case 429:
			return rateLimitErr(err)
		}
		return apiErr(err)
	}
	return apiErr(err)
}

// webNumField returns the first numeric value found under the given keys.
func webNumField(m map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			return v, true
		}
	}
	return 0, false
}

// webStrField returns the first non-empty string value found under the given keys.
func webStrField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// validateHourlyStore enforces the hourly chart's store coverage: the web
// surface serves stores 1-3 only and has no united (store 5) hourly chart.
func validateHourlyStore(store int) error {
	switch store {
	case 1, 2, 3:
		return nil
	}
	return fmt.Errorf("invalid --store %d: hourly charts cover 1 (Google Play), 2 (iPhone App Store), or 3 (iPad App Store) only; the united store 5 has no hourly chart", store)
}

var webHourlyChartOrder = map[string]int{"free": 0, "grossing": 1, "paid": 2}

// parseHourlyTops extracts per-chart rank rows from the web surface's
// {data:[{top_free:{...}, top_grossing:{...}, top_paid:{...}}], date} shape.
// Field extraction is defensive: missing chart sections are skipped and a
// missing rank falls back to the row's position within its chart.
func parseHourlyTops(raw json.RawMessage) (string, []webHourlyTopRow, error) {
	var envelope struct {
		Date json.RawMessage              `json:"date"`
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", nil, fmt.Errorf("unexpected hourly-tops response shape: %w", err)
	}
	date := ""
	if len(envelope.Date) > 0 && !isRawJSONNull(envelope.Date) {
		if json.Unmarshal(envelope.Date, &date) != nil {
			date = strings.Trim(string(envelope.Date), `"`)
		}
	}
	charts := []struct{ key, label string }{
		{"top_free", "free"},
		{"top_grossing", "grossing"},
		{"top_paid", "paid"},
	}
	rows := make([]webHourlyTopRow, 0, len(envelope.Data)*len(charts))
	perChart := map[string]int{}
	for _, entry := range envelope.Data {
		for _, ch := range charts {
			rawEntry, ok := entry[ch.key]
			if !ok || isRawJSONNull(rawEntry) {
				continue
			}
			var m map[string]any
			if json.Unmarshal(rawEntry, &m) != nil {
				continue
			}
			perChart[ch.label]++
			row := webHourlyTopRow{Chart: ch.label, Rank: perChart[ch.label]}
			if v, ok := webNumField(m, "rank", "place", "position"); ok {
				row.Rank = int(v)
			}
			if v, ok := webNumField(m, "diff", "rank_diff", "change"); ok {
				row.Diff = int(v)
			}
			if app, ok := m["application"].(map[string]any); ok {
				row.App = webStrField(app, "name", "title")
				row.Publisher = webStrField(app, "publisher_name", "publisher")
				if v, ok := webNumField(app, "id", "united_application_id", "application_id"); ok {
					row.ApplicationID = int64(v)
				}
			}
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if webHourlyChartOrder[rows[i].Chart] != webHourlyChartOrder[rows[j].Chart] {
			return webHourlyChartOrder[rows[i].Chart] < webHourlyChartOrder[rows[j].Chart]
		}
		return rows[i].Rank < rows[j].Rank
	})
	return date, rows, nil
}

func newNovelWebHourlyTopsCmd(flags *rootFlags) *cobra.Command {
	var flagStore int
	var flagCountry string
	var flagTopDepth int
	var flagDate string

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "hourly-tops",
		Short: "Intraday top-chart rankings with rank-change diffs from the web app surface.",
		Long: "Use this command to see intraday (hourly) top-chart rankings with rank-change diffs for the free, grossing, and paid charts; the official API only offers daily granularity. " +
			"This command uses the UNOFFICIAL appmagic.rocks web surface and needs APPMAGIC_WEB_TOKEN (Bearer token from a logged-in browser session, localStorage key 'datamagic.token'). The surface can change without notice.",
		Example: strings.Trim(`
  appmagic-pp-cli web hourly-tops --store 2 --country US
  appmagic-pp-cli web hourly-tops --store 1 --country US --top-depth 25 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch hourly top-chart rankings from the appmagic.rocks web surface")
				return nil
			}
			if err := validateHourlyStore(flagStore); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			date := flagDate
			if date == "" {
				date = time.Now().UTC().Format("2006-01-02")
			} else if _, err := time.Parse("2006-01-02", date); err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --date %q: expected YYYY-MM-DD", flagDate))
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would query the appmagic.rocks web surface")
				return nil
			}
			wc, err := webapi.New(flags.timeout)
			if err != nil {
				return authErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			raw, err := wc.Get(ctx, "/top/hourly-apps", map[string]string{
				"topDepth": strconv.Itoa(flagTopDepth),
				"store":    strconv.Itoa(flagStore),
				"country":  flagCountry,
				"date":     date,
			})
			if err != nil {
				return webSurfaceErr(err)
			}
			respDate, rows, err := parseHourlyTops(raw)
			if err != nil {
				return apiErr(err)
			}
			if respDate == "" {
				respDate = date
			}
			view := webHourlyTopsView{
				Date:     respDate,
				Store:    flagStore,
				Country:  flagCountry,
				TopDepth: flagTopDepth,
				Rows:     rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&flagStore, "store", 2, "Store to query: 1 = Google Play, 2 = iPhone App Store, 3 = iPad App Store (united store 5 has no hourly chart)")
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Two-letter country code for the chart, e.g. US, GB, JP")
	cmd.Flags().IntVar(&flagTopDepth, "top-depth", 10, "How many chart positions per chart (free, grossing, paid) to return")
	cmd.Flags().StringVar(&flagDate, "date", "", "Chart date in YYYY-MM-DD format (defaults to today UTC)")
	return cmd
}
