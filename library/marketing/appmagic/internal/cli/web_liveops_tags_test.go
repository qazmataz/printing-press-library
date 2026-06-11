// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestParseLiveopsTagCounts(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []webLiveopsTagRow
		wantErr bool
	}{
		{
			name: "flat dict sorted by count desc with alphabetical ties",
			raw:  `{"Tournament": 42, "Battle Pass": 97, "Collection": 42, "Sale": 5}`,
			want: []webLiveopsTagRow{
				{Tag: "Battle Pass", Count: 97},
				{Tag: "Collection", Count: 42},
				{Tag: "Tournament", Count: 42},
				{Tag: "Sale", Count: 5},
			},
		},
		{
			name: "non-numeric values are skipped",
			raw:  `{"Tournament": 10, "meta": "ignored", "nested": {"x": 1}}`,
			want: []webLiveopsTagRow{{Tag: "Tournament", Count: 10}},
		},
		{
			name: "data envelope fallback",
			raw:  `{"data": {"Sale": 3, "Gacha": 8}}`,
			want: []webLiveopsTagRow{{Tag: "Gacha", Count: 8}, {Tag: "Sale", Count: 3}},
		},
		{
			name: "empty dict renders empty non-nil slice",
			raw:  `{}`,
			want: []webLiveopsTagRow{},
		},
		{
			name:    "array response is a typed error",
			raw:     `[{"tag": "x"}]`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := parseLiveopsTagCounts(json.RawMessage(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rows == nil {
				t.Fatal("rows is nil, want non-nil slice so JSON renders []")
			}
			if !reflect.DeepEqual(rows, tc.want) {
				t.Errorf("rows mismatch:\n got: %+v\nwant: %+v", rows, tc.want)
			}
		})
	}
}

func TestLiveopsTagsWindow(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		since    string
		wantFrom string
		wantTo   string
		wantErr  bool
	}{
		{name: "30d default-style window", since: "30d", wantFrom: "2026-05-11", wantTo: "2026-06-10"},
		{name: "weeks notation", since: "1w", wantFrom: "2026-06-03", wantTo: "2026-06-10"},
		{name: "hours notation", since: "24h", wantFrom: "2026-06-09", wantTo: "2026-06-10"},
		{name: "garbage duration is an error", since: "soonish", wantErr: true},
		{name: "empty duration is an error", since: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			from, to, err := liveopsTagsWindow(tc.since, now)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if from != tc.wantFrom || to != tc.wantTo {
				t.Errorf("window = (%s, %s), want (%s, %s)", from, to, tc.wantFrom, tc.wantTo)
			}
		})
	}
}
