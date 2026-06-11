// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNovelWatchlistReportMetricsParsing(t *testing.T) {
	tests := []struct {
		name    string
		csv     string
		want    []string
		wantErr bool
	}{
		{name: "default pair", csv: "downloads,revenue", want: []string{"downloads", "revenue"}},
		{name: "all three with spaces and case", csv: " Downloads, REVENUE ,retention", want: []string{"downloads", "revenue", "retention"}},
		{name: "retention only", csv: "retention", want: []string{"retention"}},
		{name: "trailing comma tolerated", csv: "downloads,", want: []string{"downloads"}},
		{name: "unknown metric rejected", csv: "downloads,dau", wantErr: true},
		{name: "empty csv rejected", csv: "", wantErr: true},
		{name: "only commas rejected", csv: ",,", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := wlParseMetrics(tt.csv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("wlParseMetrics(%q) = %v, want error", tt.csv, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("wlParseMetrics(%q) unexpected error: %v", tt.csv, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("wlParseMetrics(%q) = %v, want exactly %v", tt.csv, got, tt.want)
			}
			for _, m := range tt.want {
				if !got[m] {
					t.Errorf("wlParseMetrics(%q) missing metric %q", tt.csv, m)
				}
			}
		})
	}
}

func TestNovelWatchlistReportHistoryAggregation(t *testing.T) {
	i64 := func(v int64) *int64 { return &v }
	fixture := []wlHistoryRecord{
		{UnitedApplicationID: 100, Date: "2026-06-01", Downloads: i64(1000), Revenue: i64(5000)},
		{UnitedApplicationID: 100, Date: "2026-06-02", Downloads: i64(2000), Revenue: i64(7000)},
		{UnitedApplicationID: 100, Date: "2026-06-03", Downloads: nil, Revenue: i64(1)}, // NULL downloads counts as zero
		{UnitedApplicationID: 200, Date: "2026-06-01", Downloads: i64(50), Revenue: nil},
		{UnitedApplicationID: 999, Date: "2026-06-01", Downloads: i64(7), Revenue: i64(7)}, // not on the watchlist
	}
	want := map[int64]bool{100: true, 200: true, 300: true}

	got := wlAggregateHistory(fixture, want)

	if agg := got[100]; agg == nil || agg.Downloads != 3000 || agg.Revenue != 12001 {
		t.Errorf("app 100 = %+v, want downloads 3000 revenue 12001", got[100])
	}
	if agg := got[200]; agg == nil || agg.Downloads != 50 || agg.Revenue != 0 {
		t.Errorf("app 200 = %+v, want downloads 50 revenue 0 (NULL revenue)", got[200])
	}
	if _, ok := got[999]; ok {
		t.Errorf("app 999 aggregated but it is not on the watchlist")
	}
	if _, ok := got[300]; ok {
		t.Errorf("app 300 has no history rows; expected no aggregate entry")
	}

	// Defensive parse: bare array and {data:[...]} envelope both decode.
	bare := json.RawMessage(`[{"united_application_id":100,"date":"2026-06-01","downloads":10,"revenue":null}]`)
	if rows, err := wlParseHistoryRecords(bare); err != nil || len(rows) != 1 || rows[0].Downloads == nil || *rows[0].Downloads != 10 || rows[0].Revenue != nil {
		t.Errorf("wlParseHistoryRecords(bare array) = %+v, %v, want one row with downloads 10 and NULL revenue", rows, err)
	}
	wrapped := json.RawMessage(`{"data":[{"united_application_id":200,"date":"2026-06-01","downloads":1,"revenue":2}]}`)
	if rows, err := wlParseHistoryRecords(wrapped); err != nil || len(rows) != 1 || rows[0].UnitedApplicationID != 200 {
		t.Errorf("wlParseHistoryRecords(envelope) = %+v, %v, want one row for app 200", rows, err)
	}
	if rows, err := wlParseHistoryRecords(json.RawMessage(`"nonsense"`)); err == nil {
		t.Errorf("wlParseHistoryRecords(non-array) = %+v, want a shape error", rows)
	}
	if rows, err := wlParseHistoryRecords(json.RawMessage(`{"data":null}`)); err == nil {
		t.Errorf("wlParseHistoryRecords(envelope with null data) = %+v, want a shape error", rows)
	}
}

func TestNovelWatchlistReportWindowDates(t *testing.T) {
	end := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)

	got := wlWindowDates(end, wlWindowDays(7*24*time.Hour))
	if len(got) != 7 {
		t.Fatalf("7d window = %d dates %v, want 7", len(got), got)
	}
	if got[0] != "2026-06-02" || got[6] != "2026-06-08" {
		t.Errorf("7d window = %v, want 2026-06-02 .. 2026-06-08", got)
	}

	// Partial days round up; sub-day windows floor at one date.
	if days := wlWindowDays(36 * time.Hour); days != 2 {
		t.Errorf("wlWindowDays(36h) = %d, want 2", days)
	}
	if got := wlWindowDates(end, wlWindowDays(time.Hour)); len(got) != 1 || got[0] != "2026-06-08" {
		t.Errorf("1h window = %v, want exactly [2026-06-08]", got)
	}
}

func TestNovelWatchlistReportStoreInference(t *testing.T) {
	tests := []struct {
		id   string
		want int
	}{
		{"com.dreamgames.royalmatch", 1}, // reversed-domain -> Google Play
		{"835599320", 2},                 // numeric -> iPhone App Store
		{"545519333", 2},
		{"com.king.candycrushsaga", 1},
		{"id835599320", 1}, // letters present -> not a numeric App Store id
		{"", 1},            // defensive default
	}
	for _, tt := range tests {
		if got := storeForStoreAppID(tt.id); got != tt.want {
			t.Errorf("storeForStoreAppID(%q) = %d, want %d", tt.id, got, tt.want)
		}
	}
}
