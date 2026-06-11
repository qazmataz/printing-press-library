// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/source/webapi"
	"github.com/spf13/cobra"
)

// webLiveopsTagRow is one tag with its live-ops event count.
type webLiveopsTagRow struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

// liveopsTagsWindow turns the loose --since duration into the dateFrom/dateTo
// pair the web surface expects (YYYY-MM-DD, window ending today UTC).
func liveopsTagsWindow(since string, now time.Time) (string, string, error) {
	d, err := cliutil.ParseDurationLoose(since)
	if err != nil {
		return "", "", fmt.Errorf("invalid --since %q: %w", since, err)
	}
	if d <= 0 {
		return "", "", fmt.Errorf("invalid --since %q: must be a positive duration", since)
	}
	to := now.UTC()
	from := to.Add(-d)
	return from.Format("2006-01-02"), to.Format("2006-01-02"), nil
}

// parseLiveopsTagCounts parses the flat {tag_name: count} dictionary the
// count-by-tags endpoint returns, sorted by count descending (ties break
// alphabetically for stable output). Non-numeric values are skipped; a
// {data:{...}} envelope is tolerated as a fallback shape.
func parseLiveopsTagCounts(raw json.RawMessage) ([]webLiveopsTagRow, error) {
	var dict map[string]json.RawMessage
	if err := json.Unmarshal(raw, &dict); err != nil {
		return nil, fmt.Errorf("unexpected count-by-tags response shape: %w", err)
	}
	rows := make([]webLiveopsTagRow, 0, len(dict))
	for tag, v := range dict {
		var n float64
		if json.Unmarshal(v, &n) != nil {
			continue
		}
		rows = append(rows, webLiveopsTagRow{Tag: tag, Count: int64(n)})
	}
	// Fallback: some web-surface responses wrap payloads in {data:{...}}.
	if len(rows) == 0 {
		if inner, ok := dict["data"]; ok && !isRawJSONNull(inner) {
			var innerDict map[string]json.RawMessage
			if json.Unmarshal(inner, &innerDict) == nil && len(innerDict) > 0 {
				for tag, v := range innerDict {
					var n float64
					if json.Unmarshal(v, &n) != nil {
						continue
					}
					rows = append(rows, webLiveopsTagRow{Tag: tag, Count: int64(n)})
				}
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Tag < rows[j].Tag
	})
	return rows, nil
}

func newNovelWebLiveopsTagsCmd(flags *rootFlags) *cobra.Command {
	var flagStore int
	var flagCountry string
	var flagSince string

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "liveops-tags",
		Short: "Market-level counts of live-ops events grouped by tag for a store and country.",
		Long: "Use this command to see which live-ops event types dominate a market: it returns event counts grouped by feature-intelligence tag for a store and country over a recent window. " +
			"This command uses the UNOFFICIAL appmagic.rocks web surface and needs APPMAGIC_WEB_TOKEN (Bearer token from a logged-in browser session, localStorage key 'datamagic.token'). The surface can change without notice.",
		Example: strings.Trim(`
  appmagic-pp-cli web liveops-tags --store 1 --country US
  appmagic-pp-cli web liveops-tags --store 2 --country US --since 90d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch live-ops event counts by tag from the appmagic.rocks web surface")
				return nil
			}
			from, to, err := liveopsTagsWindow(flagSince, time.Now())
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
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
			raw, err := wc.Post(ctx, "/feature-intelligence/live-ops/count-by-tags", map[string]any{
				"store":    flagStore,
				"country":  flagCountry,
				"dateFrom": from,
				"dateTo":   to,
			})
			if err != nil {
				return webSurfaceErr(err)
			}
			rows, err := parseLiveopsTagCounts(raw)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagStore, "store", 1, "Store to query: 1 = Google Play, 2 = iPhone App Store, 3 = iPad App Store")
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Two-letter country code scoping the live-ops events, e.g. US, GB, JP")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Lookback window for live-ops events (loose duration: 30d, 12w, 24h)")
	return cmd
}
