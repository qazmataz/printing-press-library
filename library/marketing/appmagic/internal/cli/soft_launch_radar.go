// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for appmagic-pp-cli.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/appmagic/internal/store"
	"github.com/spf13/cobra"
)

// softLaunchRadarMaxCountries caps how many test markets one run scans.
const softLaunchRadarMaxCountries = 5

// softLaunchAPIRow is one parsed row of GET /tops/soft-launches:
// {store_application_id, store, release_date, current_app_downloads,
// current_app_revenue, publisher_apps_count, publisher_countries_list}.
// Rows carry no united id and no app name; names are enriched separately.
type softLaunchAPIRow struct {
	Store               int
	StoreApplicationID  string
	ReleaseDate         string
	CurrentAppDownloads int64
	Raw                 json.RawMessage
}

// softLaunchSighting is one persisted (or fixture) sighting row.
type softLaunchSighting struct {
	Country            string
	Store              int
	StoreApplicationID string
	AppName            string
	PublisherName      string
	ReleaseDate        string
	FirstSeen          string
	LastSeen           string
	Downloads          int64
}

type softLaunchFetchFailure struct {
	Country string `json:"country"`
	Error   string `json:"error"`
}

type softLaunchRadarRow struct {
	App                string `json:"app,omitempty"`
	StoreApplicationID string `json:"store_application_id"`
	Store              int    `json:"store"`
	Country            string `json:"country"`
	PublisherName      string `json:"publisher_name,omitempty"`
	ReleaseDate        string `json:"release_date,omitempty"`
	FirstSeen          string `json:"first_seen"`
	Downloads          int64  `json:"downloads"`
}

type softLaunchRadarView struct {
	Countries        []string                 `json:"countries"`
	Since            string                   `json:"since"`
	Store            int                      `json:"store"`
	ScannedCountries int                      `json:"scanned_countries"`
	Sightings        []softLaunchRadarRow     `json:"sightings"`
	FetchFailures    []softLaunchFetchFailure `json:"fetch_failures,omitempty"`
	Note             string                   `json:"note,omitempty"`
}

// mergeSightingDates implements the first-seen upsert rule: an existing
// sighting keeps its original first_seen; a new sighting gets today.
// last_seen always advances to today.
func mergeSightingDates(existingFirstSeen, today string) (firstSeen, lastSeen string) {
	if strings.TrimSpace(existingFirstSeen) != "" {
		return existingFirstSeen, today
	}
	return today, today
}

