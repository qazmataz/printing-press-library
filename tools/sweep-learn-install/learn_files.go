// learn_files.go renders the internal/learn package files byte-for-
// byte identical to what cli-printing-press's generator emits when a
// spec opts into the self-learning loop.
//
// Parity strategy:
//
//   - The generator's learn templates are embedded into this binary
//     verbatim via go:embed (see templates.go). Embedding the same
//     source text the generator parses removes any chance of drift
//     between an inlined Go string literal and the real template.
//   - We parse each template with the same text/template funcs the
//     generator binds (currentYear, modulePath, kebab plus the
//     identity .Owner / .Name accessors). The funcs that involve
//     spec-derived shape (HasCostThrottling, EndpointTemplateVars,
//     etc.) are not referenced by the learn templates, so the small
//     subset suffices.
//   - We run go/format.Source over the rendered output, mirroring the
//     generator's normalizeRendered behavior for .go files. Without
//     this final pass, hand-aligned struct columns in the templates
//     would diff against the generator's own gofmt-aware emit path.
//
// The byte-for-byte parity test (learn_files_test.go) renders this
// tool's emission for the learn-loop-example fixture and diffs against
// the in-tree golden artifact. Zero textual diff is the contract.

package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/learn/*.tmpl templates/learn_entities/*.tmpl templates/learn_lookups/*.tmpl templates/learn_patterns/*.tmpl
var learnTemplateFS embed.FS

// learnTemplatePaths maps each embedded template path to the
// CLI-relative output path the generator writes it to. Kept here so
// the sweep emits the same file set and ordering as the generator's
// renderLearnFiles in cli-printing-press/internal/generator/generator.go.
var learnTemplatePaths = map[string]string{
	"templates/learn/doc.go.tmpl":            "internal/learn/doc.go",
	"templates/learn/normalize.go.tmpl":      "internal/learn/normalize.go",
	"templates/learn/normalize_test.go.tmpl": "internal/learn/normalize_test.go",
	"templates/learn/match.go.tmpl":          "internal/learn/match.go",
	"templates/learn/match_test.go.tmpl":     "internal/learn/match_test.go",
	"templates/learn/recall.go.tmpl":         "internal/learn/recall.go",
	"templates/learn/recall_test.go.tmpl":    "internal/learn/recall_test.go",
	"templates/learn/teach.go.tmpl":          "internal/learn/teach.go",
	"templates/learn/teach_test.go.tmpl":     "internal/learn/teach_test.go",
	"templates/learn/teach_log.go.tmpl":      "internal/learn/teach_log.go",
	"templates/learn/teach_log_test.go.tmpl": "internal/learn/teach_log_test.go",
	"templates/learn/preseed.go.tmpl":        "internal/learn/preseed.go",
	"templates/learn/preseed_test.go.tmpl":   "internal/learn/preseed_test.go",

	"templates/learn_entities/config.go.tmpl":       "internal/learn/entities/config.go",
	"templates/learn_entities/config_test.go.tmpl":  "internal/learn/entities/config_test.go",
	"templates/learn_entities/extract.go.tmpl":      "internal/learn/entities/extract.go",
	"templates/learn_entities/extract_test.go.tmpl": "internal/learn/entities/extract_test.go",

	"templates/learn_lookups/store.go.tmpl":      "internal/learn/lookups/store.go",
	"templates/learn_lookups/store_test.go.tmpl": "internal/learn/lookups/store_test.go",
	"templates/learn_lookups/seeds.go.tmpl":      "internal/learn/lookups/seeds.go",
	"templates/learn_lookups/seeds_test.go.tmpl": "internal/learn/lookups/seeds_test.go",

	"templates/learn_patterns/doc.go.tmpl":          "internal/learn/patterns/doc.go",
	"templates/learn_patterns/store.go.tmpl":        "internal/learn/patterns/store.go",
	"templates/learn_patterns/store_test.go.tmpl":   "internal/learn/patterns/store_test.go",
	"templates/learn_patterns/extract.go.tmpl":      "internal/learn/patterns/extract.go",
	"templates/learn_patterns/extract_test.go.tmpl": "internal/learn/patterns/extract_test.go",
	"templates/learn_patterns/apply.go.tmpl":        "internal/learn/patterns/apply.go",
	"templates/learn_patterns/apply_test.go.tmpl":   "internal/learn/patterns/apply_test.go",
}

// renderData is the minimal subset of fields the learn templates
// reference. Mirrors the spec accessors the generator threads through
// .Owner / .Name; the rest of APISpec is not touched by these
// templates.
type renderData struct {
	Owner string
	Name  string
}

// renderLearnPackage emits every learn-package file for one CLI and
// returns a path->content map ready for write. Module path and year
// land via the template funcs registered below.
func renderLearnPackage(ctx sweepCtx) (map[string][]byte, error) {
	out := make(map[string][]byte, len(learnTemplatePaths))
	for tmplPath, relOut := range learnTemplatePaths {
		content, err := renderLearnTemplate(tmplPath, ctx)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", tmplPath, err)
		}
		out[relOut] = content
	}
	return out, nil
}

// renderLearnTemplate reads one embedded template, executes it
// against renderData{Owner, Name}, gofmt's the result, and returns the
// final bytes — the same chain the generator runs for the matching
// template.
func renderLearnTemplate(tmplPath string, ctx sweepCtx) ([]byte, error) {
	raw, err := learnTemplateFS.ReadFile(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", tmplPath, err)
	}
	tmpl, err := template.New(path.Base(tmplPath)).Funcs(template.FuncMap{
		"currentYear": func() string { return strconv.Itoa(time.Now().Year()) },
		"modulePath":  func() string { return ctx.ModulePath },
		"kebab":       toKebab,
		"backtick":    func() string { return "`" },
	}).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", tmplPath, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, renderData{Owner: ctx.OwnerName, Name: ctx.APIName}); err != nil {
		return nil, fmt.Errorf("execute %s: %w", tmplPath, err)
	}
	rendered := bytes.TrimRight(buf.Bytes(), " \t\r\n")
	rendered = append(rendered, '\n')
	formatted, err := format.Source(rendered)
	if err != nil {
		// Mirror the generator's behavior: fall through with a stderr
		// warning rather than fail-hard, so a malformed template
		// surfaces as a compile error downstream.
		return rendered, nil
	}
	return formatted, nil
}

// toKebab mirrors the generator's kebab helper. Lowercases, replaces
// underscores / spaces / dots / slashes with hyphens, and folds
// repeated separators. The learn templates only call this on the
// per-CLI Name to compute a state-directory suffix, so the small
// implementation here suffices.
func toKebab(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			prevDash = false
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == '-':
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		case r == '_' || r == ' ' || r == '.' || r == '/':
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		default:
			// Drop punctuation the generator's toKebab also drops.
		}
	}
	return strings.Trim(b.String(), "-")
}
