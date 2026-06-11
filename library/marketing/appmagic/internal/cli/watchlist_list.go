// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/store"

	"github.com/spf13/cobra"
)

// watchlistReadRows loads every saved watchlist row, newest first. The
// store_application_ids column is JSON-decoded defensively: a malformed value
// degrades to an empty id list rather than failing the whole read.
func watchlistReadRows(ctx context.Context, db *store.Store) ([]watchlistRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT united_application_id, name, store_application_ids, added_at
		FROM watchlist
		ORDER BY added_at DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("reading watchlist: %w", err)
	}
	defer rows.Close()

	out := make([]watchlistRow, 0)
	for rows.Next() {
		var r watchlistRow
		var idsJSON string
		if err := rows.Scan(&r.UnitedApplicationID, &r.Name, &idsJSON, &r.AddedAt); err != nil {
			return nil, fmt.Errorf("scanning watchlist row: %w", err)
		}
		r.StoreApplicationIDs = make([]string, 0)
		_ = json.Unmarshal([]byte(idsJSON), &r.StoreApplicationIDs)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating watchlist rows: %w", err)
	}
	return out, nil
}

func newNovelWatchlistListCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	// pp:data-source local
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show every competitor saved on your local watchlist, newest first.",
		Long: "Use this command to review the locally saved competitor watchlist: united application id, " +
			"resolved name, per-store application ids, and when each app was added. The read is entirely " +
			"local (no network); populate the list with 'watchlist add' and pull metrics with 'watchlist report'.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  appmagic-pp-cli watchlist list
  appmagic-pp-cli watchlist list --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list the locally saved competitor watchlist")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			// Missing-mirror guard: a brand-new machine has no database yet.
			if snapshotMirrorMissing(cmd, flags, flagDB,
				"run 'appmagic-pp-cli watchlist add <app>' to start a watchlist") {
				return nil
			}

			ctx := cmd.Context()
			db, _, err := openSnapshotDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			items, err := watchlistReadRows(ctx, db)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: the watchlist is empty; run 'appmagic-pp-cli watchlist add <app>' to save a competitor")
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite watchlist database (default: ~/.local/share/appmagic-pp-cli/data.db)")
	return cmd
}
