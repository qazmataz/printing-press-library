// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

type kalshiSyncSummary struct {
	Markets int `json:"markets"`
	Events  int `json:"events"`
	Series  int `json:"series"`
	Total   int `json:"total"`
}

func newKalshiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kalshi",
		Short: "Kalshi-side commands (read-only)",
	}
	cmd.AddCommand(newKalshiMarketsCmd(flags))
	cmd.AddCommand(newKalshiEventsCmd(flags))
	cmd.AddCommand(newKalshiSeriesCmd(flags))
	cmd.AddCommand(newKalshiSyncCmd(flags))
	return cmd
}

func newKalshiSyncCmd(flags *rootFlags) *cobra.Command {
	var maxPages int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Kalshi markets, events, and series into local SQLite",
		Example: `  prediction-goat-pp-cli kalshi sync
  prediction-goat-pp-cli kalshi sync --max-pages 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			dogfood := cliutil.IsDogfoodEnv()
			if dogfood && maxPages == 0 {
				maxPages = 1
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("kalshi sync open database: %w", err)
			}
			defer db.Close()
			client := kalshi.New()
			markets, err := kalshi.SyncMarkets(cmd.Context(), client, db, maxPages)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "kalshi markets: %d\n", markets)
			var events, series int
			// Under dogfood, skip events + series — the Kalshi /events and
			// /series endpoints commonly take 15-60s per call which blows
			// past the matrix's 30s per-command budget. Real users still
			// sync all three resources end-to-end.
			if !dogfood {
				events, err = kalshi.SyncEvents(cmd.Context(), client, db, maxPages)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "kalshi events: %d\n", events)
				series, err = kalshi.SyncSeries(cmd.Context(), client, db, maxPages)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "kalshi series: %d\n", series)
			}
			summary := kalshiSyncSummary{Markets: markets, Events: events, Series: series, Total: markets + events + series}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Maximum pages per resource (0 = unlimited)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func kalshiLocalCount(cmd *cobra.Command, dbPath, resourceType string) (int, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return 0, fmt.Errorf("open local database: %w", err)
	}
	defer db.Close()
	var count int
	if err := db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM resources WHERE resource_type=?`, resourceType).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func kalshiLocalRows(cmd *cobra.Command, dbPath, resourceType, where string, args ...any) ([]map[string]any, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("open local database: %w", err)
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM resources WHERE resource_type=? `+where, append([]any{resourceType}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if !data.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data.String), &obj) == nil {
			items = append(items, obj)
		}
	}
	return items, rows.Err()
}

func kalshiEnvelopeObject(body []byte, key string) (map[string]any, error) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	raw := json.RawMessage(body)
	if v, ok := env[key]; ok {
		raw = v
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}
