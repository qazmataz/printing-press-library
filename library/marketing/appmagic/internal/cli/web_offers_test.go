// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseWebOffers(t *testing.T) {
	fixture := `{
		"priceRange": {"min": 0.99, "max": 99.99},
		"totalCount": 84,
		"data": [
			{
				"game_name": "Royal Match",
				"category": "Puzzle",
				"type": "starter_pack",
				"structure": "bundle",
				"duration": 3,
				"offers": [{"price": 4.99}],
				"offers_calendar": [{"date": "2026-06-01"}]
			},
			{
				"game_name": "Township",
				"category": "Casual",
				"type": "chain_offer",
				"structure": "chain"
			}
		]
	}`

	view, err := parseWebOffers(json.RawMessage(fixture))
	if err != nil {
		t.Fatalf("parseWebOffers returned error: %v", err)
	}
	if view.TotalCount != 84 {
		t.Errorf("TotalCount = %d, want 84", view.TotalCount)
	}
	if view.PriceRange == nil {
		t.Error("PriceRange dropped, want passthrough of the priceRange object")
	}
	if len(view.Offers) != 2 {
		t.Fatalf("got %d offers, want 2", len(view.Offers))
	}
	first := view.Offers[0]
	if first.GameName != "Royal Match" || first.Category != "Puzzle" || first.Type != "starter_pack" || first.Structure != "bundle" {
		t.Errorf("first offer fields wrong: %+v", first)
	}
	if d, ok := first.Duration.(float64); !ok || d != 3 {
		t.Errorf("first offer duration = %v, want 3", first.Duration)
	}
	if view.Offers[1].Duration != nil {
		t.Errorf("second offer duration = %v, want nil when absent", view.Offers[1].Duration)
	}
	// The capped view must not leak the verbose offers[]/offers_calendar[] arrays.
	out, err := json.Marshal(view.Offers[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var roundTrip map[string]any
	if err := json.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, banned := range []string{"offers", "offers_calendar"} {
		if _, found := roundTrip[banned]; found {
			t.Errorf("capped offer row leaked verbose field %q", banned)
		}
	}
}

func TestParseWebOffersEdgeCases(t *testing.T) {
	t.Run("empty data renders empty non-nil slice", func(t *testing.T) {
		view, err := parseWebOffers(json.RawMessage(`{"totalCount": 0, "data": []}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if view.Offers == nil {
			t.Error("Offers is nil, want empty non-nil slice so JSON renders []")
		}
		if len(view.Offers) != 0 {
			t.Errorf("got %d offers, want 0", len(view.Offers))
		}
	})
	t.Run("missing totalCount falls back to row count", func(t *testing.T) {
		view, err := parseWebOffers(json.RawMessage(`{"data": [{"game_name": "A"}, {"game_name": "B"}]}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if view.TotalCount != 2 {
			t.Errorf("TotalCount = %d, want fallback 2", view.TotalCount)
		}
	})
	t.Run("invalid JSON is a typed error", func(t *testing.T) {
		if _, err := parseWebOffers(json.RawMessage(`[1,2,3]`)); err == nil {
			t.Error("expected error for non-envelope response, got nil")
		}
	})
}

func TestWebOffersWindow(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		from, to string
		wantFrom string
		wantTo   string
		wantErr  bool
	}{
		{name: "defaults to last 30 days", wantFrom: "2026-05-11", wantTo: "2026-06-10"},
		{name: "explicit window passes through", from: "2026-05-01", to: "2026-06-01", wantFrom: "2026-05-01", wantTo: "2026-06-01"},
		{name: "explicit from with default to", from: "2026-06-01", wantFrom: "2026-06-01", wantTo: "2026-06-10"},
		{name: "invalid from is an error", from: "June 1", wantErr: true},
		{name: "invalid to is an error", to: "2026-13-45", wantErr: true},
		{name: "from after to is an error", from: "2026-06-09", to: "2026-06-01", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			from, to, err := webOffersWindow(tc.from, tc.to, now)
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
