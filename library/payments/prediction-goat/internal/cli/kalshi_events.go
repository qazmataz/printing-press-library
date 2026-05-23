// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
)

type kalshiEventItem struct {
	EventTicker       string `json:"event_ticker"`
	SeriesTicker      string `json:"series_ticker,omitempty"`
	Title             string `json:"title"`
	Category          string `json:"category,omitempty"`
	MutuallyExclusive bool   `json:"mutually_exclusive"`
}

func newKalshiEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "events", Short: "Kalshi events"}
	cmd.AddCommand(newKalshiEventsListCmd(flags))
	cmd.AddCommand(newKalshiEventsGetCmd(flags))
	return cmd
}

func newKalshiEventsListCmd(flags *rootFlags) *cobra.Command {
	var cursor, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Kalshi event groups, filterable by series and synced locally",
		Example: `  prediction-goat-pp-cli kalshi events list --data-source live --limit 10 --json
  prediction-goat-pp-cli kalshi events list --series KXELONMARS`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			useLive := flags.dataSource == "live"
			if flags.dataSource == "auto" {
				count, err := kalshiLocalCount(cmd, dbPath, "kalshi_events")
				if err != nil {
					return fmt.Errorf("kalshi events local count: %w", err)
				}
				useLive = count == 0
			}
			var items []kalshiEventItem
			var err error
			if useLive {
				items, err = liveKalshiEvents(cmd, cursor, limit)
			} else {
				items, err = localKalshiEvents(cmd, dbPath, limit)
			}
			if err != nil {
				return fmt.Errorf("kalshi events list: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func newKalshiEventsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <event_ticker>",
		Short:       "Get a Kalshi event by ticker",
		Example:     `  prediction-goat-pp-cli kalshi events get KXELONMARS-99 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			body, err := kalshi.New().Get(cmd.Context(), "/events/"+url.PathEscape(args[0]), url.Values{})
			if err != nil {
				return fmt.Errorf("kalshi events get: %w", err)
			}
			obj, err := kalshiEnvelopeObject(body, "event")
			if err != nil {
				return fmt.Errorf("kalshi events get decode: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), kalshiEventSlim(obj), flags)
		},
	}
	return cmd
}

func liveKalshiEvents(cmd *cobra.Command, cursor string, limit int) ([]kalshiEventItem, error) {
	params := url.Values{"limit": {fmt.Sprint(limit)}}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	body, err := kalshi.New().Get(cmd.Context(), "/events", params)
	if err != nil {
		return nil, err
	}
	var resp kalshi.EventsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	items := make([]kalshiEventItem, 0, len(resp.Events))
	for _, raw := range resp.Events {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) == nil {
			items = append(items, kalshiEventSlim(obj))
		}
	}
	return items, nil
}

func localKalshiEvents(cmd *cobra.Command, dbPath string, limit int) ([]kalshiEventItem, error) {
	rows, err := kalshiLocalRows(cmd, dbPath, "kalshi_events", `ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	items := make([]kalshiEventItem, 0, len(rows))
	for _, obj := range rows {
		items = append(items, kalshiEventSlim(obj))
	}
	return items, nil
}

func kalshiEventSlim(obj map[string]any) kalshiEventItem {
	return kalshiEventItem{EventTicker: jsonString(obj, "event_ticker"), SeriesTicker: jsonString(obj, "series_ticker"), Title: jsonString(obj, "title"), Category: jsonString(obj, "category"), MutuallyExclusive: jsonString(obj, "mutually_exclusive") == "true"}
}
