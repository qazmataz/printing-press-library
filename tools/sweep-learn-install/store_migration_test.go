package main

import (
	"strings"
	"testing"
)

// preLearnStoreSnippet is a minimal store.go fragment carrying the
// post-U6 learn-migrations anchor. The sweep finds the anchor and
// rewrites the block between it and the FTS create.
const preLearnStoreSnippet = `package store

const StoreSchemaVersion = 1

func migrate() {
	migrations := []string{
		` + "`CREATE TABLE IF NOT EXISTS resources (id TEXT)`" + `,
		// CLI Printing Press: learn migrations
		` + "`CREATE TABLE IF NOT EXISTS search_learnings (old shape)`" + `,
		` + "`CREATE VIRTUAL TABLE IF NOT EXISTS search_learnings_fts USING fts5(query_pattern, tokenize='porter unicode61')`" + `,
	}
	_ = migrations
}
`

// preLearnNoAnchorSnippet is the shape a non-learn-enabled CLI carries
// before any retrofit. The sweep refuses to patch this (caller skips
// the CLI with "anchor not found").
const preLearnNoAnchorSnippet = `package store

const StoreSchemaVersion = 1

func migrate() {
	migrations := []string{
		` + "`CREATE TABLE IF NOT EXISTS resources (id TEXT)`" + `,
	}
	_ = migrations
}
`

func TestHasLearnMigrationAnchor(t *testing.T) {
	if !hasLearnMigrationAnchor([]byte(preLearnStoreSnippet)) {
		t.Error("expected anchor to be detected in pre-learn snippet")
	}
	if hasLearnMigrationAnchor([]byte(preLearnNoAnchorSnippet)) {
		t.Error("expected anchor absent in no-anchor snippet")
	}
}

func TestPatchStoreMigrations_RewritesBlockAndBumpsVersion(t *testing.T) {
	ctx := sweepCtx{CLIName: "demo-pp-cli", APIName: "demo"}
	got, changed, err := patchStoreMigrations(preLearnStoreSnippet, ctx)
	if err != nil {
		t.Fatalf("patchStoreMigrations: %v", err)
	}
	if !changed {
		t.Error("expected changed=true on first run")
	}
	if !strings.Contains(got, "search_patterns") {
		t.Errorf("canonical block missing search_patterns:\n%s", got)
	}
	if !strings.Contains(got, "entity_lookups") {
		t.Errorf("canonical block missing entity_lookups:\n%s", got)
	}
	if !strings.Contains(got, "teach_log_metadata") {
		t.Errorf("canonical block missing teach_log_metadata:\n%s", got)
	}
	if strings.Contains(got, "old shape") {
		t.Errorf("stale (old shape) content was not replaced:\n%s", got)
	}
	if !strings.Contains(got, "const StoreSchemaVersion = 3") {
		t.Errorf("StoreSchemaVersion not bumped to 3:\n%s", got)
	}
}

func TestPatchStoreMigrations_Idempotent(t *testing.T) {
	ctx := sweepCtx{CLIName: "demo-pp-cli", APIName: "demo"}
	first, _, err := patchStoreMigrations(preLearnStoreSnippet, ctx)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, changed, err := patchStoreMigrations(first, ctx)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if changed {
		t.Error("expected changed=false on idempotent re-run")
	}
	if second != first {
		t.Errorf("second run produced diff:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestPatchStoreMigrations_RefusesWithoutAnchor(t *testing.T) {
	ctx := sweepCtx{CLIName: "demo-pp-cli", APIName: "demo"}
	_, _, err := patchStoreMigrations(preLearnNoAnchorSnippet, ctx)
	if err == nil {
		t.Error("expected error when anchor missing")
	}
}

func TestBumpStoreSchemaVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"bumps-lower-version",
			"const StoreSchemaVersion = 1",
			"const StoreSchemaVersion = 3",
		},
		{
			"idempotent-at-target",
			"const StoreSchemaVersion = 3",
			"const StoreSchemaVersion = 3",
		},
		{
			"leaves-higher-alone",
			"const StoreSchemaVersion = 5",
			"const StoreSchemaVersion = 5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bumpStoreSchemaVersion(tc.in, 3)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
