# Phase 4.8/4.9/4.95 Findings Ledger — appmagic-pp-cli

## Template-shape retro candidates (full detail; NOT fixed in generated templates)
1. **Basic-auth pair vs token-shaped auth scaffold** — internal/cli/auth.go (generated) +
   internal/config/config.go:174 (generated). For `securitySchemes: http/basic` with a
   two-var x-auth-env-vars pair, `auth setup` prints `export APPMAGIC_LOGIN="<your-token>"`
   + `export APPMAGIC_PASSWORD="<your-token>"` (both labeled "token") and recommends
   `auth set-token <token>`, but set-token writes the value into the LOGIN config field
   only — a single token can never satisfy the login+password pair, so set-token leaves
   auth permanently incomplete on Basic-auth CLIs. Filed for retro: the press's auth
   templates need a basic-pair-aware branch (setup text naming login/password and a
   set-credentials login+password form, or set-token disabled for basic).
   Mitigation applied in THIS print: corrected the setup prose in place (see autofix
   summary) and docs consistently instruct setting both env vars.
2. **classifyAPIError hint says "Set your API key: export APPMAGIC_LOGIN=<your-key>"**
   (generated helpers.go) — wording is key-shaped for a Basic login; cosmetic, same root
   cause as #1. Retro: hint template should render auth-mode-aware wording.
3. **Sample-probe parallel first-run SQLITE_BUSY** — 12-way concurrent first-time DB
   init can lose the 5s busy race during DDL (store already WAL + busy_timeout(5000)).
   Retro: scorecard sampler could serialize first-run init or retry SQLITE_BUSY once.

## Phase 4.8 SKILL review (PASS, 4 warnings — fixed in batched doc pass)
- web liveops-tags + web tag-counts entries lacked the APPMAGIC_WEB_TOKEN note.
- asa/contacts/period-comparison/steam command-reference headers lacked the
  entitlement-gated label.
- tag-rollup recipe claim implied genre-wide totals; qualified with top-N wording.
- search recipe claimed exit 2 on no-match, false under --agent (exit 0, empty matches).

## Phase 4.95 security review (lens 2 of 3)
- VERDICT: source clean (no SQLi, no credential leakage, no URL injection; webapi reads
  capped 32MB; allowlisted path segments; masked credentials in cached probe details).
- FIXED IMMEDIATELY (commit ae23606):
  - CRITICAL: dev binaries embedded /Users/<operator>/go debug paths (no -trimpath);
    root binary deleted, stage binary rebuilt -trimpath -ldflags=-buildid= (0 hits);
    .mcpb inner binaries verified already clean.
  - HIGH: unanchored .gitignore line swallowed cmd/appmagic-pp-cli/ from the tree;
    anchored all patterns, cmd/appmagic-pp-cli/main.go now tracked.
- QUEUED for batch pass: LOW watchlist report --since uncapped goroutine-per-day fan-out
  (cap at 366d); INFO LIKE-wildcard escape + snapshot pruning (optional, may skip).
- Template-shape retro candidates added: generated client io.ReadAll without LimitReader;
  webapi Retry-After parsed but not slept (politeness).

## Phase 4.95 maintainability review (lens 3 of 3) — 9 findings, none high; all queued for batch autofix
1. MED openSnapshotDB/snapshotMirrorMissing helpers exist but 7 commands hand-roll the same block — hoist + reuse.
2. MED envelope-tolerant array decode duplicated 4x — extract decodeObjectArray(data, keys...).
3. LOW two divergent store-id classifiers (watchlist_report vs aso_movers) — consolidate.
4. MED --db registered but never read on aso-movers + liveops-overlay — drop the flag.
5. LOW wlFetchResult.idx written never read — delete field.
6. MED tag_rollup.go pp:data-source says auto but behavior is live — change annotation to live.
7. MED-LOW entitlements missing-mirror guard misses flags.agent — add (and route via shared guard).
8. LOW migrations comment claims doctor reads entitlement_probes — it doesn't; fix comment.
9. LOW liveops-overlay outputs catalog 'duration' array under JSON field 'tags' — rename to duration.
Also queued: watchlist report --since capped at 366d (security LOW); auth.go setup prose
corrected for Basic pair (4.8 side note; in-place mitigation, template fix goes to retro).

## Phase 4.9 README/SKILL/AGENTS audit — 2 errors, 5 warnings, 3 info; GD-scan PASS everywhere
ERRORS: Claude Desktop JSON env block omits APPMAGIC_PASSWORD (+ <your-key> should be
<your-login>); troubleshoot claims doctor surfaces X-RateLimit-Reset (doctor does not -
absorbed row 24's surfacing half was NOT shipped; 429 adaptive-limiter half DID ship;
disposition: reword docs + Known Gap disclosure in shipcheck report).
WARNINGS: MCPB step-3 names one env var; doctor quickstart comment overstates /last-date;
"sync twice" advice wrong for auto-capturing snapshot commands; "Appmagic" casing 2x;
auth-status vs `auth status`.
INFO: generic "list command" boilerplate; --idempotent wording; env table gaps (LOGIN
description empty, WEB_TOKEN row missing).
CLEAN: 137/137 documented paths resolve; 12/12 novel features match; zero employer strings
incl. strings(1) on binaries.

## Phase 4.95 correctness review (lens 1 of 3) — 1 MED + 5 LOW, all verified
C1 MED watchlist_report wlParseHistoryRecords returns nil on shape drift with no error ->
   metrics silently zero; must return error into fetch_failures (rule-15 violation).
C2 LOW webapi 429 loop never sleeps Retry-After (limiter floor ~2s); context-aware sleep
   min(retryAfter, cap) before retry.
C3 LOW chart-diff accepts --from > --to, silently inverts diff; usageErr.
C4 LOW watchlist add stores united id "0" on identity drift; guard ua.ID==0.
C5 LOW watchlist report joins >100 ids into united_application_ids (spec maxItems:100);
   chunk into <=100 batches.
C6 LOW replaceChartSnapshot name carry-forward scan continue drops rows silently; warn/error.
CLEAN: JSON shapes vs spec verified (incl. search-by-ids object-array - design doc's sketch
was wrong, code is right); fan-outs race-free; NULL-safe scans; ctx propagation; nil-flag safe.

## Convergence: round 1 fix batch dispatched (all 5 lenses complete)

## Autofix summary
30+ findings autofixed in-place in 1 round across commits ae23606 (security: trimpath
rebuild, anchored .gitignore) and d4feb7b (correctness A1-A6, security B1, maintainability
C1-C9, auth prose D1, docs E1-E14; 18 files, +234/-268). research.json updated on disk
(lives at run root, outside the repo).

## Surface-to-user findings
None outstanding. The single judgment call surfaced via disclosure instead: absorbed-row 24's
"X-RateLimit-* surfaced in doctor" half-feature was descoped (docs no longer claim it;
recorded as Known Gap in the shipcheck report; retro candidate filed for an auth-mode-aware
rate-limit panel).

## Convergence outcome
Findings cleared at round 1 (in-scope). Round 2 = shipcheck confirmation only.

## Review path chosen
Direct reviewer-subagent dispatch via the Agent tool: correctness, security, maintainability
(always-on trio), plus Phase 4.8 SKILL reviewer and Phase 4.9 docs auditor. /review (PR-shaped)
deliberately not used - no PR exists pre-publish.
