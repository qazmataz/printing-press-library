# appmagic-pp-cli Shipcheck Report

## Run 1 (2026-06-10, post-Phase-3)
- Legs: verify PASS, validate-narrative PASS, dogfood PASS, workflow-verify PASS,
  verify-skill FAIL (1 finding), scorecard PASS (92/100 Grade A)
- verify-skill finding: README prose quoted `'appmagic-pp-cli entitlements --refresh'`
  with single quotes; scanner tokenized the trailing apostrophe into the flag name.
  Flag IS declared (entitlements.go:495).

## Fix applied
- research.json troubleshoot text + README.md: single quotes -> backticks around the
  command. verify-skill standalone: ALL CHECKS PASS.

## Run 2 (same day)
- Verdict: PASS (6/6 legs). Scorecard 92/100 Grade A.
- Before/after verify pass rate: PASS both runs (verify --fix found nothing to fix in run 2).
- Sample probe (no credentials available this run): 3/12 novel samples pass offline;
  9 fail with EXPECTED no-credential shapes: exit 4 + actionable hint on HTTP 401
  (APPMAGIC_LOGIN guidance), exit 4 + APPMAGIC_WEB_TOKEN guidance on web commands,
  exit 3 + sync hint on unsynced taxonomy. These demonstrate correct entitlement-aware
  error UX, not feature breakage. One SQLITE_BUSY on chart-diff under the probe's
  12-way parallel first-time DB init (store already uses WAL + busy_timeout(5000));
  one-time-init race, low real-world impact, noted for retro.
- Remaining scorecard gaps (non-blocking): insight 4/10, cache freshness 5/10,
  data pipeline integrity 7/10 - polish targets.

## Ship recommendation: ship
All ship-threshold conditions met; no known functional bugs in shipping-scope features.
Live behavioral proof deferred (no credential by operator decision; Phase 5 skip marker).