// filterSightings applies the report window: keep sightings first seen on or
// after cutoff (ISO dates compare lexicographically), optionally filtered by
// a case-insensitive publisher-name substring, ordered first_seen descending
// (ties: store_application_id ascending), capped at limit when limit > 0.
func filterSightings(rows []softLaunchSighting, cutoff, publisherSub string, limit int) []softLaunchSighting {
	out := make([]softLaunchSighting, 0, len(rows))
	sub := strings.ToLower(strings.TrimSpace(publisherSub))
	for _, r := range rows {
		if r.FirstSeen < cutoff {
			continue
		}
		if sub != "" && !strings.Contains(strings.ToLower(r.PublisherName), sub) {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FirstSeen != out[j].FirstSeen {
			return out[i].FirstSeen > out[j].FirstSeen
		}
		return out[i].StoreApplicationID < out[j].StoreApplicationID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// parseSoftLaunchRows defensively parses the bare-array response of
// GET /tops/soft-launches, tolerating a {data:[...]} envelope.
func parseSoftLaunchRows(data json.RawMessage) []softLaunchAPIRow {
	arr := decodeObjectArray(data, "data", "items", "results")
	out := make([]softLaunchAPIRow, 0, len(arr))
	for _, m := range arr {
		row := softLaunchAPIRow{}
		if v, ok := m["store"].(float64); ok {
			row.Store = int(v)
		}
		if v, ok := m["store_application_id"].(string); ok {
			row.StoreApplicationID = v
		}
		if v, ok := m["release_date"].(string); ok {
			row.ReleaseDate = v
		}
		if v, ok := m["current_app_downloads"].(float64); ok {
			row.CurrentAppDownloads = int64(v)
		}
		if row.StoreApplicationID == "" {
			continue
		}
		if raw, err := json.Marshal(m); err == nil {
			row.Raw = raw
		}
		out = append(out, row)
	}
	return out
}

// softLaunchAppIdentity is the enrichment payload pulled from
// POST /applications/search-by-ids per (store, store_application_id).
type softLaunchAppIdentity struct {
	Name          string
	PublisherName string
}

func softLaunchIdentityKey(storeID int, storeApplicationID string) string {
	return strconv.Itoa(storeID) + "|" + storeApplicationID
}

// parseSoftLaunchIdentities extracts app names and publisher names from the
// POST /applications/search-by-ids response (bare array of application
// objects with store, store_application_id, name, publisher_name).
func parseSoftLaunchIdentities(data json.RawMessage) map[string]softLaunchAppIdentity {
	arr := decodeObjectArray(data, "data", "items", "results")
	out := make(map[string]softLaunchAppIdentity, len(arr))
	for _, m := range arr {
		sid, _ := m["store_application_id"].(string)
		if sid == "" {
			continue
		}
		storeID := 0
		if v, ok := m["store"].(float64); ok {
			storeID = int(v)
		}
		ident := softLaunchAppIdentity{}
		if v, ok := m["name"].(string); ok {
			ident.Name = v
		}
		if v, ok := m["publisher_name"].(string); ok {
			ident.PublisherName = v
		}
		out[softLaunchIdentityKey(storeID, sid)] = ident
	}
	return out
}

// upsertSoftLaunchSightings writes fetched rows into soft_launch_sightings
// keyed (country, store, store_application_id); the store_application_id is
// stored in the united_application_id column per the table schema. Returns
// the rows that were first seen this run (candidates for name enrichment).
func upsertSoftLaunchSightings(ctx context.Context, db *store.Store, country string, fallbackStore int, rows []softLaunchAPIRow, today string) ([]softLaunchSighting, error) {
	newOnes := make([]softLaunchSighting, 0)
	for _, r := range rows {
		storeID := r.Store
		if storeID == 0 {
			storeID = fallbackStore
		}
		var existingFirstSeen sql.NullString
		err := db.DB().QueryRowContext(ctx, `
			SELECT first_seen FROM soft_launch_sightings
			WHERE country = ? AND store = ? AND united_application_id = ?`,
			country, storeID, r.StoreApplicationID).Scan(&existingFirstSeen)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("reading sighting state: %w", err)
		}
		firstSeen, lastSeen := mergeSightingDates(existingFirstSeen.String, today)
		if _, err := db.DB().ExecContext(ctx, `
			INSERT INTO soft_launch_sightings
				(country, store, united_application_id, release_date, first_seen, last_seen, raw)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(country, store, united_application_id) DO UPDATE SET
				release_date = excluded.release_date,
				last_seen = excluded.last_seen,
				raw = excluded.raw`,
			country, storeID, r.StoreApplicationID, r.ReleaseDate, firstSeen, lastSeen, string(r.Raw)); err != nil {
			return nil, fmt.Errorf("upserting sighting: %w", err)
		}
		if !existingFirstSeen.Valid || strings.TrimSpace(existingFirstSeen.String) == "" {
			newOnes = append(newOnes, softLaunchSighting{
				Country:            country,
				Store:              storeID,
				StoreApplicationID: r.StoreApplicationID,
				ReleaseDate:        r.ReleaseDate,
				FirstSeen:          firstSeen,
				LastSeen:           lastSeen,
				Downloads:          r.CurrentAppDownloads,
			})
		}
	}
	return newOnes, nil
}

// enrichSoftLaunchNames resolves names for newly seen sightings via
// POST /applications/search-by-ids (body: bare array of
// {store, store_application_id} objects per the spec), capped at 50 ids.
// Best-effort: failures degrade to a stderr warning at the call site.
func enrichSoftLaunchNames(ctx context.Context, c *client.Client, db *store.Store, sightings []softLaunchSighting) error {
	if len(sightings) == 0 {
		return nil
	}
	if len(sightings) > 50 {
		sightings = sightings[:50]
	}
	body := make([]map[string]any, 0, len(sightings))
	seen := map[string]bool{}
	for _, s := range sightings {
		key := softLaunchIdentityKey(s.Store, s.StoreApplicationID)
		if seen[key] {
			continue
		}
		seen[key] = true
		body = append(body, map[string]any{
			"store":                s.Store,
			"store_application_id": s.StoreApplicationID,
		})
	}
	data, _, err := c.Post(ctx, "/applications/search-by-ids", body)
	if err != nil {
		return classifyAPIError(err, nil)
	}
	idents := parseSoftLaunchIdentities(data)
	for _, s := range sightings {
		ident, ok := idents[softLaunchIdentityKey(s.Store, s.StoreApplicationID)]
		if !ok || (ident.Name == "" && ident.PublisherName == "") {
			continue
		}
		_, _ = db.DB().ExecContext(ctx, `
			UPDATE soft_launch_sightings SET app_name = ?, publisher_name = ?
			WHERE store = ? AND united_application_id = ?`,
			ident.Name, ident.PublisherName, s.Store, s.StoreApplicationID)
	}
	return nil
}

// loadSoftLaunchSightings reads stored sightings for the requested markets.
// storeID 5 (all stores) does not filter by store; downloads come from the
// persisted raw API row.
func loadSoftLaunchSightings(ctx context.Context, db *store.Store, countries []string, storeID int) ([]softLaunchSighting, error) {
	query := `
		SELECT country, store, united_application_id, app_name, publisher_name, release_date, first_seen, last_seen, raw
		FROM soft_launch_sightings
		WHERE country IN (?` + strings.Repeat(",?", len(countries)-1) + `)`
	args := make([]any, 0, len(countries)+1)
	for _, c := range countries {
		args = append(args, c)
	}
	if storeID != 5 {
		query += ` AND store = ?`
		args = append(args, storeID)
	}
	query += ` ORDER BY first_seen DESC`
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("loading sightings: %w", err)
	}
	defer rows.Close()
	out := make([]softLaunchSighting, 0)
	for rows.Next() {
		var s softLaunchSighting
		var appName, pubName, releaseDate, raw sql.NullString
		if err := rows.Scan(&s.Country, &s.Store, &s.StoreApplicationID, &appName, &pubName, &releaseDate, &s.FirstSeen, &s.LastSeen, &raw); err != nil {
			return nil, err
		}
		s.AppName = appName.String
		s.PublisherName = pubName.String
		s.ReleaseDate = releaseDate.String
		if raw.Valid && raw.String != "" {
			var payload struct {
				CurrentAppDownloads float64 `json:"current_app_downloads"`
			}
			if json.Unmarshal([]byte(raw.String), &payload) == nil {
				s.Downloads = int64(payload.CurrentAppDownloads)
			}
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func newNovelSoftLaunchRadarCmd(flags *rootFlags) *cobra.Command {
	var (
		flagCountries string
		flagSince     string
		flagStore     int
		flagPublisher string
		flagTag       string
		flagLimit     int
		flagDB        string
		flagNoCapture bool
	)

	// pp:data-source auto
	cmd := &cobra.Command{
		Use:   "soft-launch-radar",
		Short: "Find newly detected soft-launch titles with first-seen dates per test market.",
		Long:  "Use this command to find newly detected soft-launch titles with first-seen dates per test market, optionally filtered by publisher or tag. Do NOT use it for general top-chart rank movement; use 'chart-diff' instead.",
		Example: strings.Trim(`
  appmagic-pp-cli soft-launch-radar --countries PH,CA,AU --since 30d
  appmagic-pp-cli soft-launch-radar --publisher "Dream Games" --since 14d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan the soft-launch charts per test market and report newly first-seen titles")
				return nil
			}
			since, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil || since <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since must be a positive duration like 7d, 30d, or 1w; got %q", flagSince))
			}
			if flagStore < 1 || flagStore > 5 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--store must be 1-5 (1 Google Play, 2 iPhone, 3 iPad, 4 iPhone+iPad, 5 all stores); got %d", flagStore))
			}
			if flagLimit < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be at least 1; got %d", flagLimit))
			}
			countries := make([]string, 0, softLaunchRadarMaxCountries)
			for _, c := range strings.Split(flagCountries, ",") {
				c = strings.ToUpper(strings.TrimSpace(c))
				if c != "" {
					countries = append(countries, c)
				}
			}
			if len(countries) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--countries must list at least one country code, e.g. PH,CA,AU"))
			}
			if len(countries) > softLaunchRadarMaxCountries {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: scanning the first %d of %d requested countries; run again with a narrower --countries list for the rest\n",
					softLaunchRadarMaxCountries, len(countries))
				countries = countries[:softLaunchRadarMaxCountries]
			}
			if cliutil.IsDogfoodEnv() && len(countries) > 1 {
				countries = countries[:1]
			}

			ctx := cmd.Context()
			capture := !flagNoCapture && flags.dataSource != "local"
			if !capture && snapshotMirrorMissing(cmd, flags, flagDB,
				"re-run without --no-capture (and without --data-source local) to scan the soft-launch charts first") {
				return nil
			}
			db, _, err := openSnapshotDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			today := time.Now().UTC().Format(snapshotDateLayout)
			failures := make([]softLaunchFetchFailure, 0)
			scanned := 0
			if capture {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				tagParam := ""
				if strings.TrimSpace(flagTag) != "" {
					tagID, _, err := resolveTagID(ctx, db, flagTag)
					if err != nil {
						return err
					}
					tagParam = strconv.FormatInt(tagID, 10)
				}
				dateFrom := time.Now().UTC().AddDate(0, 0, -90).Format(snapshotDateLayout)
				newSightings := make([]softLaunchSighting, 0)
				for _, country := range countries {
					params := map[string]string{
						"include_countries": country,
						"date_from":         dateFrom,
						"date_to":           today,
						"sort":              "current_app_downloads",
						"store":             strconv.Itoa(flagStore),
						"limit":             strconv.Itoa(flagLimit),
					}
					if tagParam != "" {
						params["tag_ids"] = tagParam
					}
					data, err := c.Get(ctx, "/tops/soft-launches", params)
					if err != nil {
						failures = append(failures, softLaunchFetchFailure{
							Country: country,
							Error:   classifyAPIError(err, nil).Error(),
						})
						continue
					}
					scanned++
					rows := parseSoftLaunchRows(data)
					fresh, err := upsertSoftLaunchSightings(ctx, db, country, flagStore, rows, today)
					if err != nil {
						return err
					}
					newSightings = append(newSightings, fresh...)
				}
				if len(failures) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d country fetches failed; computed over %d countries\n",
						len(failures), len(countries), scanned)
				}
				if err := enrichSoftLaunchNames(ctx, c, db, newSightings); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: name enrichment failed: %v\n", err)
				}
			}

			stored, err := loadSoftLaunchSightings(ctx, db, countries, flagStore)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-since).Format(snapshotDateLayout)
			visible := filterSightings(stored, cutoff, flagPublisher, flagLimit)

			view := softLaunchRadarView{
				Countries:        countries,
				Since:            flagSince,
				Store:            flagStore,
				ScannedCountries: scanned,
				Sightings:        make([]softLaunchRadarRow, 0, len(visible)),
				FetchFailures:    nil,
			}
			if len(failures) > 0 {
				view.FetchFailures = failures
			}
			for _, s := range visible {
				view.Sightings = append(view.Sightings, softLaunchRadarRow{
					App:                s.AppName,
					StoreApplicationID: s.StoreApplicationID,
					Store:              s.Store,
					Country:            s.Country,
					PublisherName:      s.PublisherName,
					ReleaseDate:        s.ReleaseDate,
					FirstSeen:          s.FirstSeen,
					Downloads:          s.Downloads,
				})
			}
			if len(view.Sightings) == 0 {
				view.Note = fmt.Sprintf(
					"no soft-launch sightings first seen within --since %s for countries %s; widen --since, add --countries, or raise --limit (current cap %d per country)",
					flagSince, strings.Join(countries, ","), flagLimit)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCountries, "countries", "PH,CA,AU", "Comma-separated test-market country codes to scan, e.g. PH,CA,AU (max 5 per run)")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Report sightings first seen within this window, e.g. 7d, 30d, 1w")
	cmd.Flags().IntVar(&flagStore, "store", 5, "Store id: 1 Google Play, 2 iPhone, 3 iPad, 4 iPhone+iPad, 5 all stores combined")
	cmd.Flags().StringVar(&flagPublisher, "publisher", "", "Case-insensitive substring filter on the enriched publisher name")
	cmd.Flags().StringVar(&flagTag, "tag", "", "Tag name or numeric tag id to filter fetched soft launches (resolved against the synced taxonomy)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum sightings fetched per country and reported overall")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite sightings store path (default: ~/.local/share/appmagic-pp-cli/data.db)")
	cmd.Flags().BoolVar(&flagNoCapture, "no-capture", false, "Skip scanning the charts; report only sightings already stored locally")
	return cmd
}
