// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/source/webapi"
	"github.com/spf13/cobra"
)

// webOfferRow is the high-gravity projection of one offer-library entry.
// The raw rows also carry offers[] and offers_calendar[] arrays, which are
// deliberately capped out of the view to keep agent output dense.
type webOfferRow struct {
	GameName  string `json:"game_name"`
	Category  string `json:"category,omitempty"`
	Type      string `json:"type,omitempty"`
	Structure string `json:"structure,omitempty"`
	Duration  any    `json:"duration,omitempty"`
}

// webOffersView is the typed output envelope for `web offers`.
type webOffersView struct {
	TotalCount int           `json:"total_count"`
	PriceRange any           `json:"price_range,omitempty"`
	Offers     []webOfferRow `json:"offers"`
}

// webOffersWindow resolves the --from/--to date window, defaulting to the
// last 30 days ending today (UTC). Explicit values must be YYYY-MM-DD and
// chronologically ordered.
func webOffersWindow(from, to string, now time.Time) (string, string, error) {
	if to == "" {
		to = now.UTC().Format("2006-01-02")
	}
	if from == "" {
		from = now.UTC().AddDate(0, 0, -30).Format("2006-01-02")
	}
	fromT, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", fmt.Errorf("invalid --from %q: expected YYYY-MM-DD", from)
	}
	toT, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", "", fmt.Errorf("invalid --to %q: expected YYYY-MM-DD", to)
	}
	if fromT.After(toT) {
		return "", "", fmt.Errorf("--from %s is after --to %s", from, to)
	}
	return from, to, nil
}

// parseWebOffers extracts the offer library view from the web surface's
// {priceRange, totalCount, data:[...]} envelope. totalCount falls back to
// the row count when absent or non-numeric.
func parseWebOffers(raw json.RawMessage) (*webOffersView, error) {
	var envelope struct {
		TotalCount json.RawMessage  `json:"totalCount"`
		PriceRange any              `json:"priceRange"`
		Data       []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("unexpected offers response shape: %w", err)
	}
	view := &webOffersView{
		PriceRange: envelope.PriceRange,
		Offers:     make([]webOfferRow, 0, len(envelope.Data)),
	}
	var total float64
	if len(envelope.TotalCount) > 0 && json.Unmarshal(envelope.TotalCount, &total) == nil {
		view.TotalCount = int(total)
	} else {
		view.TotalCount = len(envelope.Data)
	}
	for _, m := range envelope.Data {
		row := webOfferRow{
			GameName:  webStrField(m, "game_name", "name"),
			Category:  webStrField(m, "category"),
			Type:      webStrField(m, "type"),
			Structure: webStrField(m, "structure"),
		}
		if d, ok := m["duration"]; ok && d != nil {
			row.Duration = d
		}
		view.Offers = append(view.Offers, row)
	}
	return view, nil
}

func newNovelWebOffersCmd(flags *rootFlags) *cobra.Command {
	var flagStore int
	var flagCountry string
	var flagLimit int
	var flagFrom string
	var flagTo string

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "offers",
		Short: "Browse AppMagic's in-game IAP offer library (structure, duration, pricing)",
		Long: "Use this command to browse the monetization-intelligence library of in-game IAP offers (game, category, offer type, structure, duration) observed across tracked titles in a date window. " +
			"This command uses the UNOFFICIAL appmagic.rocks web surface and needs APPMAGIC_WEB_TOKEN (Bearer token from a logged-in browser session, localStorage key 'datamagic.token'). The surface can change without notice.",
		Example: strings.Trim(`
  appmagic-pp-cli web offers --store 1 --country US
  appmagic-pp-cli web offers --store 2 --country US --limit 50 --from 2026-05-01 --to 2026-06-01 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the in-game offer library from the appmagic.rocks web surface")
				return nil
			}
			from, to, err := webOffersWindow(flagFrom, flagTo, time.Now())
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
			raw, err := wc.Post(ctx, "/monetization-intelligence/offers", map[string]any{
				"store":    flagStore,
				"country":  flagCountry,
				"limit":    flagLimit,
				"dateFrom": from,
				"dateTo":   to,
			})
			if err != nil {
				return webSurfaceErr(err)
			}
			view, err := parseWebOffers(raw)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&flagStore, "store", 1, "Store to query: 1 = Google Play, 2 = iPhone App Store, 3 = iPad App Store")
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Two-letter country code scoping the offer observations, e.g. US, GB, DE")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum number of offer-library entries to return")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start date YYYY-MM-DD (defaults to 30 days ago)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end date YYYY-MM-DD (defaults to today UTC)")
	return cmd
}
