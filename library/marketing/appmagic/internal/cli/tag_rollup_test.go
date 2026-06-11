// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

const tagRollupHistoryFixture = `[
  {"united_application_id": 101, "date": "2026-06-01", "country": "US", "store": 5, "downloads": 1000, "revenue": 5000},
  {"united_application_id": 101, "date": "2026-06-02", "country": "US", "store": 5, "downloads": 2000, "revenue": 7000},
  {"united_application_id": 102, "date": "2026-06-01", "country": "US", "store": 5, "downloads": 300, "revenue": 900},
  {"united_application_id": 101, "date": "2026-06-01", "country": "US", "store": 5, "downloads": 1000, "revenue": 5000},
  {"united_application_id": 999, "date": "2026-06-01", "country": "US", "store": 5, "downloads": 50000, "revenue": 99999}
]`

func TestTagRollupSum(t *testing.T) {
	rows, err := parseTagRollupHistoryRows(json.RawMessage(tagRollupHistoryFixture))
	if err != nil {
		t.Fatalf("parseTagRollupHistoryRows: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 fixture rows, got %d", len(rows))
	}
	cohort := map[int64]bool{101: true, 102: true}

	tests := []struct {
		name        string
		rows        []tagRollupHistoryRow
		cohort      map[int64]bool
		metric      string
		wantTotal   float64
		wantCounted int
	}{
		// Duplicate (101, 2026-06-01) row must count once; app 999 is
		// outside the cohort and must be excluded entirely.
		{name: "revenue dedupes and filters cohort", rows: rows, cohort: cohort, metric: "revenue", wantTotal: 5000 + 7000 + 900, wantCounted: 2},
		{name: "downloads dedupes and filters cohort", rows: rows, cohort: cohort, metric: "downloads", wantTotal: 1000 + 2000 + 300, wantCounted: 2},
		{name: "single-app cohort", rows: rows, cohort: map[int64]bool{102: true}, metric: "downloads", wantTotal: 300, wantCounted: 1},
		{name: "empty rows", rows: nil, cohort: cohort, metric: "revenue", wantTotal: 0, wantCounted: 0},
		{name: "cohort with no matching rows", rows: rows, cohort: map[int64]bool{777: true}, metric: "revenue", wantTotal: 0, wantCounted: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			total, counted := tagRollupSum(tc.rows, tc.cohort, tc.metric)
			if total != tc.wantTotal {
				t.Errorf("total = %v, want %v", total, tc.wantTotal)
			}
			if counted != tc.wantCounted {
				t.Errorf("apps counted = %d, want %d", counted, tc.wantCounted)
			}
		})
	}
}

func TestTagRollupDates(t *testing.T) {
	day := func(s string) time.Time {
		d, err := time.ParseInLocation("2006-01-02", s, time.UTC)
		if err != nil {
			t.Fatalf("bad fixture date %q: %v", s, err)
		}
		return d
	}

	tests := []struct {
		name      string
		start     string
		end       string
		wantAgg   string
		wantCount int
		wantFirst string
		wantLast  string
	}{
		{name: "single day is daily", start: "2026-06-01", end: "2026-06-01", wantAgg: "daily", wantCount: 1, wantFirst: "2026-06-01", wantLast: "2026-06-01"},
		{name: "30-day window is daily", start: "2026-05-03", end: "2026-06-01", wantAgg: "daily", wantCount: 30, wantFirst: "2026-05-03", wantLast: "2026-06-01"},
		// 2026-03-04 is a Wednesday; its week starts Monday 2026-03-02.
		// 90 days spans 14 distinct ISO weeks.
		{name: "90-day window steps weekly from Monday", start: "2026-03-04", end: "2026-06-01", wantAgg: "weekly", wantCount: 14, wantFirst: "2026-03-02", wantLast: "2026-06-01"},
		{name: "365-day window steps monthly from month start", start: "2025-06-15", end: "2026-06-14", wantAgg: "monthly", wantCount: 13, wantFirst: "2025-06-01", wantLast: "2026-06-01"},
		{name: "swapped bounds normalize", start: "2026-06-01", end: "2026-05-30", wantAgg: "daily", wantCount: 3, wantFirst: "2026-05-30", wantLast: "2026-06-01"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			agg, dates := tagRollupDates(day(tc.start), day(tc.end))
			if agg != tc.wantAgg {
				t.Errorf("aggregation = %q, want %q", agg, tc.wantAgg)
			}
			if len(dates) != tc.wantCount {
				t.Fatalf("len(dates) = %d, want %d (dates: %v)", len(dates), tc.wantCount, dates)
			}
			if dates[0] != tc.wantFirst {
				t.Errorf("first date = %q, want %q", dates[0], tc.wantFirst)
			}
			if dates[len(dates)-1] != tc.wantLast {
				t.Errorf("last date = %q, want %q", dates[len(dates)-1], tc.wantLast)
			}
			seen := map[string]bool{}
			for _, d := range dates {
				if seen[d] {
					t.Errorf("duplicate request date %q", d)
				}
				seen[d] = true
			}
		})
	}
}

func TestTagRollupParseTopIDs(t *testing.T) {
	fixture := json.RawMessage(`[
	  {"place": 1, "united_application_id": 11, "value": 900},
	  {"place": 2, "united_application_id": 22, "value": 800},
	  {"place": 3, "united_application_id": 0, "value": 700},
	  {"place": 4, "united_application_id": 44, "value": 600}
	]`)
	ids := parseTagRollupTopIDs(fixture, 2)
	if len(ids) != 2 || ids[0] != 11 || ids[1] != 22 {
		t.Errorf("cap 2: ids = %v, want [11 22]", ids)
	}
	ids = parseTagRollupTopIDs(fixture, 10)
	if len(ids) != 3 || ids[2] != 44 {
		t.Errorf("zero-id rows must be skipped: ids = %v, want [11 22 44]", ids)
	}
	if got := parseTagRollupTopIDs(json.RawMessage(`{"not":"an array"}`), 5); len(got) != 0 {
		t.Errorf("non-array response should yield no ids, got %v", got)
	}
}
