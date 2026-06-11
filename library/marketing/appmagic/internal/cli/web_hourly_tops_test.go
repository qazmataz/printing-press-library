// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseHourlyTops(t *testing.T) {
	fixture := `{
		"date": "2026-06-10",
		"data": [
			{
				"top_free": {"rank": 1, "diff": 2, "application": {"id": 6249271, "name": "Royal Match", "publisher_name": "Dream Games"}},
				"top_grossing": {"rank": 1, "diff": 0, "application": {"id": 5050, "name": "TikTok"}},
				"top_paid": {"rank": 1, "diff": -1, "application": {"name": "Minecraft"}}
			},
			{
				"top_free": {"rank": 2, "diff": -1, "application": {"id": 777, "name": "Block Blast!"}},
				"top_grossing": null
			}
		]
	}`

	date, rows, err := parseHourlyTops(json.RawMessage(fixture))
	if err != nil {
		t.Fatalf("parseHourlyTops returned error: %v", err)
	}
	if date != "2026-06-10" {
		t.Errorf("date = %q, want 2026-06-10", date)
	}
	want := []webHourlyTopRow{
		{Chart: "free", Rank: 1, Diff: 2, App: "Royal Match", ApplicationID: 6249271, Publisher: "Dream Games"},
		{Chart: "free", Rank: 2, Diff: -1, App: "Block Blast!", ApplicationID: 777},
		{Chart: "grossing", Rank: 1, Diff: 0, App: "TikTok", ApplicationID: 5050},
		{Chart: "paid", Rank: 1, Diff: -1, App: "Minecraft"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows mismatch:\n got: %+v\nwant: %+v", rows, want)
	}
}

func TestParseHourlyTopsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantErr  bool
		wantRows int
		check    func(t *testing.T, rows []webHourlyTopRow)
	}{
		{
			name:     "empty data renders empty non-nil slice",
			raw:      `{"date": "2026-06-10", "data": []}`,
			wantRows: 0,
			check: func(t *testing.T, rows []webHourlyTopRow) {
				if rows == nil {
					t.Error("rows is nil, want empty non-nil slice so JSON renders []")
				}
			},
		},
		{
			name:    "invalid JSON is a typed error",
			raw:     `<html>not json</html>`,
			wantErr: true,
		},
		{
			name:     "missing rank falls back to per-chart position",
			raw:      `{"data": [{"top_free": {"diff": 3, "application": {"name": "A"}}}, {"top_free": {"diff": 0, "application": {"name": "B"}}}]}`,
			wantRows: 2,
			check: func(t *testing.T, rows []webHourlyTopRow) {
				if rows[0].Rank != 1 || rows[1].Rank != 2 {
					t.Errorf("fallback ranks = %d,%d, want 1,2", rows[0].Rank, rows[1].Rank)
				}
			},
		},
		{
			name:     "rows sorted chart-order then rank",
			raw:      `{"data": [{"top_paid": {"rank": 2}, "top_free": {"rank": 5}}, {"top_paid": {"rank": 1}}]}`,
			wantRows: 3,
			check: func(t *testing.T, rows []webHourlyTopRow) {
				if rows[0].Chart != "free" || rows[1].Chart != "paid" || rows[1].Rank != 1 || rows[2].Rank != 2 {
					t.Errorf("sort order wrong: %+v", rows)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, rows, err := parseHourlyTops(json.RawMessage(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rows) != tc.wantRows {
				t.Fatalf("got %d rows, want %d: %+v", len(rows), tc.wantRows, rows)
			}
			if tc.check != nil {
				tc.check(t, rows)
			}
		})
	}
}

func TestValidateHourlyStore(t *testing.T) {
	tests := []struct {
		store   int
		wantErr bool
	}{
		{1, false},
		{2, false},
		{3, false},
		{0, true},
		{4, true},
		{5, true},
		{-1, true},
	}
	for _, tc := range tests {
		err := validateHourlyStore(tc.store)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateHourlyStore(%d) error = %v, wantErr %v", tc.store, err, tc.wantErr)
		}
	}
}
