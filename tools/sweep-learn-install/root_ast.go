// root_ast.go injects the learn-loop wiring into a CLI's
// internal/cli/root.go. Operates on the canonical rootFlags-struct
// shape (per printing-press-library/AGENTS.md "CLI root.go shape").
// The legacy `var rootCmd` package-global shape is refused with an
// error so the sweep does not silently no-op or produce a broken
// patch.
//
// Three pieces are injected:
//
//  1. A persistent `--no-learn` boolean flag on the root command,
//     stored as rootFlags.NoLearn.
//  2. teach / recall / learnings AddCommand registrations, gated
//     behind `if !flags.NoLearn`. The registrations sit alongside
//     the other AddCommand sites and reuse the same flags struct.
//  3. A learnHookSkipList map declaring command names that must
//     bypass the PersistentPreRunE learn-init hook. The list is
//     consulted by the spec-driven internal/cli/learn_init.go
//     emitted alongside this file (out of scope for this sweep —
//     the per-CLI Learn config drives that emission).
//
// Idempotency: a second run with the same input produces zero diff.
// Each injection probes for its own canonical marker before adding
// and is a no-op when the marker is already present.

package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

type rootShape int

const (
	rootShapeUnknown rootShape = iota
	// rootShapeFlagsStruct is the canonical shape: a rootFlags type
	// + Execute() with a local rootCmd binding + addPersistentFlags
	// against that local. The generator emits this for every new
	// CLI; the sweep retrofits learn wiring into it.
	rootShapeFlagsStruct
	// rootShapeLegacy is the agent-capture / instacart shape: a
	// package-global var rootCmd with no rootFlags struct. The AST
	// sweep refuses to patch this shape and reports it to the
	// operator for manual review.
	rootShapeLegacy
)

// detectRootShape parses root.go and decides which shape it carries.
// Returns rootShapeUnknown when the file doesn't even parse so the
// caller surfaces a clear error rather than silently no-oping.
func detectRootShape(src []byte) (rootShape, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "root.go", src, parser.ParseComments)
	if err != nil {
		return rootShapeUnknown, fmt.Errorf("parse root.go: %w", err)
	}

	hasRootFlagsType := false
	hasPackageRootCmd := false
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if s.Name != nil && s.Name.Name == "rootFlags" {
					hasRootFlagsType = true
				}
			case *ast.ValueSpec:
				for _, n := range s.Names {
					if n.Name == "rootCmd" {
						hasPackageRootCmd = true
					}
				}
			}
		}
	}

	if hasRootFlagsType {
		return rootShapeFlagsStruct, nil
	}
	if hasPackageRootCmd {
		return rootShapeLegacy, nil
	}
	return rootShapeUnknown, fmt.Errorf("root.go shape unrecognized (no rootFlags type, no var rootCmd)")
}

// patchRootAST applies the three injections (flag, AddCommand calls,
// skip-list) to a canonical-shape root.go. Returns the new source
// (still go-fmt-clean because edits operate on whole lines or self-
// contained blocks) plus a changed boolean.
func patchRootAST(src string, ctx sweepCtx) (string, bool, error) {
	out := src
	changed := false

	if added, ok := injectNoLearnFlagField(out); ok {
		out = added
		changed = true
	}
	if added, ok := injectNoLearnPersistentFlag(out); ok {
		out = added
		changed = true
	}
	if added, ok := injectLearnAddCommands(out, ctx); ok {
		out = added
		changed = true
	}
	if added, ok := injectLearnHookSkipList(out); ok {
		out = added
		changed = true
	}
	if changed {
		// Run gofmt over the final source so injection seams (extra
		// blank lines, slightly off-spec indentation) settle into a
		// canonical shape. If gofmt fails (a non-canonical input
		// would surface as a compile error downstream), pass the
		// raw output through and let the caller see it.
		if formatted, err := format.Source([]byte(out)); err == nil {
			out = string(formatted)
		}
	}
	return out, changed, nil
}

