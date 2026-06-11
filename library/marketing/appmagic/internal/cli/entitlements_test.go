// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEntitlementVerdictClassification(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		wantVerdict string
	}{
		{"200 data", 200, verdictIncluded},
		{"201 created-ish 2xx", 201, verdictIncluded},
		{"400 validation reached", 400, verdictIncluded},
		{"422 validation reached", 422, verdictIncluded},
		{"404 routed past auth", 404, verdictIncluded},
		{"401 bad credentials", 401, verdictAuthRequired},
		{"403 contract gate", 403, verdictNotIncluded},
		{"429 throttled", 429, verdictRateLimited},
		{"0 network failure", 0, verdictError},
		{"500 server error", 500, verdictUnknown},
		{"418 unexpected", 418, verdictUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verdict, detail := entitlementVerdict(tc.status)
			if verdict != tc.wantVerdict {
				t.Fatalf("entitlementVerdict(%d) = %q, want %q", tc.status, verdict, tc.wantVerdict)
			}
			if detail == "" {
				t.Fatalf("entitlementVerdict(%d) returned empty detail", tc.status)
			}
		})
	}
}

// fixtureEntitlementRows decodes cache rows from fixture JSON, mirroring the
// persisted entitlement_probes shape.
func fixtureEntitlementRows(t *testing.T, fixture string) map[string]entitlementProbeRow {
	t.Helper()
	var rows []entitlementProbeRow
	if err := json.Unmarshal([]byte(fixture), &rows); err != nil {
		t.Fatalf("decoding fixture rows: %v", err)
	}
	out := map[string]entitlementProbeRow{}
	for _, r := range rows {
		out[r.Group] = r
	}
	return out
}

func TestEntitlementCacheFreshness(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-24 * time.Hour).Format(time.RFC3339)
	almostStale := now.Add(-entitlementCacheMaxAge + time.Minute).Format(time.RFC3339)
	stale := now.Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	exactBoundary := now.Add(-entitlementCacheMaxAge).Format(time.RFC3339)

	fixture := `[
		{"group":"tops","probe_method":"GET","probe_path":"/tops/united-applications","http_status":200,"verdict":"included","checked_at":"` + fresh + `"},
		{"group":"tags","probe_method":"GET","probe_path":"/tags","http_status":200,"verdict":"included","checked_at":"` + almostStale + `"},
		{"group":"contacts","probe_method":"GET","probe_path":"/contacts/companies/1","http_status":403,"verdict":"not-included","checked_at":"` + stale + `"},
		{"group":"asa","probe_method":"GET","probe_path":"/asa/dates","http_status":200,"verdict":"included","checked_at":"` + exactBoundary + `"},
		{"group":"aso","probe_method":"GET","probe_path":"/aso/dates","http_status":200,"verdict":"included","checked_at":"not-a-timestamp"}
	]`
	rows := fixtureEntitlementRows(t, fixture)

	cases := []struct {
		name   string
		groups []string
		want   bool
	}{
		{"all fresh", []string{"tops", "tags"}, true},
		{"single fresh", []string{"tops"}, true},
		{"stale row forces probe", []string{"tops", "contacts"}, false},
		{"exact max-age boundary is stale", []string{"asa"}, false},
		{"missing group forces probe", []string{"tops", "history"}, false},
		{"unparsable timestamp forces probe", []string{"aso"}, false},
		{"empty selection never serves cache", []string{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entitlementCacheUsable(rows, tc.groups, now); got != tc.want {
				t.Fatalf("entitlementCacheUsable(%v) = %v, want %v", tc.groups, got, tc.want)
			}
		})
	}
}

