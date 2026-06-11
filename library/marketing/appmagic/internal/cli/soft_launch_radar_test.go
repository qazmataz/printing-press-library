// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSoftLaunchMergeSightingDates(t *testing.T) {
	tests := []struct {
		name              string
		existingFirstSeen string
		today             string
		wantFirst         string
		wantLast          string
	}{
		{
			name:              "new sighting gets today as first and last seen",
			existingFirstSeen: "",
			today:             "2026-06-10",
			wantFirst:         "2026-06-10",
			wantLast:          "2026-06-10",
		},
		{
			name:              "existing sighting keeps original first seen",
			existingFirstSeen: "2026-05-01",
			today:             "2026-06-10",
			wantFirst:         "2026-05-01",
			wantLast:          "2026-06-10",
		},
		{
			name:              "whitespace-only existing first seen counts as new",
			existingFirstSeen: "   ",
			today:             "2026-06-10",
			wantFirst:         "2026-06-10",
			wantLast:          "2026-06-10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, last := mergeSightingDates(tt.existingFirstSeen, tt.today)
			if first != tt.wantFirst || last != tt.wantLast {
				t.Errorf("mergeSightingDates(%q, %q) = (%q, %q), want (%q, %q)",
					tt.existingFirstSeen, tt.today, first, last, tt.wantFirst, tt.wantLast)
			}
		})
	}
}

func TestSoftLaunchFilterSightings(t *testing.T) {
	rows := []softLaunchSighting{
		{StoreApplicationID: "com.dreamgames.royalkingdom", PublisherName: "Dream Games", Country: "PH", FirstSeen: "2026-06-08"},
		{StoreApplicationID: "com.playrix.fishdom2", PublisherName: "Playrix", Country: "CA", FirstSeen: "2026-06-09"},
		{StoreApplicationID: "com.king.match.next", PublisherName: "King", Country: "AU", FirstSeen: "2026-05-01"},
		{StoreApplicationID: "1234567890", PublisherName: "Dream Games", Country: "CA", FirstSeen: "2026-06-09"},
	}

	t.Run("since window excludes old sightings", func(t *testing.T) {
		got := filterSightings(rows, "2026-06-01", "", 0)
		if len(got) != 3 {
			t.Fatalf("filtered %d rows, want 3 (2026-05-01 sighting is outside the window)", len(got))
		}
		for _, r := range got {
			if r.FirstSeen < "2026-06-01" {
				t.Errorf("row %s first seen %s is before cutoff", r.StoreApplicationID, r.FirstSeen)
			}
		}
	})

	t.Run("ordered first_seen desc with deterministic tie-break", func(t *testing.T) {
		got := filterSightings(rows, "2026-06-01", "", 0)
		wantOrder := []string{"1234567890", "com.playrix.fishdom2", "com.dreamgames.royalkingdom"}
		for i, id := range wantOrder {
			if got[i].StoreApplicationID != id {
				t.Errorf("order[%d] = %s, want %s", i, got[i].StoreApplicationID, id)
			}
		}
	})

	t.Run("publisher substring filter is case-insensitive", func(t *testing.T) {
		got := filterSightings(rows, "2026-06-01", "dream", 0)
		if len(got) != 2 {
			t.Fatalf("publisher filter matched %d rows, want 2", len(got))
		}
		for _, r := range got {
			if r.PublisherName != "Dream Games" {
				t.Errorf("unexpected publisher %q in filtered rows", r.PublisherName)
			}
		}
	})

	t.Run("limit caps the result", func(t *testing.T) {
		got := filterSightings(rows, "2026-01-01", "", 2)
		if len(got) != 2 {
			t.Fatalf("limited to %d rows, want 2", len(got))
		}
	})

	t.Run("empty input yields empty non-nil slice", func(t *testing.T) {
		got := filterSightings(nil, "2026-01-01", "", 10)
		if got == nil || len(got) != 0 {
			t.Fatalf("got %v, want empty non-nil slice", got)
		}
	})
}

func TestSoftLaunchParseRows(t *testing.T) {
	fixture := json.RawMessage(`[
		{"store": 1, "store_application_id": "com.dreamgames.royalkingdom", "release_date": "2026-04-20",
		 "current_app_downloads": 152000, "current_app_revenue": 84000,
		 "publisher_apps_count": 4, "publisher_countries_list": ["TR"]},
		{"store": 2, "store_application_id": "6451290400", "release_date": "2026-05-30",
		 "current_app_downloads": 9100, "current_app_revenue": 0},
		{"store": 1, "release_date": "2026-05-01", "current_app_downloads": 5}
	]`)
	rows := parseSoftLaunchRows(fixture)
	if len(rows) != 2 {
		t.Fatalf("parsed %d rows, want 2 (row without store_application_id must be skipped)", len(rows))
	}
	if rows[0].Store != 1 || rows[0].StoreApplicationID != "com.dreamgames.royalkingdom" ||
		rows[0].ReleaseDate != "2026-04-20" || rows[0].CurrentAppDownloads != 152000 {
		t.Errorf("row[0] = %+v, want store=1 id=com.dreamgames.royalkingdom release=2026-04-20 downloads=152000", rows[0])
	}
	if rows[1].Store != 2 || rows[1].StoreApplicationID != "6451290400" {
		t.Errorf("row[1] = %+v, want store=2 id=6451290400", rows[1])
	}
	if !strings.Contains(string(rows[0].Raw), "current_app_downloads") {
		t.Errorf("row[0].Raw must preserve the original JSON element, got %s", rows[0].Raw)
	}

	if got := parseSoftLaunchRows(json.RawMessage(`{"data":[{"store":1,"store_application_id":"x"}]}`)); len(got) != 1 {
		t.Errorf("envelope parse got %d rows, want 1", len(got))
	}
	if got := parseSoftLaunchRows(json.RawMessage(`null`)); len(got) != 0 {
		t.Errorf("null body should parse to zero rows, got %d", len(got))
	}
}

func TestSoftLaunchParseIdentities(t *testing.T) {
	fixture := json.RawMessage(`[
		{"store": 1, "store_application_id": "com.dreamgames.royalkingdom", "name": "Royal Kingdom", "publisher_name": "Dream Games"},
		{"store": 2, "store_application_id": "6451290400", "name": "Capybara Go!"},
		{"store": 1, "name": "No ID App"}
	]`)
	idents := parseSoftLaunchIdentities(fixture)
	if len(idents) != 2 {
		t.Fatalf("parsed %d identities, want 2", len(idents))
	}
	got, ok := idents[softLaunchIdentityKey(1, "com.dreamgames.royalkingdom")]
	if !ok || got.Name != "Royal Kingdom" || got.PublisherName != "Dream Games" {
		t.Errorf("identity for store 1 = %+v (found=%v), want Royal Kingdom / Dream Games", got, ok)
	}
	if _, ok := idents[softLaunchIdentityKey(2, "6451290400")]; !ok {
		t.Errorf("identity for store 2 id 6451290400 missing")
	}
}
