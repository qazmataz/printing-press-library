// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/source/webapi"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/store"
	"github.com/spf13/cobra"
)

// webTagCountRow is one taxonomy tag with the number of apps carrying it.
// TagName is filled best-effort from the local synced taxonomy.
type webTagCountRow struct {
	TagID   int64  `json:"tag_id"`
	TagName string `json:"tag_name,omitempty"`
	Count   int64  `json:"count"`
}

// parseTagAppCounts parses the {data:[{tags:[tag_id], count}]} response into
// per-tag rows sorted by count descending (ties break by tag id ascending).
// Rows with no tag ids are skipped; ids arriving as JSON strings are coerced.
func parseTagAppCounts(raw json.RawMessage) ([]webTagCountRow, error) {
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("unexpected tags/apps-count response shape: %w", err)
	}
	rows := make([]webTagCountRow, 0, len(envelope.Data))
	for _, m := range envelope.Data {
		tagsRaw, ok := m["tags"].([]any)
		if !ok || len(tagsRaw) == 0 {
			continue
		}
		var tagID int64
		switch v := tagsRaw[0].(type) {
		case float64:
			tagID = int64(v)
		case string:
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				continue
			}
			tagID = parsed
		default:
			continue
		}
		var count int64
		if v, ok := webNumField(m, "count"); ok {
			count = int64(v)
		}
		rows = append(rows, webTagCountRow{TagID: tagID, Count: count})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].TagID < rows[j].TagID
	})
	return rows, nil
}

// applyTagNames joins tag ids to names in place. Missing ids stay unnamed;
// a nil map is a no-op so callers can pass the best-effort lookup straight in.
func applyTagNames(rows []webTagCountRow, names map[int64]string) {
	if len(names) == 0 {
		return
	}
	for i := range rows {
		if n, ok := names[rows[i].TagID]; ok && n != "" {
			rows[i].TagName = n
		}
	}
}

// loadLocalTagNames reads the synced tag taxonomy from the local mirror,
// best-effort: any failure (missing db, missing table, no synced tags)
// returns nil so enrichment silently degrades to id-only rows.
func loadLocalTagNames(ctx context.Context, dbPath string) map[int64]string {
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, COALESCE(json_extract(data, '$.name'), '') FROM resources
		WHERE resource_type IN ('tags', 'tag')`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	names := map[int64]string{}
	for rows.Next() {
		var idStr, name string
		if rows.Scan(&idStr, &name) != nil {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || name == "" {
			continue
		}
		names[id] = name
	}
	if rows.Err() != nil {
		return names
	}
	return names
}

func newNovelWebTagCountsCmd(flags *rootFlags) *cobra.Command {
	var flagStore int
	var flagCountry string
	var flagDate string
	var flagDB string

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "tag-counts",
		Short: "Number of apps per taxonomy tag for market-niche sizing.",
		Long: "Use this command to size market niches by the number of apps carrying each taxonomy tag on a given date; tag ids are enriched with names when the local taxonomy is synced (appmagic-pp-cli sync --resources tags). " +
			"This command uses the UNOFFICIAL appmagic.rocks web surface and needs APPMAGIC_WEB_TOKEN (Bearer token from a logged-in browser session, localStorage key 'datamagic.token'). The surface can change without notice.",
		Example: strings.Trim(`
  appmagic-pp-cli web tag-counts --store 1 --country WW
  appmagic-pp-cli web tag-counts --store 2 --country US --date 2026-06-01 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch per-tag app counts from the appmagic.rocks web surface")
				return nil
			}
			date := flagDate
			if date == "" {
				date = time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
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
			raw, err := wc.Get(ctx, "/tags/apps-count", map[string]string{
				"store":   strconv.Itoa(flagStore),
				"country": flagCountry,
				"date":    date,
			})
			if err != nil {
				return webSurfaceErr(err)
			}
			rows, err := parseTagAppCounts(raw)
			if err != nil {
				return apiErr(err)
			}
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("appmagic-pp-cli")
			}
			applyTagNames(rows, loadLocalTagNames(ctx, dbPath))
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagStore, "store", 1, "Store to query: 1 = Google Play, 2 = iPhone App Store, 3 = iPad App Store")
	cmd.Flags().StringVar(&flagCountry, "country", "WW", "Two-letter country code or WW for the worldwide aggregate")
	cmd.Flags().StringVar(&flagDate, "date", "", "Count date in YYYY-MM-DD format (defaults to two days ago UTC, the latest settled date)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite mirror used for best-effort tag-name enrichment (default ~/.local/share/appmagic-pp-cli/data.db)")
	return cmd
}