// injectNoLearnFlagField adds a NoLearn bool to the rootFlags struct.
// Idempotent: skipped when the field is already present.
func injectNoLearnFlagField(src string) (string, bool) {
	if strings.Contains(src, "NoLearn bool") {
		return src, false
	}
	// Find the rootFlags struct opening brace and inject a NoLearn
	// field right before the closing brace. Conservative: matches
	// the literal `type rootFlags struct {` header so we don't
	// accidentally patch a similarly-named local.
	const header = "type rootFlags struct {"
	idx := strings.Index(src, header)
	if idx < 0 {
		return src, false
	}
	openBrace := idx + len(header) - 1
	depth := 0
	closeIdx := -1
	for i := openBrace; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		return src, false
	}
	// Walk back to the line start so we can insert a properly
	// indented line just before the closing brace.
	lineStart := closeIdx
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	insertion := "\t// NoLearn suppresses self-learning loop seed/extract/recall side\n" +
		"\t// effects when true. Set by the persistent --no-learn flag.\n" +
		"\tNoLearn bool\n"
	return src[:lineStart] + insertion + src[lineStart:], true
}

// injectNoLearnPersistentFlag adds the cobra BoolVar binding for
// --no-learn. Idempotent: skipped when the binding is already present.
func injectNoLearnPersistentFlag(src string) (string, bool) {
	if strings.Contains(src, `BoolVar(&flags.NoLearn, "no-learn"`) {
		return src, false
	}
	// Find the last line in Execute() that calls rootCmd.PersistentFlags()
	// and inject immediately after the end of that line. Line-scope
	// matching avoids splitting a chained method call (the `()` in
	// `PersistentFlags()` would otherwise satisfy the first depth=0
	// drop and yield a splice point inside the statement).
	lineEnd := lastLineEndContaining(src, "rootCmd.PersistentFlags()")
	if lineEnd < 0 {
		return src, false
	}
	insertion := "\trootCmd.PersistentFlags().BoolVar(&flags.NoLearn, \"no-learn\", false, \"Disable self-learning loop side effects (recall, teach, preseed)\")\n"
	return src[:lineEnd] + insertion + src[lineEnd:], true
}

// lastLineEndContaining returns the byte offset just past the newline
// of the last line that contains needle. -1 when none. Used by
// inject* helpers that want to splice immediately after a stable
// per-line anchor.
func lastLineEndContaining(src, needle string) int {
	idx := strings.LastIndex(src, needle)
	if idx < 0 {
		return -1
	}
	lineEnd := strings.Index(src[idx:], "\n")
	if lineEnd < 0 {
		return len(src)
	}
	return idx + lineEnd + 1
}

// injectLearnAddCommands wires the teach/recall/learnings cobra
// commands into root.go. The teach package emits newTeachCmd /
// newRecallCmd / newLearningsCmd constructors in internal/cli/teach.go;
// this injection is the one site root.go calls them from.
//
// Idempotent: skipped when newTeachCmd is already referenced.
func injectLearnAddCommands(src string, ctx sweepCtx) (string, bool) {
	if strings.Contains(src, "newTeachCmd(&flags)") {
		return src, false
	}
	// Anchor on the last line that calls rootCmd.AddCommand. Same
	// line-scoped splicing as injectNoLearnPersistentFlag to keep
	// each statement intact.
	lineEnd := lastLineEndContaining(src, "rootCmd.AddCommand(")
	if lineEnd < 0 {
		return src, false
	}
	insertion := "\trootCmd.AddCommand(newTeachCmd(&flags))\n" +
		"\trootCmd.AddCommand(newRecallCmd(&flags))\n" +
		"\trootCmd.AddCommand(newLearningsCmd(&flags))\n"
	return src[:lineEnd] + insertion + src[lineEnd:], true
}

// injectLearnHookSkipList adds the learnHookSkipList map. The list
// names commands that must bypass the PersistentPreRunE learn-init
// hook (auth, doctor, version, help — commands that ship without a
// store). Defined at file scope so the per-CLI learn_init.go emitted
// alongside can consult it via PersistentPreRunE.
//
// Idempotent: skipped when learnHookSkipList already exists.
func injectLearnHookSkipList(src string) (string, bool) {
	if strings.Contains(src, "learnHookSkipList") {
		return src, false
	}
	// Append at file end so we don't disturb any existing top-level
	// declarations. The block carries its own doc comment so a
	// downstream reader knows what it's for without grepping.
	insertion := "\n// learnHookSkipList declares commands that must bypass the\n" +
		"// PersistentPreRunE learn-init hook. These commands ship without a\n" +
		"// store and run before initLearn would be safe to call. Keep in sync\n" +
		"// with the generator's learn_init.go template.\n" +
		"var learnHookSkipList = map[string]struct{}{\n" +
		"\t\"auth\":       {},\n" +
		"\t\"completion\": {},\n" +
		"\t\"doctor\":     {},\n" +
		"\t\"help\":       {},\n" +
		"\t\"version\":    {},\n" +
		"}\n"
	out := src
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out + insertion, true
}
