# Phase 4.85 Output Review — appmagic-pp-cli (2026-06-10)

status: WARN (Wave B: warnings surface, do not block)

Finding (review-coverage, warning): 12 features sampled; 11 failed environment
preconditions (no credentials / no web token / unsynced taxonomy / parallel-init
SQLITE_BUSY) and per contract were not judged; the 1 passing sample (watchlist report)
exercised the empty-watchlist short-circuit and never touched the live API. Output
plausibility of real data (relevance, live formatting, ranking) is therefore unreviewed
this run. The empty-state behavior itself reviewed clean: in-band note + stderr hint,
documented alphabetical order, fetch_failures envelope present, intentional 2-day
official-data lag echoed in the window field.

Suggestion carried to README/known-gaps + Phase 5 skip context: first credentialed user
should run entitlements, sync, a non-empty watchlist report, and the snapshot commands
on two days; /printing-press-amend can mine that session for fixes.
