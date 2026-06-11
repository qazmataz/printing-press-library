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
	"github.com/spf13/cobra"
)

// asoTermPosition is one keyword's standing on a single snapshot date, parsed
// from the POST /aso/by_app (or /asa/by_app) response.
type asoTermPosition struct {
	Term       string
	Position   int // medianPlace in the API response
	Score      float64
	Popularity float64
}

type asoMoverGained struct {
	Term       string  `json:"term"`
	Position   int     `json:"position"`
	Score      float64 `json:"score,omitempty"`
	Popularity float64 `json:"popularity,omitempty"`
}

type asoMoverLost struct {
	Term     string `json:"term"`
	Position int    `json:"position"`
}

type asoMoverMoved struct {
	Term         string `json:"term"`
	FromPosition int    `json:"from_position"`
	ToPosition   int    `json:"to_position"`
	Delta        int    `json:"delta"`
}

type asoMoversView struct {
	App      string           `json:"app"`
	Dataset  string           `json:"dataset"`
	Country  string           `json:"country"`
	FromDate string           `json:"from_date"`
	ToDate   string           `json:"to_date"`
	Gained   []asoMoverGained `json:"gained"`
	Lost     []asoMoverLost   `json:"lost"`
	Moved    []asoMoverMoved  `json:"moved"`
}

