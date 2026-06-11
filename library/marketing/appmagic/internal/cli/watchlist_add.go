// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/store"

	"github.com/spf13/cobra"
)

// watchlistRow mirrors one row of the local watchlist table. The
// store_application_ids column stores a JSON-encoded string array so a single
// row carries every per-store identity of the united application.
type watchlistRow struct {
	UnitedApplicationID string   `json:"united_application_id"`
	Name                string   `json:"name"`
	StoreApplicationIDs []string `json:"store_application_ids"`
	AddedAt             string   `json:"added_at"`
}

// watchlistUpsertRow inserts the resolved app into the watchlist, refreshing
// name and store IDs on conflict while preserving the original added_at so
// re-adding an app does not rewrite its history.
func watchlistUpsertRow(ctx context.Context, db *store.Store, row watchlistRow) error {
	idsJSON, err := json.Marshal(row.StoreApplicationIDs)
	if err != nil {
		return fmt.Errorf("encoding store application ids: %w", err)
	}
	_, err = db.DB().ExecContext(ctx, `
		INSERT INTO watchlist (united_application_id, name, store_application_ids, added_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(united_application_id) DO UPDATE SET
			name = excluded.name,
			store_application_ids = excluded.store_application_ids`,
		row.UnitedApplicationID, row.Name, string(idsJSON), row.AddedAt)
	if err != nil {
		return fmt.Errorf("upserting watchlist row: %w", err)
	}
	return nil
}

func newNovelWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "add <app-name-or-united-id>",
		Short: "Resolve an app by name or united id and save it to your local competitor watchlist.",
		Long: "Use this command to add a competitor to the locally stored watchlist. The positional argument " +
			"is resolved against the live AppMagic API (free-text name prefix or exact numeric united " +
			"application id), and the resolved identity, including every per-store application id, is " +
			"upserted into the local SQLite watchlist table. Re-adding an app refreshes its name and store " +
			"ids without changing the original added_at. Pull metrics for the saved set with 'watchlist report'.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  appmagic-pp-cli watchlist add "Royal Match"
  appmagic-pp-cli watchlist add 6346813 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the app against the live API and add it to the local watchlist")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an app name or united application id is required"))
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			arg := strings.Join(args, " ")
			ua, err := resolveUnitedApp(ctx, c, arg)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if ua.ID == 0 {
				return notFoundErr(fmt.Errorf("could not resolve %q to a united application id; try a more specific name", arg))
			}

			db, _, err := openSnapshotDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			row := watchlistRow{
				UnitedApplicationID: strconv.FormatInt(ua.ID, 10),
				Name:                ua.Name,
				StoreApplicationIDs: append(make([]string, 0, len(ua.StoreApplicationIDs)), ua.StoreApplicationIDs...),
				AddedAt:             time.Now().UTC().Format(time.RFC3339),
			}
			if err := watchlistUpsertRow(ctx, db, row); err != nil {
				return err
			}

			view := struct {
				UnitedApplicationID string   `json:"united_application_id"`
				Name                string   `json:"name"`
				PublisherName       string   `json:"publisher_name,omitempty"`
				StoreApplicationIDs []string `json:"store_application_ids"`
				AddedAt             string   `json:"added_at"`
			}{
				UnitedApplicationID: row.UnitedApplicationID,
				Name:                row.Name,
				PublisherName:       ua.PublisherName,
				StoreApplicationIDs: row.StoreApplicationIDs,
				AddedAt:             row.AddedAt,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite watchlist database (default: ~/.local/share/appmagic-pp-cli/data.db)")
	return cmd
}
