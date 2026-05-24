// go_mod_helper.go owns the go.mod retrofit step for the seven library
// CLIs (snapshot mid-2026) that do not yet carry modernc.org/sqlite.
// `go mod tidy` runs afterward to populate go.sum.
//
// Snapshots of go.mod AND go.sum are taken by applySweep before any
// of the work below runs, so a tidy failure restores both files
// cleanly. We do not pin a specific transitive shape; tidy resolves
// whatever go.mod expresses.

package main

import (
	"fmt"
	"os"
	"strings"
)

// addSQLiteDep appends the modernc.org/sqlite require line to go.mod
// when missing. The added line goes inside the require block if one
// exists; otherwise a new require block is appended.
//
// Idempotent: skipped when modernc.org/sqlite is already declared.
func addSQLiteDep(goModPath string) error {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("read go.mod: %w", err)
	}
	src := string(data)
	if strings.Contains(src, "modernc.org/sqlite") {
		return nil
	}

	line := fmt.Sprintf("\tmodernc.org/sqlite %s\n", sqliteVersion)

	// Look for an existing `require (` block.
	if idx := strings.Index(src, "\nrequire (\n"); idx >= 0 {
		closeIdx := strings.Index(src[idx:], "\n)")
		if closeIdx < 0 {
			return fmt.Errorf("require block has no closing paren")
		}
		closeIdx += idx
		newSrc := src[:closeIdx] + line + src[closeIdx:]
		return os.WriteFile(goModPath, []byte(newSrc), 0o644)
	}

	// No block — append a fresh one. Lands at EOF after a separator
	// blank line to keep the rest of go.mod's shape stable.
	if !strings.HasSuffix(src, "\n") {
		src += "\n"
	}
	src += "\nrequire (\n" + line + ")\n"
	return os.WriteFile(goModPath, []byte(src), 0o644)
}
