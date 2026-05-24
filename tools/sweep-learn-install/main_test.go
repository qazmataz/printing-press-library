package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSweepCLI_SkipsMissingManifest exercises skip rule #1: a directory
// without .printing-press.json is skipped silently.
func TestSweepCLI_SkipsMissingManifest(t *testing.T) {
	dir := t.TempDir()
	status, err := sweepCLI(dir, sweepOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != statusSkipped {
		t.Errorf("expected statusSkipped; got %q", status)
	}
}

// TestSweepCLI_SkipsOptOutMarker exercises skip rule #2: a directory
// with .no-learn-sweep is skipped.
func TestSweepCLI_SkipsOptOutMarker(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".printing-press.json"),
		`{"api_name":"demo","cli_name":"demo-pp-cli"}`)
	writeFile(t, filepath.Join(dir, ".no-learn-sweep"), "")
	status, err := sweepCLI(dir, sweepOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != statusSkipped {
		t.Errorf("expected statusSkipped (opt-out); got %q", status)
	}
}

// TestSweepCLI_RefusesLegacyRootShape exercises skip rule #3: a CLI
// with the legacy `var rootCmd` shape is refused.
func TestSweepCLI_RefusesLegacyRootShape(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".printing-press.json"),
		`{"api_name":"demo","cli_name":"demo-pp-cli"}`)
	writeFile(t, filepath.Join(dir, "internal/cli/root.go"), legacyRootShape)

	status, err := sweepCLI(dir, sweepOpts{})
	if err == nil {
		t.Fatal("expected error for legacy shape")
	}
	if status != statusSkipped {
		t.Errorf("expected statusSkipped; got %q", status)
	}
	if !strings.Contains(err.Error(), "legacy var rootCmd shape") {
		t.Errorf("expected legacy-shape diagnostic; got %v", err)
	}
}

// TestSweepCLI_SkipsMissingAnchor exercises skip rule #4: a CLI whose
// store.go lacks the learn-migrations anchor is skipped with a
// manual-review diagnostic.
func TestSweepCLI_SkipsMissingAnchor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".printing-press.json"),
		`{"api_name":"demo","cli_name":"demo-pp-cli"}`)
	writeFile(t, filepath.Join(dir, "internal/cli/root.go"), canonicalRootFlagsShape)
	writeFile(t, filepath.Join(dir, "internal/store/store.go"), preLearnNoAnchorSnippet)

	status, err := sweepCLI(dir, sweepOpts{})
	if err == nil {
		t.Fatal("expected error for missing anchor")
	}
	if status != statusSkipped {
		t.Errorf("expected statusSkipped; got %q", status)
	}
	if !strings.Contains(err.Error(), "anchor") {
		t.Errorf("expected anchor diagnostic; got %v", err)
	}
}

// TestSweepCLI_IdempotentOnSecondRun runs the full sweep twice on the
// same fixture and asserts the second run reports statusUnchanged.
// This is the binding idempotency contract for the per-CLI pipeline.
func TestSweepCLI_IdempotentOnSecondRun(t *testing.T) {
	dir := stageMinimalCLIDir(t)

	// First run: should patch.
	st1, err := sweepCLI(dir, sweepOpts{})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if st1 != statusPatched {
		t.Errorf("first run expected patched; got %q", st1)
	}

	// Second run: should report unchanged.
	st2, err := sweepCLI(dir, sweepOpts{})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if st2 != statusUnchanged {
		t.Errorf("second run expected unchanged; got %q", st2)
	}
}

// TestSweepCLI_DryRunDoesNotWrite verifies the -dry-run flag short-
// circuits before any file write. The pre-sweep manifest survives
// untouched.
func TestSweepCLI_DryRunDoesNotWrite(t *testing.T) {
	dir := stageMinimalCLIDir(t)
	manifestPath := filepath.Join(dir, ".printing-press.json")
	before, _ := os.ReadFile(manifestPath)

	if _, err := sweepCLI(dir, sweepOpts{DryRun: true}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	after, _ := os.ReadFile(manifestPath)
	if string(before) != string(after) {
		t.Errorf("dry-run wrote to manifest: %s -> %s", before, after)
	}
	// Learn files should not appear.
	if _, err := os.Stat(filepath.Join(dir, "internal/learn/recall.go")); err == nil {
		t.Error("dry-run wrote learn files; expected none")
	}
}

// TestSweepCLI_ReadmeOnlyOnlyTouchesSkill verifies the -readme-only
// branch skips Go-source surgery and writes nothing else.
func TestSweepCLI_ReadmeOnlyOnlyTouchesSkill(t *testing.T) {
	dir := stageMinimalCLIDir(t)
	rootPath := filepath.Join(dir, "internal/cli/root.go")
	rootBefore, _ := os.ReadFile(rootPath)

	if _, err := sweepCLI(dir, sweepOpts{ReadmeOnly: true}); err != nil {
		t.Fatalf("readme-only: %v", err)
	}

	rootAfter, _ := os.ReadFile(rootPath)
	if string(rootBefore) != string(rootAfter) {
		t.Error("readme-only sweep modified root.go; expected untouched")
	}
	skill, _ := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if !strings.Contains(string(skill), "Automatic Learning") {
		t.Error("readme-only sweep did not patch SKILL.md")
	}
}

// stageMinimalCLIDir creates the minimal file set the sweep needs to
// run successfully on a fixture CLI:
//
//   - .printing-press.json with required identity fields
//   - SKILL.md with an H1 so the learn section has an insertion point
//   - internal/cli/root.go in canonical shape
//   - internal/store/store.go with the learn-migrations anchor
//   - go.mod already declaring modernc.org/sqlite so `go mod tidy`
//     is skipped (the test doesn't shell out)
//
// Returns the directory path.
func stageMinimalCLIDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".printing-press.json"),
		`{"api_name":"demo","cli_name":"demo-pp-cli","printing_press_version":"0.0.0"}`)
	writeFile(t, filepath.Join(dir, "SKILL.md"),
		"---\nname: pp-demo\n---\n\n# Demo CLI\n\n## Usage\n\nstuff.\n")
	writeFile(t, filepath.Join(dir, "internal/cli/root.go"), canonicalRootFlagsShape)
	writeFile(t, filepath.Join(dir, "internal/store/store.go"), preLearnStoreSnippet)
	writeFile(t, filepath.Join(dir, "go.mod"),
		"module github.com/example/demo-pp-cli\n\ngo 1.26\n\nrequire (\n\tmodernc.org/sqlite v1.37.0\n)\n")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
