// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"testing"
	"time"
)

func overlayTestDay(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		t.Fatalf("bad fixture date %q: %v", s, err)
	}
	return d
}

func TestLiveopsOverlayDelta(t *testing.T) {
	// Synthetic series: 100 downloads / 1000 revenue per day Jan 1-7,
	// 150 downloads / 800 revenue per day Jan 8-14.
	series := map[string]overlayDayMetrics{}
	for d := 1; d <= 7; d++ {
		series[time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")] = overlayDayMetrics{Downloads: 100, Revenue: 1000}
	}
	for d := 8; d <= 14; d++ {
		series[time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")] = overlayDayMetrics{Downloads: 150, Revenue: 800}
	}
	start := overlayTestDay(t, "2026-01-08")
	downloads := func(m overlayDayMetrics) float64 { return m.Downloads }
	revenue := func(m overlayDayMetrics) float64 { return m.Revenue }

	before, after, pct := overlayDelta(series, start, 7, downloads)
	if before == nil || *before != 100 {
		t.Fatalf("downloads before avg = %v, want 100", before)
	}
	if after == nil || *after != 150 {
		t.Fatalf("downloads after avg = %v, want 150", after)
	}
	if pct == nil || *pct != 50 {
		t.Fatalf("downloads delta pct = %v, want +50", pct)
	}

	before, after, pct = overlayDelta(series, start, 7, revenue)
	if before == nil || *before != 1000 || after == nil || *after != 800 {
		t.Fatalf("revenue avgs = %v / %v, want 1000 / 800", before, after)
	}
	if pct == nil || *pct != -20 {
		t.Fatalf("revenue delta pct = %v, want -20", pct)
	}

	// Missing days average over the days present, not the full window.
	sparse := map[string]overlayDayMetrics{
		"2026-01-05": {Downloads: 90},
		"2026-01-06": {Downloads: 110},
		"2026-01-08": {Downloads: 200},
	}
	before, after, pct = overlayDelta(sparse, start, 7, downloads)
	if before == nil || *before != 100 {
		t.Errorf("sparse before avg = %v, want 100 (mean of the 2 present days)", before)
	}
	if after == nil || *after != 200 {
		t.Errorf("sparse after avg = %v, want 200 (single present day)", after)
	}
	if pct == nil || *pct != 100 {
		t.Errorf("sparse delta pct = %v, want +100", pct)
	}

	// A zero before-average has undefined growth: averages set, pct nil.
	zeroBefore := map[string]overlayDayMetrics{
		"2026-01-07": {Downloads: 0},
		"2026-01-08": {Downloads: 50},
	}
	before, after, pct = overlayDelta(zeroBefore, start, 7, downloads)
	if before == nil || *before != 0 || after == nil || *after != 50 {
		t.Errorf("zero-before avgs = %v / %v, want 0 / 50", before, after)
	}
	if pct != nil {
		t.Errorf("zero-before delta pct = %v, want nil (undefined)", *pct)
	}

	// A side with no data at all yields nils across the board.
	afterOnly := map[string]overlayDayMetrics{"2026-01-09": {Downloads: 10}}
	before, after, pct = overlayDelta(afterOnly, start, 7, downloads)
	if before != nil || pct != nil {
		t.Errorf("no before data: before=%v pct=%v, want both nil", before, pct)
	}
	if after == nil || *after != 10 {
		t.Errorf("no before data: after = %v, want 10", after)
	}
}

func TestLiveopsOverlayNeededDates(t *testing.T) {
	maxDate := overlayTestDay(t, "2026-01-11")

	// Two events with overlapping windows: starts Jan 8 and Jan 10,
	// window 3 -> Jan 5..Jan 10 union Jan 7..Jan 12, clipped at Jan 11.
	starts := []time.Time{overlayTestDay(t, "2026-01-08"), overlayTestDay(t, "2026-01-10")}
	got := overlayNeededDates(starts, 3, maxDate)
	want := []string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-10", "2026-01-11"}
	if len(got) != len(want) {
		t.Fatalf("dates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dates[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}

	// An event entirely beyond the availability horizon contributes only
	// its before-window days that fall on or before maxDate.
	future := []time.Time{overlayTestDay(t, "2026-01-13")}
	got = overlayNeededDates(future, 2, maxDate)
	want = []string{"2026-01-11"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("future-event dates = %v, want %v", got, want)
	}

	// No events -> empty, non-nil slice.
	got = overlayNeededDates(nil, 7, maxDate)
	if got == nil || len(got) != 0 {
		t.Fatalf("no events: dates = %v, want empty slice", got)
	}
}

func TestLiveopsOverlayMatchGame(t *testing.T) {
	app := &unitedApp{
		ID:                  42,
		Name:                "Royal Match",
		StoreApplicationIDs: []string{"1482155847", "com.dreamgames.royalmatch"},
	}
	games := []overlayGame{
		{GameName: "Royal Kingdom", StoreApplicationID: "com.dreamgames.royalkingdom"},
		{GameName: "Royal Match", StoreApplicationID: "2_1482155847"},
	}
	name, ok := overlayMatchGame(games, app)
	if !ok || name != "Royal Match" {
		t.Errorf("store-id match = %q/%v, want Royal Match via prefixed store id", name, ok)
	}

	// Name match when store ids do not line up.
	nameOnly := []overlayGame{{GameName: "royal match", StoreApplicationID: "id.elsewhere"}}
	name, ok = overlayMatchGame(nameOnly, app)
	if !ok || name != "royal match" {
		t.Errorf("case-insensitive name match = %q/%v, want royal match", name, ok)
	}

	// Single server-filtered row is trusted.
	single := []overlayGame{{GameName: "Royal Match: Puzzle", StoreApplicationID: "other.id"}}
	if name, ok = overlayMatchGame(single, app); !ok || name != "Royal Match: Puzzle" {
		t.Errorf("single-row fallback = %q/%v, want Royal Match: Puzzle", name, ok)
	}

	// Ambiguous multi-row result without a real match is not covered.
	ambiguous := []overlayGame{
		{GameName: "Match Masters", StoreApplicationID: "a.b.c"},
		{GameName: "Matchington", StoreApplicationID: "d.e.f"},
	}
	if _, ok = overlayMatchGame(ambiguous, app); ok {
		t.Error("ambiguous rows must not match")
	}
}
