// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	// pp:data-source local
	cmd := &cobra.Command{
		Use:   "remove <app-name-or-united-id>",
		Short: "Delete a saved competitor from your local watchlist by exact name or united id.",
		Long: "Use this command to drop an app from the locally saved watchlist. The positional argument must " +
			"exactly match a saved row's united application id or its name (name matching is case-insensitive " +
			"but otherwise exact; run 'watchlist list' to see the saved spellings). The delete is entirely local.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  appmagic-pp-cli watchlist remove "Royal Match"
  appmagic-pp-cli watchlist remove 6346813 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would remove the matching app from the local watchlist")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an app name or united application id is required"))
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			query := strings.Join(args, " ")
			// Missing-mirror guard: nothing to remove on a machine with no database.
			if snapshotMirrorMissing(cmd, flags, flagDB, "the watchlist is empty") {
				return nil
			}

			ctx := cmd.Context()
			db, _, err := openSnapshotDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			// Select the matching rows first so the removal can be reported.
			rows, err := watchlistReadRows(ctx, db)
			if err != nil {
				return err
			}
			removed := make([]watchlistRow, 0)
			for _, r := range rows {
				if r.UnitedApplicationID == query || strings.EqualFold(r.Name, query) {
					removed = append(removed, r)
				}
			}
			if len(removed) == 0 {
				return notFoundErr(fmt.Errorf("%q is not on the watchlist; run 'appmagic-pp-cli watchlist list' to see saved apps", query))
			}
			res, err := db.DB().ExecContext(ctx, `
				DELETE FROM watchlist
				WHERE united_application_id = ? OR LOWER(name) = LOWER(?)`, query, query)
			if err != nil {
				return fmt.Errorf("deleting watchlist row: %w", err)
			}
			count, _ := res.RowsAffected()

			view := struct {
				Removed []watchlistRow `json:"removed"`
				Count   int64          `json:"count"`
			}{Removed: removed, Count: count}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite watchlist database (default: ~/.local/share/appmagic-pp-cli/data.db)")
	return cmd
}
