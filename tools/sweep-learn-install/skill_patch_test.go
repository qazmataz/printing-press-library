package main

import (
	"strings"
	"testing"
)

const skillBefore = `---
name: pp-demo
description: "Demo CLI"
---

# Demo CLI

This is a demo CLI.

## Usage

Some stuff.
`

func TestPatchSkillLearnSection_InsertsAfterH1(t *testing.T) {
	ctx := sweepCtx{CLIName: "demo-pp-cli", APIName: "demo"}
	got := patchSkillLearnSection(skillBefore, ctx)

	if !strings.Contains(got, "## Automatic Learning") {
		t.Errorf("expected Automatic Learning heading; got:\n%s", got)
	}
	if !strings.Contains(got, learnSectionStart) {
		t.Errorf("expected start anchor; got:\n%s", got)
	}
	if !strings.Contains(got, learnSectionEnd) {
		t.Errorf("expected end anchor; got:\n%s", got)
	}
	// Section should appear after the H1, before Usage.
	h1Idx := strings.Index(got, "# Demo CLI")
	learnIdx := strings.Index(got, "## Automatic Learning")
	usageIdx := strings.Index(got, "## Usage")
	if h1Idx < 0 || learnIdx < 0 || usageIdx < 0 {
		t.Fatalf("missing required heading: h1=%d learn=%d usage=%d", h1Idx, learnIdx, usageIdx)
	}
	if !(h1Idx < learnIdx && learnIdx < usageIdx) {
		t.Errorf("expected order H1 -> Automatic Learning -> Usage; got %d/%d/%d", h1Idx, learnIdx, usageIdx)
	}
}

func TestPatchSkillLearnSection_Idempotent(t *testing.T) {
	ctx := sweepCtx{CLIName: "demo-pp-cli", APIName: "demo"}
	first := patchSkillLearnSection(skillBefore, ctx)
	second := patchSkillLearnSection(first, ctx)
	if second != first {
		t.Errorf("second run produced diff:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestPatchSkillLearnSection_StaleContentReplaced(t *testing.T) {
	bodyWithStaleSection := `# Demo CLI

` + learnSectionStart + `
## Automatic Learning

OLD STALE CONTENT FROM PRIOR SWEEP — should be replaced.
` + learnSectionEnd + `

## Usage

Some stuff.
`
	ctx := sweepCtx{CLIName: "demo-pp-cli", APIName: "demo"}
	got := patchSkillLearnSection(bodyWithStaleSection, ctx)
	if strings.Contains(got, "OLD STALE CONTENT") {
		t.Errorf("stale content not removed:\n%s", got)
	}
	if !strings.Contains(got, "recall") {
		t.Errorf("expected canonical recall mention; got:\n%s", got)
	}
}

func TestPatchSkillLearnSection_NamesCLI(t *testing.T) {
	ctx := sweepCtx{CLIName: "weather-pp-cli", APIName: "weather"}
	got := patchSkillLearnSection(skillBefore, ctx)
	if !strings.Contains(got, "weather-pp-cli") {
		t.Errorf("expected CLI binary name in section; got:\n%s", got)
	}
}
