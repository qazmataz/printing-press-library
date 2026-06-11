// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
	"github.com/spf13/cobra"
)

// overlayCalendarEntry is one dated event appearance from
// GET /live-ops/live-ops-calendar.
type overlayCalendarEntry struct {
	LiveOpsID string `json:"live_ops_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// overlayCatalogEvent carries the descriptive metadata for a live-ops event
// from GET /live-ops/live-ops, keyed by live_ops_id.
type overlayCatalogEvent struct {
	Name     string
	Duration []string
}

// overlayDayMetrics is one day of united history for the overlaid app.
type overlayDayMetrics struct {
	Downloads float64
	Revenue   float64
}

type overlayEventView struct {
	Event              string   `json:"event"`
	LiveOpsID          string   `json:"live_ops_id,omitempty"`
	Start              string   `json:"start"`
	End                string   `json:"end,omitempty"`
	Duration           []string `json:"duration,omitempty"`
	DownloadsBeforeAvg *float64 `json:"downloads_before_avg,omitempty"`
	DownloadsAfterAvg  *float64 `json:"downloads_after_avg,omitempty"`
	DownloadsDeltaPct  *float64 `json:"downloads_delta_pct,omitempty"`
	RevenueBeforeAvg   *float64 `json:"revenue_before_avg,omitempty"`
	RevenueAfterAvg    *float64 `json:"revenue_after_avg,omitempty"`
	RevenueDeltaPct    *float64 `json:"revenue_delta_pct,omitempty"`
}

type liveopsOverlayView struct {
	App           string             `json:"app"`
	Game          string             `json:"game,omitempty"`
	Country       string             `json:"country"`
	WindowDays    int                `json:"window_days"`
	Events        []overlayEventView `json:"events"`
	ScannedEvents int                `json:"scanned_events"`
	Note          string             `json:"note,omitempty"`
	FetchFailures []string           `json:"fetch_failures,omitempty"`
}

func overlayDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func overlayRound2(v float64) float64 {
	return math.Round(v*100) / 100
}

// overlayNeededDates returns the deduped, ascending list of YYYY-MM-DD dates
// the before/after windows of the given event starts require. Each event
// needs [start-windowDays, start+windowDays). Dates after maxDate (history
// availability) are clipped.
func overlayNeededDates(starts []time.Time, windowDays int, maxDate time.Time) []string {
	maxDate = overlayDay(maxDate)
	seen := make(map[string]bool)
	dates := make([]string, 0)
	for _, start := range starts {
		start = overlayDay(start)
		for d := start.AddDate(0, 0, -windowDays); d.Before(start.AddDate(0, 0, windowDays)); d = d.AddDate(0, 0, 1) {
			if d.After(maxDate) {
				continue
			}
			key := d.Format("2006-01-02")
			if seen[key] {
				continue
			}
			seen[key] = true
			dates = append(dates, key)
		}
	}
	sort.Strings(dates)
	return dates
}

// overlayDelta computes the average daily value of one metric in the
// [start-windowDays, start) window versus the [start, start+windowDays)
// window, plus the percentage change. Averages cover only the days present
// in the series (missing history days do not zero-dilute the mean). A side
// with no data yields nil; a zero before-average yields a nil delta
// percentage (undefined growth).
func overlayDelta(series map[string]overlayDayMetrics, start time.Time, windowDays int, metric func(overlayDayMetrics) float64) (beforeAvg, afterAvg, deltaPct *float64) {
	start = overlayDay(start)
	avgIn := func(from, to time.Time) *float64 {
		var sum float64
		var n int
		for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
			m, ok := series[d.Format("2006-01-02")]
			if !ok {
				continue
			}
			sum += metric(m)
			n++
		}
		if n == 0 {
			return nil
		}
		v := overlayRound2(sum / float64(n))
		return &v
	}
	beforeAvg = avgIn(start.AddDate(0, 0, -windowDays), start)
	afterAvg = avgIn(start, start.AddDate(0, 0, windowDays))
	if beforeAvg != nil && afterAvg != nil && *beforeAvg != 0 {
		pct := overlayRound2((*afterAvg - *beforeAvg) / *beforeAvg * 100)
		deltaPct = &pct
	}
	return beforeAvg, afterAvg, deltaPct
}

// parseOverlayCalendar decodes the bare-array live-ops-calendar response.
func parseOverlayCalendar(data json.RawMessage) []overlayCalendarEntry {
	var rows []overlayCalendarEntry
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil
	}
	return rows
}

// parseOverlayCatalog indexes the bare-array /live-ops/live-ops response by
// live_ops_id for event labels and duration tags.
func parseOverlayCatalog(data json.RawMessage) map[string]overlayCatalogEvent {
	var rows []struct {
		Name      string   `json:"name"`
		LiveOpsID string   `json:"live_ops_id"`
		Duration  []string `json:"duration"`
	}
	out := make(map[string]overlayCatalogEvent)
	if err := json.Unmarshal(data, &rows); err != nil {
		return out
	}
	for _, r := range rows {
		if r.LiveOpsID == "" {
			continue
		}
		out[r.LiveOpsID] = overlayCatalogEvent{Name: r.Name, Duration: r.Duration}
	}
	return out
}

// overlayGame is one row of GET /live-ops/games (the curated coverage list).
type overlayGame struct {
	GameName           string `json:"game_name"`
	StoreApplicationID string `json:"store_application_id"`
}

// overlayMatchGame picks the covered live-ops game matching the resolved app:
// a store-id match wins, then an exact case-insensitive name match, then a
// single-row server-filtered result.
func overlayMatchGame(games []overlayGame, app *unitedApp) (string, bool) {
	appIDs := make(map[string]bool, len(app.StoreApplicationIDs))
	for _, id := range app.StoreApplicationIDs {
		appIDs[strings.ToLower(id)] = true
	}
	for _, g := range games {
		if g.StoreApplicationID != "" && appIDs[strings.ToLower(stripStorePrefix(g.StoreApplicationID))] {
			return g.GameName, true
		}
	}
	for _, g := range games {
		if g.GameName != "" && strings.EqualFold(g.GameName, app.Name) {
			return g.GameName, true
		}
	}
	if len(games) == 1 && games[0].GameName != "" {
		return games[0].GameName, true
	}
	return "", false
}

func newNovelLiveopsOverlayCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string
	var flagSince string
	var flagWindow int
	var flagMaxEvents int

	// pp:data-source live
	cmd := &cobra.Command{
		Use:   "liveops-overlay <app-name-or-id>",
		Short: "Correlate a competitor's live-ops events with their revenue and download movement.",
		Long: "Use this command to overlay a covered game's live-ops event start dates onto its daily downloads and " +
			"revenue history (store 5, all stores united) and report before/after deltas per event. Live-ops data " +
			"covers a curated game list; uncovered apps return a note instead of events. Do NOT use it for the raw " +
			"event catalog; use 'live-ops get' instead.",
		Example: strings.Trim(`
  appmagic-pp-cli liveops-overlay "Royal Match" --since 90d --window 7
  appmagic-pp-cli liveops-overlay 6092344 --country US --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would overlay live-ops events on the app's downloads and revenue history")
				return nil
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("exactly one app name or united application id argument is required"))
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if flagWindow < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --window %d: must be at least 1 day", flagWindow))
			}
			if flagMaxEvents < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --max-events %d: must be at least 1", flagMaxEvents))
			}
			since, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil || since <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: use loose durations like 90d, 12w", flagSince))
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			app, err := resolveUnitedApp(ctx, c, args[0])
			if err != nil {
				return err
			}
			if app.ID == 0 {
				return notFoundErr(fmt.Errorf("could not resolve %q to a united application id", args[0]))
			}

			view := liveopsOverlayView{
				App:        app.Name,
				Country:    flagCountry,
				WindowDays: flagWindow,
				Events:     make([]overlayEventView, 0),
			}

			// Live-ops covers a curated game list; check coverage first.
			gamesData, err := c.Get(ctx, "/live-ops/games", map[string]string{"game_name": app.Name})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var games []overlayGame
			_ = json.Unmarshal(gamesData, &games)
			gameName, covered := overlayMatchGame(games, app)
			if !covered {
				view.Note = fmt.Sprintf("app %q is not in the live-ops coverage list; run 'appmagic-pp-cli live-ops get-games' to see covered titles", app.Name)
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			view.Game = gameName

			today := overlayDay(time.Now().UTC())
			days := int(since.Hours() / 24)
			if days < 1 {
				days = 1
			}
			windowStart := today.AddDate(0, 0, -days)

			calData, err := c.Get(ctx, "/live-ops/live-ops-calendar", map[string]string{
				"game_name": gameName,
				"date":      windowStart.Format("2006-01-02"),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			entries := parseOverlayCalendar(calData)
			type datedEntry struct {
				entry overlayCalendarEntry
				start time.Time
			}
			inWindow := make([]datedEntry, 0, len(entries))
			for _, e := range entries {
				start, err := time.ParseInLocation("2006-01-02", e.StartDate, time.UTC)
				if err != nil {
					continue
				}
				if start.Before(windowStart) || start.After(today) {
					continue
				}
				inWindow = append(inWindow, datedEntry{entry: e, start: start})
			}
			sort.Slice(inWindow, func(i, j int) bool { return inWindow[i].start.After(inWindow[j].start) })
			view.ScannedEvents = len(inWindow)

			maxEvents := flagMaxEvents
			if cliutil.IsDogfoodEnv() && maxEvents > 1 {
				maxEvents = 1
			}
			capped := false
			if len(inWindow) > maxEvents {
				inWindow = inWindow[:maxEvents]
				capped = true
			}
			if len(inWindow) == 0 {
				view.Note = fmt.Sprintf("no live-ops events with a start date in the last %s for %q; widen --since to scan further back", flagSince, gameName)
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			attempted, succeeded := 0, 0
			fetchFailures := make([]string, 0)
			catalog := make(map[string]overlayCatalogEvent)
			attempted++
			catData, err := c.Get(ctx, "/live-ops/live-ops", map[string]string{"game_name": gameName})
			if err != nil {
				fetchFailures = append(fetchFailures, fmt.Sprintf("live-ops catalog: %v", err))
			} else {
				succeeded++
				catalog = parseOverlayCatalog(catData)
			}

			starts := make([]time.Time, 0, len(inWindow))
			for _, e := range inWindow {
				starts = append(starts, e.start)
			}
			// History availability trails today by ~2 days.
			dates := overlayNeededDates(starts, flagWindow, today.AddDate(0, 0, -2))
			if cliutil.IsDogfoodEnv() && len(dates) > 3 {
				dates = dates[len(dates)-3:]
			}

			series := make(map[string]overlayDayMetrics, len(dates))
			for _, date := range dates {
				attempted++
				histData, err := c.Get(ctx, "/history/united-applications", map[string]string{
					"date":                   date,
					"store":                  "5",
					"country":                flagCountry,
					"aggregation":            "daily",
					"united_application_ids": strconv.FormatInt(app.ID, 10),
				})
				if err != nil {
					fetchFailures = append(fetchFailures, fmt.Sprintf("history %s: %v", date, err))
					continue
				}
				succeeded++
				var rows []struct {
					UnitedApplicationID int64   `json:"united_application_id"`
					Date                string  `json:"date"`
					Downloads           float64 `json:"downloads"`
					Revenue             float64 `json:"revenue"`
				}
				if err := json.Unmarshal(histData, &rows); err != nil {
					fetchFailures = append(fetchFailures, fmt.Sprintf("history %s: unexpected response shape", date))
					continue
				}
				for _, r := range rows {
					if r.UnitedApplicationID != app.ID {
						continue
					}
					d := r.Date
					if d == "" {
						d = date
					}
					series[d] = overlayDayMetrics{Downloads: r.Downloads, Revenue: r.Revenue}
				}
			}
			if len(fetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed; deltas computed over %d successful fetches\n",
					len(fetchFailures), attempted, succeeded)
			}

			for _, e := range inWindow {
				ev := overlayEventView{
					Event:     e.entry.LiveOpsID,
					LiveOpsID: e.entry.LiveOpsID,
					Start:     e.entry.StartDate,
					End:       e.entry.EndDate,
				}
				if meta, ok := catalog[e.entry.LiveOpsID]; ok {
					if meta.Name != "" {
						ev.Event = meta.Name
					}
					ev.Duration = meta.Duration
				}
				ev.DownloadsBeforeAvg, ev.DownloadsAfterAvg, ev.DownloadsDeltaPct = overlayDelta(series, e.start, flagWindow,
					func(m overlayDayMetrics) float64 { return m.Downloads })
				ev.RevenueBeforeAvg, ev.RevenueAfterAvg, ev.RevenueDeltaPct = overlayDelta(series, e.start, flagWindow,
					func(m overlayDayMetrics) float64 { return m.Revenue })
				view.Events = append(view.Events, ev)
			}

			if capped {
				view.Note = fmt.Sprintf("showing the %d most recent of %d events in the window; raise --max-events to widen", maxEvents, view.ScannedEvents)
			}
			if len(fetchFailures) > 0 {
				view.FetchFailures = fetchFailures
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "WW", "Country or region code for the downloads/revenue history (e.g. WW, US)")
	cmd.Flags().StringVar(&flagSince, "since", "90d", "How far back to scan for live-ops event starts; loose durations like 90d, 12w")
	cmd.Flags().IntVar(&flagWindow, "window", 7, "Days averaged on each side of an event start for the before/after delta")
	cmd.Flags().IntVar(&flagMaxEvents, "max-events", 10, "Maximum number of most-recent events to overlay; the calendar scan cap")
	return cmd
}