// asoMoversLooksLikeStoreID reports whether arg is already a store application
// id: reversed-domain Google Play ids contain '.', App Store ids are all
// digits. Anything else is treated as an app name to resolve.
func asoMoversLooksLikeStoreID(arg string) bool {
	if strings.Contains(arg, ".") {
		return true
	}
	if arg == "" {
		return false
	}
	for _, r := range arg {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// asoMoversPickStoreID picks the resolved app's store application id matching
// the requested store kind: store 1 wants a reversed-domain id, stores 2-4
// want a numeric App Store id. Shape classification is delegated to the
// shared storeForStoreAppID helper.
func asoMoversPickStoreID(app *unitedApp, storeNum int) (string, error) {
	wantStore := 2
	if storeNum == 1 {
		wantStore = 1
	}
	for _, id := range app.StoreApplicationIDs {
		if id == "" {
			continue
		}
		if storeForStoreAppID(id) == wantStore {
			return id, nil
		}
	}
	return "", notFoundErr(fmt.Errorf("resolved app %q has no store-%d application id; pass the store id directly", app.Name, storeNum))
}

// parseAsoMoversTerms flattens the {tops:[{appId, country, terms:[...]}]}
// response into term positions. Duplicate terms keep their first occurrence.
func parseAsoMoversTerms(data json.RawMessage) ([]asoTermPosition, error) {
	var resp struct {
		Tops []struct {
			AppID   string `json:"appId"`
			Country string `json:"country"`
			Terms   []struct {
				Term        string  `json:"term"`
				MedianPlace int     `json:"medianPlace"`
				Score       float64 `json:"score"`
				Popularity  float64 `json:"popularity"`
			} `json:"terms"`
		} `json:"tops"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unexpected by_app response shape: %w", err)
	}
	seen := make(map[string]bool)
	out := make([]asoTermPosition, 0)
	for _, top := range resp.Tops {
		for _, t := range top.Terms {
			if t.Term == "" || seen[t.Term] {
				continue
			}
			seen[t.Term] = true
			out = append(out, asoTermPosition{
				Term:       t.Term,
				Position:   t.MedianPlace,
				Score:      t.Score,
				Popularity: t.Popularity,
			})
		}
	}
	return out, nil
}

// asoComputeMovers diffs two keyword snapshots. Gained terms appear only in
// curr (sorted by best current position), lost terms appear only in prev
// (sorted by best previous position), moved terms changed position (sorted by
// absolute delta, largest first). Delta is from_position - to_position, so a
// positive delta means the keyword climbed. Each list is capped at limit.
func asoComputeMovers(prev, curr []asoTermPosition, limit int) ([]asoMoverGained, []asoMoverLost, []asoMoverMoved) {
	prevByTerm := make(map[string]asoTermPosition, len(prev))
	for _, p := range prev {
		prevByTerm[p.Term] = p
	}
	currByTerm := make(map[string]asoTermPosition, len(curr))
	for _, c := range curr {
		currByTerm[c.Term] = c
	}

	gained := make([]asoMoverGained, 0)
	moved := make([]asoMoverMoved, 0)
	for _, c := range curr {
		p, ok := prevByTerm[c.Term]
		if !ok {
			gained = append(gained, asoMoverGained{Term: c.Term, Position: c.Position, Score: c.Score, Popularity: c.Popularity})
			continue
		}
		if p.Position != c.Position {
			moved = append(moved, asoMoverMoved{
				Term:         c.Term,
				FromPosition: p.Position,
				ToPosition:   c.Position,
				Delta:        p.Position - c.Position,
			})
		}
	}
	lost := make([]asoMoverLost, 0)
	for _, p := range prev {
		if _, ok := currByTerm[p.Term]; !ok {
			lost = append(lost, asoMoverLost{Term: p.Term, Position: p.Position})
		}
	}

	sort.Slice(gained, func(i, j int) bool {
		if gained[i].Position != gained[j].Position {
			return gained[i].Position < gained[j].Position
		}
		return gained[i].Term < gained[j].Term
	})
	sort.Slice(lost, func(i, j int) bool {
		if lost[i].Position != lost[j].Position {
			return lost[i].Position < lost[j].Position
		}
		return lost[i].Term < lost[j].Term
	})
	sort.Slice(moved, func(i, j int) bool {
		di, dj := moved[i].Delta, moved[j].Delta
		if di < 0 {
			di = -di
		}
		if dj < 0 {
			dj = -dj
		}
		if di != dj {
			return di > dj
		}
		return moved[i].Term < moved[j].Term
	})

	if limit > 0 {
		if len(gained) > limit {
			gained = gained[:limit]
		}
		if len(lost) > limit {
			lost = lost[:limit]
		}
		if len(moved) > limit {
			moved = moved[:limit]
		}
	}
	return gained, lost, moved
}

func newNovelAsoMoversCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string
	var flagStore int
	var flagSince string
	var flagDataset string
	var flagLimit int

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "aso-movers <store-app-id-or-name>",
		Short: "See which keywords an app gained, lost, or moved on between two dates.",
		Long: "Use this command to diff one app's keyword positions between two snapshot dates and surface gained, " +
			"lost, and moved terms. In 'moved', delta = from_position - to_position, so a positive delta means the " +
			"keyword climbed. Do NOT use it for raw keyword position dumps; use 'aso get-by-app' instead.",
		Example: strings.Trim(`
  appmagic-pp-cli aso-movers com.dreamgames.royalmatch --country US --since 7d
  appmagic-pp-cli aso-movers "Royal Match" --store 2 --dataset asa --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff keyword positions between two snapshot dates")
				return nil
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("exactly one app store id or app name argument is required"))
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if flagDataset != "aso" && flagDataset != "asa" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --dataset %q: must be aso (organic search) or asa (Apple Search Ads)", flagDataset))
			}
			if flagStore < 1 || flagStore > 4 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --store %d: must be 1-4 (keyword data is per concrete store)", flagStore))
			}
			if flagLimit < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --limit %d: must be at least 1", flagLimit))
			}
			since, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil || since <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: use loose durations like 7d, 4w", flagSince))
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			arg := stripStorePrefix(strings.TrimSpace(args[0]))
			appID := arg
			appLabel := arg
			if !asoMoversLooksLikeStoreID(arg) {
				ua, err := resolveUnitedApp(ctx, c, arg)
				if err != nil {
					return err
				}
				appID, err = asoMoversPickStoreID(ua, flagStore)
				if err != nil {
					return err
				}
				appLabel = ua.Name
			}

			// Latest reliable keyword data trails today by ~2 days.
			to := time.Now().UTC().AddDate(0, 0, -2)
			days := int(since.Hours() / 24)
			if days < 1 {
				days = 1
			}
			from := to.AddDate(0, 0, -days)
			fromDate := from.Format("2006-01-02")
			toDate := to.Format("2006-01-02")

			// Request deeper than --limit so movers beyond the output cap are
			// still observed on both dates; the API caps count at 1000.
			count := flagLimit * 10
			if count < 100 {
				count = 100
			}
			if count > 1000 {
				count = 1000
			}

			path := "/" + flagDataset + "/by_app"
			fetch := func(date string) ([]asoTermPosition, error) {
				body := map[string]any{
					"appIds":    []string{appID},
					"countries": []string{flagCountry},
					"dateFrom":  date,
					"dateTo":    date,
					"count":     count,
				}
				data, _, err := c.Post(ctx, path, body)
				if err != nil {
					return nil, classifyAPIError(err, flags)
				}
				return parseAsoMoversTerms(data)
			}
			prev, err := fetch(fromDate)
			if err != nil {
				return err
			}
			curr, err := fetch(toDate)
			if err != nil {
				return err
			}

			gained, lost, moved := asoComputeMovers(prev, curr, flagLimit)
			view := asoMoversView{
				App:      appLabel,
				Dataset:  flagDataset,
				Country:  flagCountry,
				FromDate: fromDate,
				ToDate:   toDate,
				Gained:   gained,
				Lost:     lost,
				Moved:    moved,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Country code whose keyword rankings are compared (e.g. US, GB)")
	cmd.Flags().IntVar(&flagStore, "store", 1, "Store kind used to pick the app's store id when resolving a name: 1 Google Play, 2 iPhone, 3 iPad, 4 iPhone+iPad")
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Gap between the two compared snapshot dates; loose durations like 7d, 4w")
	cmd.Flags().StringVar(&flagDataset, "dataset", "aso", "Keyword dataset to diff: aso (organic search) or asa (Apple Search Ads)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum entries reported per bucket (gained, lost, and moved)")
	return cmd
}
