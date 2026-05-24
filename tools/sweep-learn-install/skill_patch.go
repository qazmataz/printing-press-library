// skill_patch.go retrofits the Automatic Learning section into a CLI's
// SKILL.md. Idempotent strip-and-re-emit pattern mirroring
// sweep-canonical: the section sits between two stable HTML-comment
// anchors so a re-sweep can locate and rewrite it without ambiguity.

package main

import (
	"fmt"
	"strings"
)

// learnSectionStart / learnSectionEnd are the anchor comments that
// bracket the Automatic Learning section. Future sweeps strip and
// re-emit content between these anchors so install-instructions
// or wording updates can propagate.
const (
	learnSectionStart = "<!-- pp-learn-section-start -->"
	learnSectionEnd   = "<!-- pp-learn-section-end -->"
)

// patchSkillLearnSection inserts (or refreshes) the Automatic
// Learning section in SKILL.md. The section is anchored between
// pp-learn-section-start / pp-learn-section-end comments so re-runs
// are deterministic.
//
// Insertion point: immediately after the H1 line (the CLI's name)
// when no prior section exists. For files that already carry the
// anchors, the existing block is stripped first and the canonical
// content is re-emitted in place.
func patchSkillLearnSection(body string, ctx sweepCtx) string {
	body = stripLearnSection(body)
	section := buildLearnSection(ctx)
	return insertLearnSection(body, section)
}

// stripLearnSection removes anything between (and including) the
// learn-section anchors. Tolerant of missing anchors so the first
// sweep can run; subsequent sweeps replace the block in place.
func stripLearnSection(body string) string {
	startIdx := strings.Index(body, learnSectionStart)
	if startIdx < 0 {
		return body
	}
	endIdx := strings.Index(body, learnSectionEnd)
	if endIdx < 0 {
		return body
	}
	endIdx += len(learnSectionEnd)
	// Trim leading/trailing whitespace adjacent to the block so
	// re-emission leaves a single blank line on each side.
	start := startIdx
	for start > 0 && (body[start-1] == '\n' || body[start-1] == ' ') {
		start--
	}
	end := endIdx
	for end < len(body) && (body[end] == '\n' || body[end] == ' ') {
		end++
	}
	return body[:start] + "\n\n" + body[end:]
}

// buildLearnSection emits the canonical Automatic Learning section.
// The wording is deliberately CLI-agnostic — the per-CLI examples
// belong in narrative recipes, not in the inherited section (per
// learn-purity policy).
func buildLearnSection(ctx sweepCtx) string {
	return fmt.Sprintf(`%s
## Automatic Learning

This CLI ships a self-learning loop. When you run %s commands, the binary
quietly records the queries and the resources you act on so future
recall calls can rerank results based on prior success.

What the loop does on each command:

- `+"`recall`"+` looks up prior queries that overlap with the current one
  via a token-set Jaccard match (floor 0.6) and returns the resources
  that were associated with those queries, ranked by confidence and
  entity overlap.
- `+"`teach`"+` records a query / resource association so future
  recall calls can find it.
- `+"`learnings`"+` lists what the loop has stored locally so you can
  audit and curate the dataset.

Opting out: pass `+"`--no-learn`"+` on the root command (e.g.
`+"`%s --no-learn <command>`"+`) to disable the loop's side effects for
one invocation.

The learn store lives alongside the regular SQLite cache and is
populated additively on each command — there is no separate sync
step. See the printed CLI's `+"`internal/learn/`"+` source for the loop's
schema and the match scoring contract.
%s`, learnSectionStart, ctx.CLIName, ctx.CLIName, learnSectionEnd)
}

// insertLearnSection puts the canonical section right after the
// first H1 in SKILL.md. Falls back to prepend if no H1 exists.
func insertLearnSection(body, section string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			head := strings.Join(lines[:i+1], "\n")
			tail := strings.Join(lines[i+1:], "\n")
			return head + "\n\n" + section + "\n" + tail
		}
	}
	return section + "\n" + body
}
