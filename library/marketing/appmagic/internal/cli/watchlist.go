// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelWatchlistCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "watchlist",
		Short:       "watchlist subcommands: add, list, remove, report",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelWatchlistAddCmd(flags))
	cmd.AddCommand(newNovelWatchlistListCmd(flags))
	cmd.AddCommand(newNovelWatchlistRemoveCmd(flags))
	cmd.AddCommand(newNovelWatchlistReportCmd(flags))
	return cmd
}
