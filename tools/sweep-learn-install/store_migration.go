// store_migration.go retrofits the learn-loop migrations into a CLI's
// internal/store/store.go. The contract is anchor-based:
//
//   - The generator's store.go template (post-U6) carries the literal
//     `// CLI Printing Press: learn migrations` marker right before
//     the learn-loop CREATE TABLE statements.
//   - The sweep finds that marker, replaces the canonical migrations
//     block (marker + 5 statements), and bumps StoreSchemaVersion
//     to the learn-enabled value.
//   - If the marker is missing, the sweep refuses to patch store.go.
//     A pre-anchor or hand-modified store.go is outside this tool's
//     contract; the manual-review skip path catches it (see main.go's
//     sweepCLI).
//
// Idempotency: a second run with the same input produces zero diff.
// The marker locates the block; the block contents are re-emitted
// verbatim from the canonical source below.

package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	learnMigrationAnchor = "// CLI Printing Press: learn migrations"
	learnSchemaVersion   = 3
)

// hasLearnMigrationAnchor reports whether store.go already carries the
// canonical learn-migrations marker. Used by sweepCLI to decide
// whether the CLI is in scope.
func hasLearnMigrationAnchor(src []byte) bool {
	return strings.Contains(string(src), learnMigrationAnchor)
}

// canonicalLearnMigrationsBlock is the exact text the generator emits
// between the FTS create statement and the per-CLI tables (post-U6).
// Tab-indented to match the template's emission so the file remains
// gofmt-clean after the splice. Keep in sync with
// cli-printing-press/internal/generator/templates/store.go.tmpl.
const canonicalLearnMigrationsBlock = `		// CLI Printing Press: learn migrations
		` + "`CREATE TABLE IF NOT EXISTS search_learnings (\n" +
	`			query_pattern TEXT NOT NULL,
			query_entities TEXT NOT NULL DEFAULT '[]',
			resource_ids TEXT NOT NULL DEFAULT '[]',
			resource_type TEXT NOT NULL,
			venue TEXT,
			action TEXT,
			confidence INTEGER NOT NULL DEFAULT 0,
			source TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (query_pattern, resource_type)
		` + "`,\n" +
	"		`CREATE TABLE IF NOT EXISTS search_patterns (\n" +
	`			template TEXT NOT NULL,
			entity_kind TEXT NOT NULL,
			confidence INTEGER NOT NULL DEFAULT 0,
			source TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (template, entity_kind)
		` + "`,\n" +
	"		`CREATE TABLE IF NOT EXISTS entity_lookups (\n" +
	`			canonical TEXT NOT NULL,
			alias TEXT NOT NULL,
			kind TEXT NOT NULL,
			source TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (canonical, alias, kind)
		` + "`,\n" +
	"		`CREATE TABLE IF NOT EXISTS teach_log_metadata (\n" +
	`			rotation_at DATETIME,
			last_size_bytes INTEGER NOT NULL DEFAULT 0
		` + "`,\n" +
	"		`CREATE VIRTUAL TABLE IF NOT EXISTS search_learnings_fts USING fts5(\n" +
	`			query_pattern, tokenize='porter unicode61'
		` + "`,"

// learnMigrationsBlockEndMarker is the closing fence of the canonical
// block. The first CREATE TABLE outside the block (per-CLI tables,
// emitted from spec.Tables) starts with this anchor pattern in the
// store template. Used to delimit the rewrite range.
const learnMigrationsBlockEndMarker = "`CREATE VIRTUAL TABLE IF NOT EXISTS search_learnings_fts USING fts5("

// patchStoreMigrations rewrites the learn-migrations block in store.go
// to its canonical content and bumps StoreSchemaVersion. Returns the
// new source, a changed boolean, and any error encountered while
// locating the block boundaries.
func patchStoreMigrations(src string, _ sweepCtx) (string, bool, error) {
	if !strings.Contains(src, learnMigrationAnchor) {
		// Caller should have screened this out; defensive double-check.
		return src, false, fmt.Errorf("learn-migrations anchor not found")
	}

	// Locate the block start (the anchor line) and end (the trailing
	// backtick of the search_learnings_fts CREATE).
	startIdx := strings.Index(src, learnMigrationAnchor)
	if startIdx < 0 {
		return src, false, fmt.Errorf("anchor missing after presence check")
	}
	// Walk back to the line start so the replacement begins at the
	// canonical indent.
	lineStart := startIdx
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}

	// Locate the search_learnings_fts CREATE that closes the block.
	ftsIdx := strings.Index(src[lineStart:], learnMigrationsBlockEndMarker)
	if ftsIdx < 0 {
		return src, false, fmt.Errorf("learn-migrations block end marker not found")
	}
	ftsIdx += lineStart
	// Walk forward to the closing backtick + comma of the FTS CREATE.
	rest := src[ftsIdx:]
	tickIdx := strings.Index(rest, "`,")
	if tickIdx < 0 {
		return src, false, fmt.Errorf("FTS create not terminated with backtick+comma")
	}
	blockEnd := ftsIdx + tickIdx + len("`,")
	// Include the trailing newline.
	if blockEnd < len(src) && src[blockEnd] == '\n' {
		blockEnd++
	}

	canonical := canonicalLearnMigrationsBlock + "\n"
	newSrc := src[:lineStart] + canonical + src[blockEnd:]

	// Bump StoreSchemaVersion to learnSchemaVersion if it's lower.
	newSrc = bumpStoreSchemaVersion(newSrc, learnSchemaVersion)

	return newSrc, newSrc != src, nil
}

var storeSchemaVersionRe = regexp.MustCompile(`const StoreSchemaVersion = (\d+)`)

// bumpStoreSchemaVersion replaces `const StoreSchemaVersion = N` with
// the target when N is lower; idempotent otherwise. Does not touch
// any other `const Store...` declarations.
func bumpStoreSchemaVersion(src string, target int) string {
	return storeSchemaVersionRe.ReplaceAllStringFunc(src, func(match string) string {
		sub := storeSchemaVersionRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		current := 0
		_, err := fmt.Sscanf(sub[1], "%d", &current)
		if err != nil {
			return match
		}
		if current >= target {
			return match
		}
		return fmt.Sprintf("const StoreSchemaVersion = %d", target)
	})
}
