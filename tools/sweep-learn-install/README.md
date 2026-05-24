# sweep-learn-install

Retrofits the self-learning loop (`internal/learn` package + supporting
wiring) into every per-CLI entry under `library/<cat>/<api>/`. Sister
tool to `tools/sweep-canonical/`; same GOPATH-mode + idempotency
contract, but does Go-source surgery instead of markdown patches.

## When to run

Run this after `cli-printing-press`'s learn-loop templates change in a
way that the published library should adopt. Fresh prints from the
generator already produce the canonical shape; this tool brings
existing entries up to the same shape without a full regeneration.

The contract for when an upstream template change requires a parallel
sweep run lives in `cli-printing-press`'s AGENTS.md under
"Cross-repo dependency: published-library sweep tool" (the learn-loop
section). The tracking-issue flow described there applies.

## Invocation

From the repo root:

```bash
SWEEP_LIBRARY_ROOT=library GO111MODULE=off go run ./tools/sweep-learn-install
```

Flags:

- `-readme-only` / `--readme-only` / `SWEEP_README_ONLY=1` — patch
  only `SKILL.md`, skip Go-source surgery.
- `-dry-run` / `--dry-run` / `SWEEP_DRY_RUN=1` — print planned
  changes per CLI, write nothing.
- `-only=<slug>` — restrict to a single library entry for debugging.

Tests:

```bash
cd tools/sweep-learn-install && GO111MODULE=off go test .
```

## What the sweep does, per CLI

Atomic per directory; any error rolls back every file written for that
CLI from an in-memory snapshot before moving on. Non-zero exit if any
CLI errored.

1. Skip if `.printing-press.json` is absent.
2. Skip if `.no-learn-sweep` opt-out marker is present.
3. Refuse if `internal/cli/root.go` uses the legacy `var rootCmd`
   shape (agent-capture / instacart style). Reported as
   `manual review required`.
4. Skip if `internal/store/store.go` is missing the
   `// CLI Printing Press: learn migrations` anchor — store.go is
   presumed hand-modified.
5. Render the `internal/learn/*.go` package files. **Byte-for-byte
   parity** with `cli-printing-press` is enforced by
   `TestRenderLearnPackage_ByteForByteParity` against the
   `generate-learn-loop-api` golden fixture.
6. AST-inject `internal/cli/root.go` to add the `--no-learn` flag,
   the teach/recall/learnings `AddCommand` calls, and the
   `learnHookSkipList` map.
7. Rewrite the learn-migrations block in `store.go` and bump
   `StoreSchemaVersion` to 3.
8. Patch `SKILL.md` to add the Automatic Learning section
   (idempotent strip-then-re-emit between
   `<!-- pp-learn-section-start -->` /
   `<!-- pp-learn-section-end -->` anchors).
9. Add `modernc.org/sqlite` to `go.mod` if missing, then run
   `go mod tidy` in the CLI directory.
10. Update `printing_press_version` in `.printing-press.json`.

## What this tool does NOT do

- No `.printing-press-patches.json` entry. The learn loop is a
  generator-owned package, not a per-CLI patch.
- No per-CLI `internal/cli/learn_init.go` or `internal/cli/teach.go`
  emission. Those files are spec-driven (ticker patterns, stopwords,
  entity-lookup seeds) and the per-CLI Learn config that drives them
  isn't part of this sweep. A separate retrofit step (U14+) owns
  feeding Learn config into each library entry.
- No edits under `tools/sweep-canonical/`. The two tools are
  siblings, not chained.

## Embedded templates

`templates/learn{,_entities,_lookups,_patterns}/*.tmpl` are verbatim
copies of the upstream templates from
`cli-printing-press/internal/generator/templates/`. They are embedded
via `go:embed` so the sweep tool ships with the exact source the
generator parses, and the parity test fails if the local copies drift
from upstream.

When the upstream templates change in a way the library should adopt,
copy the updated files here and re-run the parity test before
committing.
