// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared resolution helpers for the novel commands: app-name ->
// united application, united id -> per-store IDs, and tag-name -> tag ID from
// the synced taxonomy.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/store"

	"github.com/spf13/cobra"
)

// openSnapshotDB resolves the --db flag (empty means the canonical default
// path), opens the local SQLite store, and lazily creates the hand-authored
// novel-feature tables. Shared by the novel commands that persist state.
func openSnapshotDB(ctx context.Context, dbPath string) (*store.Store, string, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = defaultDBPath("appmagic-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, dbPath, fmt.Errorf("opening local snapshot store at %s: %w", dbPath, err)
	}
	if err := db.EnsureAppmagicTables(ctx); err != nil {
		_ = db.Close()
		return nil, dbPath, err
	}
	return db, dbPath, nil
}

// snapshotMirrorMissing implements the missing-mirror guard for the offline
// paths of the snapshot commands: when the local DB file does not exist and
// no capture is allowed, emit a stderr hint, print [] for machine consumers,
// and report true so the caller returns nil (exit 0).
func snapshotMirrorMissing(cmd *cobra.Command, flags *rootFlags, dbPath, hint string) bool {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = defaultDBPath("appmagic-pp-cli")
	}
	if _, err := os.Stat(dbPath); err == nil {
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "hint: no local snapshot store at %s; %s\n", dbPath, hint)
	if flags != nil && (flags.asJSON || flags.agent) {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return true
}

// decodeObjectArray defensively decodes a bare JSON array of objects,
// tolerating an envelope under any of the given keys so spec drift degrades
// to partial rows, not parse failures.
func decodeObjectArray(data json.RawMessage, envelopeKeys ...string) []map[string]any {
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		var env map[string]json.RawMessage
		if json.Unmarshal(data, &env) == nil {
			for _, key := range envelopeKeys {
				if raw, ok := env[key]; ok && json.Unmarshal(raw, &arr) == nil {
					break
				}
			}
		}
	}
	return arr
}

// storeForStoreAppID infers the store kind from the shape of a store
// application id: all-digit ids are App Store numeric ids (store 2, iPhone
// App Store); anything else (reversed-domain Google Play package names) is
// store 1. Single source of truth for store-id shape classification.
func storeForStoreAppID(id string) int {
	if id == "" {
		return 1
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return 1
		}
	}
	return 2
}

// unitedApp is the resolved identity novel commands work with.
type unitedApp struct {
	ID                  int64    `json:"united_application_id"`
	Name                string   `json:"name"`
	PublisherName       string   `json:"publisher_name,omitempty"`
	StoreApplicationIDs []string `json:"store_application_ids,omitempty"`
}

// resolveUnitedApp resolves a free-text app name (or a numeric united ID) to
// a united application. Name resolution uses GET /united-applications
// (prefix search); numeric input short-circuits to search-by-ids so exact IDs
// never depend on search ranking.
func resolveUnitedApp(ctx context.Context, c *client.Client, query string) (*unitedApp, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("app name or united application id is required")
	}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		apps, err := unitedAppsByIDs(ctx, c, []int64{id})
		if err != nil {
			return nil, err
		}
		if len(apps) == 0 {
			return nil, notFoundErr(fmt.Errorf("no united application with id %d", id))
		}
		return apps[0], nil
	}
	data, err := c.Get(ctx, "/united-applications", map[string]string{"search": query, "limit": "5"})
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	apps := parseUnitedApps(data)
	if len(apps) == 0 {
		return nil, notFoundErr(fmt.Errorf("no united application matched %q; try a shorter name prefix", query))
	}
	return apps[0], nil
}

// unitedAppsByIDs resolves united IDs to identities via the batch
// POST /united-applications/search-by-ids endpoint (bare-array body).
func unitedAppsByIDs(ctx context.Context, c *client.Client, ids []int64) ([]*unitedApp, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	data, _, err := c.Post(ctx, "/united-applications/search-by-ids", ids)
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	return parseUnitedApps(data), nil
}

