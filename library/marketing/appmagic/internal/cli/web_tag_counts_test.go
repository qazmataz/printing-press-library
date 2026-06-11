// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseTagAppCounts(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []webTagCountRow
		wantErr bool
	}{
		{
			name: "rows sorted by count desc with tag-id ties ascending",
			raw:  `{"data": [{"tags": [5], "count": 120}, {"tags": [9], "count": 300}, {"tags": [2], "count": 120}]}`,
			want: []webTagCountRow{
				{TagID: 9, Count: 300},
				{TagID: 2, Count: 120},
				{TagID: 5, Count: 120},
			},
		},
		{
			name: "empty tags rows are skipped",
			raw:  `{"data": [{"tags": [], "count": 7}, {"tags": [3], "count": 11}, {"count": 9}]}`,
			want: []webTagCountRow{{TagID: 3, Count: 11}},
		},
		{
			name: "string-coded tag ids are coerced",
			raw:  `{"data": [{"tags": ["42"], "count": 6}, {"tags": ["not-a-number"], "count": 3}]}`,
			want: []webTagCountRow{{TagID: 42, Count: 6}},
		},
		{
			name: "multi-tag combo rows keep the first id",
			raw:  `{"data": [{"tags": [7, 8], "count": 4}]}`,
			want: []webTagCountRow{{TagID: 7, Count: 4}},
		},
		{
			name: "empty data renders empty non-nil slice",
			raw:  `{"data": []}`,
			want: []webTagCountRow{},
		},
		{
			name:    "array response is a typed error",
			raw:     `[{"tags": [1], "count": 2}]`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := parseTagAppCounts(json.RawMessage(tc.raw))
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

func TestApplyTagNames(t *testing.T) {
	t.Run("names applied where known, missing ids stay unnamed", func(t *testing.T) {
		rows := []webTagCountRow{
			{TagID: 1, Count: 10},
			{TagID: 2, Count: 5},
			{TagID: 3, Count: 1},
		}
		applyTagNames(rows, map[int64]string{1: "match-3", 3: "idle"})
		want := []webTagCountRow{
			{TagID: 1, TagName: "match-3", Count: 10},
			{TagID: 2, Count: 5},
			{TagID: 3, TagName: "idle", Count: 1},
		}
		if !reflect.DeepEqual(rows, want) {
			t.Errorf("rows mismatch:\n got: %+v\nwant: %+v", rows, want)
		}
	})
	t.Run("nil map is a no-op", func(t *testing.T) {
		rows := []webTagCountRow{{TagID: 1, Count: 10}}
		applyTagNames(rows, nil)
		if rows[0].TagName != "" {
			t.Errorf("TagName = %q, want empty after nil-map enrichment", rows[0].TagName)
		}
	})
	t.Run("empty name in map does not overwrite", func(t *testing.T) {
		rows := []webTagCountRow{{TagID: 4, Count: 2}}
		applyTagNames(rows, map[int64]string{4: ""})
		if rows[0].TagName != "" {
			t.Errorf("TagName = %q, want empty for blank taxonomy name", rows[0].TagName)
		}
	})
}