func TestEntitlementParseGroups(t *testing.T) {
	probes := entitlementProbes("2026-06-07")

	t.Run("empty selects all in canonical order", func(t *testing.T) {
		got, err := parseEntitlementGroups("", probes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != len(probes) {
			t.Fatalf("got %d groups, want %d", len(got), len(probes))
		}
		if got[0] != "applications" || got[len(got)-1] != "last-date" {
			t.Fatalf("canonical order broken: first=%q last=%q", got[0], got[len(got)-1])
		}
	})

	t.Run("subset returned in canonical order regardless of CSV order", func(t *testing.T) {
		got, err := parseEntitlementGroups("history,tops", probes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"tops", "history"}
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("normalization and aliases", func(t *testing.T) {
		got, err := parseEntitlementGroups(" Ad Intelligence , last date, United_Applications ", probes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"united-applications", "adint", "last-date"}
		if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("unknown group errors and names valid groups", func(t *testing.T) {
		_, err := parseEntitlementGroups("tops,nonsense", probes)
		if err == nil {
			t.Fatal("expected error for unknown group")
		}
		if !strings.Contains(err.Error(), "nonsense") || !strings.Contains(err.Error(), "tops") {
			t.Fatalf("error should name the bad token and valid groups, got: %v", err)
		}
	})
}

func TestEntitlementSummaryAndCacheView(t *testing.T) {
	groups := []entitlementGroupView{
		{Group: "tops", Verdict: verdictIncluded, HTTPStatus: 200},
		{Group: "history", Verdict: verdictIncluded, HTTPStatus: 422},
		{Group: "contacts", Verdict: verdictNotIncluded, HTTPStatus: 403},
		{Group: "charts", Verdict: verdictPostOnly, HTTPStatus: 0},
		{Group: "sdkint", Verdict: verdictAuthRequired, HTTPStatus: 401},
		{Group: "asa", Verdict: verdictError, HTTPStatus: 0},
	}
	sum := entitlementSummarize(groups)
	if sum.Included != 2 || sum.NotIncluded != 1 || sum.Unknown != 3 {
		t.Fatalf("summary = %+v, want included=2 not_included=1 unknown=3", sum)
	}

	fixture := `[
		{"group":"tops","probe_method":"GET","probe_path":"/tops/united-applications","http_status":200,"verdict":"included","detail":"HTTP 200: probe returned data","checked_at":"2026-06-09T10:00:00Z"},
		{"group":"contacts","probe_method":"GET","probe_path":"/contacts/companies/1","http_status":403,"verdict":"not-included","detail":"HTTP 403","checked_at":"2026-06-08T10:00:00Z"}
	]`
	rows := fixtureEntitlementRows(t, fixture)
	view, missing := entitlementViewFromRows(rows, []string{"tops", "contacts", "history"})
	if len(view.Groups) != 2 {
		t.Fatalf("served %d groups, want 2", len(view.Groups))
	}
	if view.Groups[0].Group != "tops" || view.Groups[1].Group != "contacts" {
		t.Fatalf("group order broken: %+v", view.Groups)
	}
	if view.CheckedAt != "2026-06-08T10:00:00Z" {
		t.Fatalf("checked_at should be the oldest served row, got %q", view.CheckedAt)
	}
	if len(missing) != 1 || missing[0] != "history" {
		t.Fatalf("missing = %v, want [history]", missing)
	}
	if view.Summary.Included != 1 || view.Summary.NotIncluded != 1 || view.Summary.Unknown != 0 {
		t.Fatalf("cache view summary = %+v", view.Summary)
	}
}

func TestEntitlementDogfoodCurtailment(t *testing.T) {
	t.Run("intersection kept", func(t *testing.T) {
		got := dogfoodEntitlementGroups([]string{"tops", "tags", "categories"})
		if len(got) != 2 || got[0] != "tags" || got[1] != "categories" {
			t.Fatalf("got %v, want [tags categories]", got)
		}
	})
	t.Run("empty intersection falls back to full dogfood set", func(t *testing.T) {
		got := dogfoodEntitlementGroups([]string{"tops", "history"})
		if len(got) != 3 {
			t.Fatalf("got %v, want the 3 dogfood groups", got)
		}
		want := map[string]bool{"categories": true, "tags": true, "last-date": true}
		for _, g := range got {
			if !want[g] {
				t.Fatalf("unexpected dogfood group %q in %v", g, got)
			}
		}
	})
}

func TestEntitlementProbeMapShape(t *testing.T) {
	probes := entitlementProbes("2026-06-07")
	if len(probes) != 20 {
		t.Fatalf("probe map has %d groups, want 20", len(probes))
	}
	seen := map[string]bool{}
	for _, p := range probes {
		if seen[p.group] {
			t.Fatalf("duplicate group %q", p.group)
		}
		seen[p.group] = true
		if p.postOnly {
			if p.group != "charts" {
				t.Fatalf("only charts is post-only, found %q", p.group)
			}
			continue
		}
		if p.method != "GET" {
			t.Fatalf("probe for %q must be GET, got %q", p.group, p.method)
		}
		if !strings.HasPrefix(p.path, "/") {
			t.Fatalf("probe path for %q must be absolute, got %q", p.group, p.path)
		}
	}
	// Spec-required params must be present on the probes that need them.
	for _, p := range probes {
		switch p.group {
		case "tops":
			for _, k := range []string{"sort", "store", "country", "date"} {
				if p.params[k] == "" {
					t.Fatalf("tops probe missing required param %q", k)
				}
			}
			if p.params["date"] != "2026-06-07" {
				t.Fatalf("tops probe should use the supplied recent date, got %q", p.params["date"])
			}
		case "history":
			for _, k := range []string{"date", "store", "united_application_ids", "country"} {
				if p.params[k] == "" {
					t.Fatalf("history probe missing required param %q", k)
				}
			}
		case "sdkint":
			for _, k := range []string{"store", "store_application_id"} {
				if p.params[k] == "" {
					t.Fatalf("sdkint probe missing required param %q", k)
				}
			}
		case "featuring":
			for _, k := range []string{"store", "store_application_id", "country", "date"} {
				if p.params[k] == "" {
					t.Fatalf("featuring probe missing required param %q", k)
				}
			}
		}
	}
}
