// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"math"
	"testing"
)

func TestNovelRetentionBenchmarkMedian(t *testing.T) {
	tests := []struct {
		name   string
		vals   []float64
		want   float64
		wantOK bool
	}{
		{name: "empty has no median", vals: nil, wantOK: false},
		{name: "single value", vals: []float64{0.42}, want: 0.42, wantOK: true},
		{name: "odd count picks the middle", vals: []float64{0.5, 0.1, 0.3}, want: 0.3, wantOK: true},
		{name: "even count averages the two middles", vals: []float64{0.4, 0.1, 0.2, 0.3}, want: 0.25, wantOK: true},
		{name: "unsorted input is sorted first", vals: []float64{9, 1, 5}, want: 5, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := rbMedian(tt.vals)
			if ok != tt.wantOK {
				t.Fatalf("rbMedian(%v) ok = %v, want %v", tt.vals, ok, tt.wantOK)
			}
			if ok && math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("rbMedian(%v) = %v, want %v", tt.vals, got, tt.want)
			}
		})
	}

	// rbMedian must not mutate the caller's slice (it sorts a copy).
	vals := []float64{3, 1, 2}
	_, _ = rbMedian(vals)
	if vals[0] != 3 || vals[1] != 1 || vals[2] != 2 {
		t.Errorf("rbMedian mutated its input: %v", vals)
	}
}

func TestNovelRetentionBenchmarkVerdict(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	tests := []struct {
		name   string
		app    *float64
		median *float64
		want   string
	}{
		{name: "clearly above", app: f(0.50), median: f(0.40), want: "above median"},
		{name: "clearly below", app: f(0.30), median: f(0.40), want: "below median"},
		{name: "exactly the median", app: f(0.40), median: f(0.40), want: "near median"},
		{name: "upper band edge inclusive", app: f(0.44), median: f(0.40), want: "near median"},
		{name: "lower band edge inclusive", app: f(0.36), median: f(0.40), want: "near median"},
		{name: "just past the upper band", app: f(0.4401), median: f(0.40), want: "above median"},
		{name: "just past the lower band", app: f(0.3599), median: f(0.40), want: "below median"},
		{name: "missing app value", app: nil, median: f(0.40), want: "unknown"},
		{name: "missing median", app: f(0.40), median: nil, want: "unknown"},
		{name: "both missing", app: nil, median: nil, want: "unknown"},
		{name: "zero median, positive app", app: f(0.01), median: f(0), want: "above median"},
		{name: "zero median, zero app", app: f(0), median: f(0), want: "near median"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rbVerdict(tt.app, tt.median); got != tt.want {
				t.Errorf("rbVerdict(%v, %v) = %q, want %q", tt.app, tt.median, got, tt.want)
			}
		})
	}
}

func TestNovelRetentionBenchmarkRetentionExtraction(t *testing.T) {
	fixture := json.RawMessage(`{
		"retention_1":  [[1764547200, 0.41], [1767139200, 0.43]],
		"retention_7":  [[1764547200, 0.12], [1767139200, null]],
		"retention_30": [],
		"retention_90": [[1764547200, 0.02]]
	}`)
	days := rbExtractRetentionDays(fixture)
	if days.D1 == nil || math.Abs(*days.D1-0.43) > 1e-9 {
		t.Errorf("D1 = %v, want latest point 0.43", days.D1)
	}
	// The latest D7 point is null; extraction must fall back to the previous
	// non-null point instead of returning null or zero.
	if days.D7 == nil || math.Abs(*days.D7-0.12) > 1e-9 {
		t.Errorf("D7 = %v, want 0.12 (latest non-null point)", days.D7)
	}
	if days.D30 != nil {
		t.Errorf("D30 = %v, want nil for an empty series", days.D30)
	}

	// Malformed payloads degrade to all-nil, never panic.
	for _, raw := range []string{`"oops"`, `[]`, `{"retention_1": "bad"}`, `{}`} {
		got := rbExtractRetentionDays(json.RawMessage(raw))
		if got.D1 != nil || got.D7 != nil || got.D30 != nil {
			t.Errorf("rbExtractRetentionDays(%s) = %+v, want all nil", raw, got)
		}
	}

	// Single-element points are tolerated as bare values (spec drift).
	bare := rbExtractRetentionDays(json.RawMessage(`{"retention_1": [[0.5]]}`))
	if bare.D1 == nil || math.Abs(*bare.D1-0.5) > 1e-9 {
		t.Errorf("single-element point: D1 = %v, want 0.5", bare.D1)
	}
}

func TestNovelRetentionBenchmarkFirstStoreID(t *testing.T) {
	ids := []string{"com.dreamgames.royalmatch", "1482155847"}

	if got, ok := rbFirstStoreID(ids, 1); !ok || got != "com.dreamgames.royalmatch" {
		t.Errorf("store 1 = %q ok=%v, want the Google Play package name", got, ok)
	}
	if got, ok := rbFirstStoreID(ids, 2); !ok || got != "1482155847" {
		t.Errorf("store 2 = %q ok=%v, want the numeric App Store id", got, ok)
	}
	if got, ok := rbFirstStoreID(ids, 3); !ok || got != "1482155847" {
		t.Errorf("store 3 = %q ok=%v, want the numeric App Store id", got, ok)
	}
	if got, ok := rbFirstStoreID([]string{"com.king.candycrushsaga"}, 2); ok {
		t.Errorf("store 2 with only a package name = %q ok=%v, want not found", got, ok)
	}
	if _, ok := rbFirstStoreID(nil, 1); ok {
		t.Errorf("empty id list must report not found")
	}

	// /tops/united-applications row parsing keeps rank order and drops
	// zero-id records.
	raw := json.RawMessage(`[
		{"place": 1, "united_application_id": 6346813, "value": 900},
		{"place": 2, "united_application_id": 1133361, "value": 800},
		{"place": 3, "value": 700}
	]`)
	got := rbParseTopRows(raw)
	if len(got) != 2 || got[0] != 6346813 || got[1] != 1133361 {
		t.Errorf("rbParseTopRows = %v, want [6346813 1133361]", got)
	}
	if rows := rbParseTopRows(json.RawMessage(`{"data": []}`)); rows != nil {
		t.Errorf("rbParseTopRows(non-array) = %v, want nil", rows)
	}
}