// parseUnitedApps defensively extracts united-app identities from either a
// bare array or a {data:[...]} envelope. Field extraction tolerates missing
// keys so spec drift degrades to partial identities, not parse failures.
func parseUnitedApps(data json.RawMessage) []*unitedApp {
	arr := decodeObjectArray(data, "data", "applications", "items")
	out := make([]*unitedApp, 0, len(arr))
	for _, m := range arr {
		ua := &unitedApp{}
		switch v := m["id"].(type) {
		case float64:
			ua.ID = int64(v)
		case string:
			ua.ID, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := m["united_application_id"].(float64); ok && ua.ID == 0 {
			ua.ID = int64(v)
		}
		if v, ok := m["name"].(string); ok {
			ua.Name = v
		}
		if v, ok := m["publisher_name"].(string); ok {
			ua.PublisherName = v
		}
		if appsRaw, ok := m["applications"].([]any); ok {
			for _, a := range appsRaw {
				am, ok := a.(map[string]any)
				if !ok {
					continue
				}
				if ua.Name == "" {
					if n, ok := am["name"].(string); ok {
						ua.Name = n
					}
				}
				if ua.PublisherName == "" {
					if pn, ok := am["publisher_name"].(string); ok {
						ua.PublisherName = pn
					}
				}
				for _, key := range []string{"store_application_id", "store_id"} {
					if sid, ok := am[key].(string); ok && sid != "" {
						ua.StoreApplicationIDs = append(ua.StoreApplicationIDs, stripStorePrefix(sid))
					}
				}
				if sids, ok := am["store_ids"].([]any); ok {
					for _, s := range sids {
						if sid, ok := s.(string); ok && sid != "" {
							ua.StoreApplicationIDs = append(ua.StoreApplicationIDs, stripStorePrefix(sid))
						}
					}
				}
			}
		}
		if sids, ok := m["store_ids"].([]any); ok {
			for _, s := range sids {
				if sid, ok := s.(string); ok && sid != "" {
					ua.StoreApplicationIDs = append(ua.StoreApplicationIDs, stripStorePrefix(sid))
				}
			}
		}
		ua.StoreApplicationIDs = dedupeStrings(ua.StoreApplicationIDs)
		if ua.ID != 0 || ua.Name != "" {
			out = append(out, ua)
		}
	}
	return out
}

// stripStorePrefix turns "2_835599320" into "835599320"; bare IDs pass through.
func stripStorePrefix(id string) string {
	if i := strings.Index(id, "_"); i == 1 || i == 2 {
		prefix := id[:i]
		if _, err := strconv.Atoi(prefix); err == nil {
			return id[i+1:]
		}
	}
	return id
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// resolveTagID resolves a tag name (case-insensitive; also accepts a numeric
// tag ID) against the locally synced taxonomy. Returns a typed error naming
// the sync command when the taxonomy is missing.
func resolveTagID(ctx context.Context, db *store.Store, tag string) (int64, string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return 0, "", fmt.Errorf("tag is required")
	}
	if id, err := strconv.ParseInt(tag, 10, 64); err == nil {
		return id, tag, nil
	}
	row := db.DB().QueryRowContext(ctx, `
		SELECT id, COALESCE(json_extract(data, '$.name'), '') FROM resources
		WHERE resource_type IN ('tags', 'tag')
		  AND LOWER(COALESCE(json_extract(data, '$.name'), '')) = LOWER(?)
		LIMIT 1`, tag)
	var idStr, name string
	if err := row.Scan(&idStr, &name); err != nil {
		if err == sql.ErrNoRows {
			// Fall back to a prefix match before giving up. Escape LIKE
			// wildcards in the user input so a literal % or _ cannot widen
			// the match beyond true prefix semantics.
			escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(tag)
			row2 := db.DB().QueryRowContext(ctx, `
				SELECT id, COALESCE(json_extract(data, '$.name'), '') FROM resources
				WHERE resource_type IN ('tags', 'tag')
				  AND LOWER(COALESCE(json_extract(data, '$.name'), '')) LIKE LOWER(?) || '%' ESCAPE '\'
				ORDER BY LENGTH(COALESCE(json_extract(data, '$.name'), '')) ASC
				LIMIT 1`, escaped)
			if err2 := row2.Scan(&idStr, &name); err2 != nil {
				return 0, "", notFoundErr(fmt.Errorf("tag %q not found in local taxonomy; run: appmagic-pp-cli sync --resources tags", tag))
			}
		} else {
			return 0, "", fmt.Errorf("querying tag taxonomy: %w", err)
		}
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("tag %q has non-numeric id %q in local taxonomy", tag, idStr)
	}
	return id, name, nil
}
