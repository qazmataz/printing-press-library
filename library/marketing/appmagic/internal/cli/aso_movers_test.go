// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"testing"
)

const asoMoversByAppFixture = `{
  "tops": [
    {
      "appId": "com.dreamgames.royalmatch",
      "country": "US",
      "terms": [
        {"term": "match 3", "popularity": 62.5, "score": 41.2, "medianPlace": 3},
        {"term": "puzzle", "popularity": 80.1, "score": 12.9, "medianPlace": 15},
        {"term": "royal match", "popularity": 55.0, "score": 88.8, "medianPlace": 1},
        {"term": "match 3", "popularity": 1.0, "score": 1.0, "medianPlace": 99}
      ]
    }
  ]
}`

func TestAsoMoversParseTerms(t *testing.T) {
	terms, err := parseAsoMoversTerms(json.RawMessage(asoMoversByAppFixture))
	if err != nil {
		t.Fatalf("parseAsoMoversTerms: %v", err)
	}
	if len(terms) != 3 {
		t.Fatalf("expected 3 unique terms (duplicate 'match 3' keeps first), got %d: %v", len(terms), terms)
	}
	byTerm := map[string]asoTermPosition{}
	for _, tp := range terms {
		byTerm[tp.Term] = tp
	}
	if got := byTerm["match 3"]; got.Position != 3 || got.Score != 41.2 || got.Popularity != 62.5 {
		t.Errorf("match 3 = %+v, want position 3 / score 41.2 / popularity 62.5 (first occurrence wins)", got)
	}
	if got := byTerm["royal match"]; got.Position != 1 {
		t.Errorf("royal match position = %d, want 1", got.Position)
	}
	if _, err := parseAsoMoversTerms(json.RawMessage(`[1,2,3]`)); err == nil {
		t.Error("expected an error for a non-object response shape")
	}
	empty, err := parseAsoMoversTerms(json.RawMessage(`{}`))
	if err != nil || len(empty) != 0 {
		t.Errorf("empty object should parse to zero terms without error, got %v / %v", empty, err)
	}
}

func TestAsoMoversCompute(t *testing.T) {
	prev := []asoTermPosition{
		{Term: "match 3", Position: 10},
		{Term: "puzzle", Position: 5},
		{Term: "royal match", Position: 1},
		{Term: "dropped term", Position: 7},
		{Term: "also dropped", Position: 2},
	}
	curr := []asoTermPosition{
		{Term: "match 3", Position: 3},     // moved up, delta +7
		{Term: "puzzle", Position: 6},      // moved down, delta -1
		{Term: "royal match", Position: 1}, // unchanged
		{Term: "new term", Position: 4, Score: 9.5, Popularity: 70},
		{Term: "another new", Position: 20},
	}

	gained, lost, moved := asoComputeMovers(prev, curr, 25)

	if len(gained) != 2 || gained[0].Term != "new term" || gained[1].Term != "another new" {
		t.Fatalf("gained = %+v, want [new term, another new] sorted by best position", gained)
	}
	if gained[0].Score != 9.5 || gained[0].Popularity != 70 {
		t.Errorf("gained[0] should carry score/popularity, got %+v", gained[0])
	}
	if len(lost) != 2 || lost[0].Term != "also dropped" || lost[1].Term != "dropped term" {
		t.Fatalf("lost = %+v, want [also dropped, dropped term] sorted by best previous position", lost)
	}
	if len(moved) != 2 {
		t.Fatalf("moved = %+v, want exactly 2 entries (unchanged terms excluded)", moved)
	}
	if moved[0].Term != "match 3" || moved[0].FromPosition != 10 || moved[0].ToPosition != 3 || moved[0].Delta != 7 {
		t.Errorf("moved[0] = %+v, want match 3 from 10 to 3 with delta +7 (largest |delta| first)", moved[0])
	}
	if moved[1].Term != "puzzle" || moved[1].Delta != -1 {
		t.Errorf("moved[1] = %+v, want puzzle with delta -1", moved[1])
	}

	// No-change snapshots produce three empty (non-nil) buckets.
	gained, lost, moved = asoComputeMovers(prev, prev, 25)
	if gained == nil || lost == nil || moved == nil {
		t.Error("buckets must be non-nil empty slices so JSON renders [] not null")
	}
	if len(gained) != 0 || len(lost) != 0 || len(moved) != 0 {
		t.Errorf("identical snapshots: gained=%v lost=%v moved=%v, want all empty", gained, lost, moved)
	}

	// The limit caps every bucket.
	gained, lost, moved = asoComputeMovers(prev, curr, 1)
	if len(gained) != 1 || len(lost) != 1 || len(moved) != 1 {
		t.Errorf("limit 1: lens = %d/%d/%d, want 1/1/1", len(gained), len(lost), len(moved))
	}
	if moved[0].Term != "match 3" {
		t.Errorf("limit must keep the largest |delta| first, got %q", moved[0].Term)
	}
}

func TestAsoMoversStoreIDDetection(t *testing.T) {
	tests := []struct {
		arg  string
		want bool
	}{
		{"com.dreamgames.royalmatch", true}, // reversed-domain Google Play id
		{"1482155847", true},                // numeric App Store id
		{"Royal Match", false},              // app name with space
		{"TikTok", false},                   // plain name
		{"", false},
		{"match3", false}, // alphanumeric name, no dot
	}
	for _, tc := range tests {
		if got := asoMoversLooksLikeStoreID(tc.arg); got != tc.want {
			t.Errorf("asoMoversLooksLikeStoreID(%q) = %v, want %v", tc.arg, got, tc.want)
		}
	}

	app := &unitedApp{
		ID:                  42,
		Name:                "Royal Match",
		StoreApplicationIDs: []string{"1482155847", "com.dreamgames.royalmatch"},
	}
	id, err := asoMoversPickStoreID(app, 1)
	if err != nil || id != "com.dreamgames.royalmatch" {
		t.Errorf("store 1 pick = %q (%v), want the reversed-domain id", id, err)
	}
	id, err = asoMoversPickStoreID(app, 2)
	if err != nil || id != "1482155847" {
		t.Errorf("store 2 pick = %q (%v), want the numeric App Store id", id, err)
	}
	if _, err := asoMoversPickStoreID(&unitedApp{Name: "No IDs"}, 1); err == nil {
		t.Error("expected an error when the app has no matching store id")
	}
}
