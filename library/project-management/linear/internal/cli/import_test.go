package cli

import (
	"reflect"
	"strings"
	"testing"
)

// TestImportableResources documents that issues is the only importable
// resource today, and that the dispatch is a membership check (not a hardcoded
// string compare scattered through the command).
func TestImportableResources(t *testing.T) {
	if !importableResources["issues"] {
		t.Fatal("expected issues to be importable")
	}
	for _, r := range []string{"projects", "cycles", "comments", "users", ""} {
		if importableResources[r] {
			t.Errorf("resource %q should not be importable", r)
		}
	}
}

// TestUnsupportedImportError checks the error names the requested resource and
// points the user at a working alternative, instead of surfacing a raw GraphQL
// 400.
func TestUnsupportedImportError(t *testing.T) {
	err := unsupportedImportError("projects")
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{"projects", "issues create", "GraphQL"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q\n got: %s", want, msg)
		}
	}
	// Must not blame the user's JSON / suggest a REST retry.
	if strings.Contains(msg, "400") {
		t.Errorf("error should not surface a raw HTTP 400: %s", msg)
	}
}

// TestIssueRecordToInput pins JSONL-record normalization: the "team" alias maps
// to "teamId", an explicit "teamId" wins, and other fields pass through
// untouched. This is the import-side mirror of issueInputFromFlags.
func TestIssueRecordToInput(t *testing.T) {
	tests := []struct {
		name   string
		record map[string]any
		want   map[string]any
	}{
		{
			name:   "team alias maps to teamId",
			record: map[string]any{"title": "x", "team": "ENG"},
			want:   map[string]any{"title": "x", "teamId": "ENG"},
		},
		{
			name:   "explicit teamId wins over team alias",
			record: map[string]any{"title": "x", "teamId": "uuid-1", "team": "ENG"},
			want:   map[string]any{"title": "x", "teamId": "uuid-1"},
		},
		{
			name:   "full record passes through",
			record: map[string]any{"title": "x", "teamId": "uuid", "description": "d", "priority": float64(2), "labelIds": []any{"l1"}},
			want:   map[string]any{"title": "x", "teamId": "uuid", "description": "d", "priority": float64(2), "labelIds": []any{"l1"}},
		},
		{
			name:   "no team key at all is left as-is",
			record: map[string]any{"title": "x"},
			want:   map[string]any{"title": "x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := issueRecordToInput(tt.record)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("issueRecordToInput mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
			// Must not mutate the caller's record map.
			if _, leaked := tt.record["teamId"]; leaked && tt.record["title"] == "x" && tt.name == "team alias maps to teamId" {
				t.Errorf("issueRecordToInput mutated the input record: %#v", tt.record)
			}
		})
	}
}
