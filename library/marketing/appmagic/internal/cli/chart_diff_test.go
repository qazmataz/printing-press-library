// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChartDiffSnapshots(t *testing.T) {
	prev := []chartSnapshotRow{
		{Rank: 1, ID: "100", Name: "Royal Match"},
		{Rank: 2, ID: "200", Name: "TikTok"},
		{Rank: 3, ID: "300", Name: "Gardenscapes"},
		{Rank: 4, ID: "400", Name: "Candy Crush Saga"},
	}

	tests := []struct {
		name        string
		prev, curr  []chartSnapshotRow
		maxMovers   int
		wantEntered []string
		wantDropped []string
		wantMoved   []chartDiffMove
	}{
		{
			name: "entered dropped and moved",
			prev: prev,
			curr: []chartSnapshotRow{
				{Rank: 1, ID: "200", Name: "TikTok"},
				{Rank: 2, ID: "100", Name: "Royal Match"},
				{Rank: 3, ID: "500", Name: "Whiteout Survival"},
				{Rank: 4, ID: "400", Name: "Candy Crush Saga"},
			},
			maxMovers:   100,
			wantEntered: []string{"500"},
			wantDropped: []string{"300"},
			wantMoved: []chartDiffMove{
				{UnitedApplicationID: "200", Name: "TikTok", FromRank: 2, ToRank: 1, Delta: 1},
				{UnitedApplicationID: "100", Name: "Royal Match", FromRank: 1, ToRank: 2, Delta: -1},
			},
		},
		{
			name:        "zero diff",
			prev:        prev,
			curr:        prev,
			maxMovers:   100,
			wantEntered: []string{},
			wantDropped: []string{},
			wantMoved:   []chartDiffMove{},
		},
		{
			name: "movers sorted by absolute delta and capped",
			prev: prev,
			curr: []chartSnapshotRow{
				{Rank: 1, ID: "400", Name: "Candy Crush Saga"}, // delta +3
				{Rank: 2, ID: "200", Name: "TikTok"},           // delta 0
				{Rank: 3, ID: "100", Name: "Royal Match"},      // delta -2
				{Rank: 4, ID: "300", Name: "Gardenscapes"},     // delta -1
			},
			maxMovers:   2,
			wantEntered: []string{},
			wantDropped: []string{},
			wantMoved: []chartDiffMove{
				{UnitedApplicationID: "400", Name: "Candy Crush Saga", FromRank: 4, ToRank: 1, Delta: 3},
				{UnitedApplicationID: "100", Name: "Royal Match", FromRank: 1, ToRank: 3, Delta: -2},
			},
		},
		{
			name:        "everything entered against empty previous",
			prev:        []chartSnapshotRow{},
			curr:        prev,
			maxMovers:   100,
			wantEntered: []string{"100", "200", "300", "400"},
			wantDropped: []string{},
			wantMoved:   []chartDiffMove{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered, dropped, moved := diffChartSnapshots(tt.prev, tt.curr, tt.maxMovers)
			if entered == nil || dropped == nil || moved == nil {
				t.Fatalf("diff slices must be non-nil so JSON renders [] not null")
			}
			gotEntered := make([]string, 0, len(entered))
			for _, e := range entered {
				gotEntered = append(gotEntered, e.UnitedApplicationID)
			}
			if strings.Join(gotEntered, ",") != strings.Join(tt.wantEntered, ",") {
				t.Errorf("entered = %v, want %v", gotEntered, tt.wantEntered)
			}
			gotDropped := make([]string, 0, len(dropped))
			for _, d := range dropped {
				gotDropped = append(gotDropped, d.UnitedApplicationID)
			}
			if strings.Join(gotDropped, ",") != strings.Join(tt.wantDropped, ",") {
				t.Errorf("dropped = %v, want %v", gotDropped, tt.wantDropped)
			}
			if len(moved) != len(tt.wantMoved) {
				t.Fatalf("moved = %+v, want %+v", moved, tt.wantMoved)
			}
			for i, w := range tt.wantMoved {
				if moved[i] != w {
					t.Errorf("moved[%d] = %+v, want %+v", i, moved[i], w)
				}
			}
		})
	}
}

func TestChartDiffParseTopRows(t *testing.T) {
	fixture := json.RawMessage(`[
		{"place": 1, "united_application_id": 6242710, "value": 1520000},
		{"place": 2, "united_application_id": 1136468, "value": 990000},
		{"united_application_id": "777", "value": 12},
		{"place": 4, "value": 5}
	]`)
	rows := parseChartTopRows(fixture)
	if len(rows) != 3 {
		t.Fatalf("parsed %d rows, want 3 (row without united_application_id must be skipped)", len(rows))
	}
	if rows[0].Place != 1 || rows[0].UnitedApplicationID != "6242710" || rows[0].Value != 1520000 {
		t.Errorf("row[0] = %+v, want place=1 id=6242710 value=1520000", rows[0])
	}
	if rows[1].UnitedApplicationID != "1136468" {
		t.Errorf("row[1].UnitedApplicationID = %q, want 1136468", rows[1].UnitedApplicationID)
	}
	// String id and missing place fall back to position-based rank.
	if rows[2].UnitedApplicationID != "777" || rows[2].Place != 3 {
		t.Errorf("row[2] = %+v, want id=777 place=3", rows[2])
	}
	if len(rows[0].Raw) == 0 {
		t.Errorf("row[0].Raw must preserve the original JSON element")
	}

	// Envelope form degrades gracefully instead of failing to parse.
	envelope := json.RawMessage(`{"data":[{"place":1,"united_application_id":42,"value":7}]}`)
	envRows := parseChartTopRows(envelope)
	if len(envRows) != 1 || envRows[0].UnitedApplicationID != "42" {
		t.Errorf("envelope parse = %+v, want one row with id 42", envRows)
	}

	if got := parseChartTopRows(json.RawMessage(`not json`)); len(got) != 0 {
		t.Errorf("invalid JSON should parse to zero rows, got %d", len(got))
	}
}

func TestChartDiffInsufficientNote(t *testing.T) {
	note := chartDiffInsufficientNote("free", 5, "WW", 1)
	for _, want := range []string{"only 1 snapshot", "chart-diff --sort free --store 5 --country WW", "tomorrow"} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q must contain %q", note, want)
		}
	}
	zero := chartDiffInsufficientNote("grossing", 1, "US", 0)
	if !strings.Contains(zero, "only 0 snapshot") || !strings.Contains(zero, "--sort grossing") {
		t.Errorf("zero-snapshot note %q must name the count and the sort", zero)
	}
}
