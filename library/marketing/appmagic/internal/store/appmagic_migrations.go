// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature schema for appmagic-pp-cli. Lazy-initialized by
// the novel commands that need it; kept out of the generated migration slice
// so regeneration preserves it.

package store

import (
	"context"
	"fmt"
)

// appmagicNovelTables holds the schema for the hand-built novel features:
// chart snapshots (chart-diff), soft-launch sightings (soft-launch-radar),
// the competitor watchlist (watchlist add/list/report), and the entitlement
// probe cache (entitlements).
var appmagicNovelTables = []string{
	`CREATE TABLE IF NOT EXISTS chart_snapshots (
		sort_type TEXT NOT NULL,
		store INTEGER NOT NULL,
		country TEXT NOT NULL,
		snapshot_date TEXT NOT NULL,
		rank INTEGER NOT NULL,
		united_application_id TEXT NOT NULL,
		app_name TEXT,
		publisher_name TEXT,
		value INTEGER,
		raw TEXT,
		captured_at TEXT NOT NULL,
		PRIMARY KEY (sort_type, store, country, snapshot_date, rank)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_chart_snapshots_app
		ON chart_snapshots (united_application_id, sort_type, store, country)`,
	`CREATE TABLE IF NOT EXISTS soft_launch_sightings (
		country TEXT NOT NULL,
		store INTEGER NOT NULL,
		united_application_id TEXT NOT NULL,
		app_name TEXT,
		publisher_name TEXT,
		release_date TEXT,
		first_seen TEXT NOT NULL,
		last_seen TEXT NOT NULL,
		raw TEXT,
		PRIMARY KEY (country, store, united_application_id)
	)`,
	`CREATE TABLE IF NOT EXISTS watchlist (
		united_application_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		store_application_ids TEXT NOT NULL,
		added_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS entitlement_probes (
		group_name TEXT PRIMARY KEY,
		probe_method TEXT NOT NULL,
		probe_path TEXT NOT NULL,
		http_status INTEGER NOT NULL,
		verdict TEXT NOT NULL,
		detail TEXT,
		checked_at TEXT NOT NULL
	)`,
}

// EnsureAppmagicTables creates the novel-feature tables when absent. Safe to
// call from every command invocation; CREATE TABLE IF NOT EXISTS is a no-op
// once the tables exist.
func (s *Store) EnsureAppmagicTables(ctx context.Context) error {
	for _, stmt := range appmagicNovelTables {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring appmagic novel tables: %w", err)
		}
	}
	return nil
}
