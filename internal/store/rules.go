package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"streammon/internal/models"
)

const ruleColumns = `id, name, type, enabled, config, created_at, updated_at`

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}


func scanRule(scanner interface{ Scan(...any) error }) (models.Rule, error) {
	var r models.Rule
	var enabled int
	var configJSON string
	err := scanner.Scan(&r.ID, &r.Name, &r.Type, &enabled, &configJSON, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return r, err
	}
	r.Enabled = enabled != 0
	r.Config = json.RawMessage(configJSON)
	return r, nil
}

// scanRuleRows iterates over rows and scans them into rules.
func scanRuleRows(rows *sql.Rows) ([]models.Rule, error) {
	defer rows.Close()
	rules := []models.Rule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Store) CreateRule(rule *models.Rule) error {
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}
	configJSON := "{}"
	if len(rule.Config) > 0 {
		configJSON = string(rule.Config)
	}
	result, err := s.db.Exec(`INSERT INTO rules (name, type, enabled, config) VALUES (?, ?, ?, ?)`,
		rule.Name, rule.Type, boolToInt(rule.Enabled), configJSON)
	if err != nil {
		return fmt.Errorf("creating rule: %w", err)
	}
	id, _ := result.LastInsertId()
	rule.ID = id
	return nil
}

func (s *Store) GetRule(id int64) (*models.Rule, error) {
	r, err := scanRule(s.db.QueryRow(`SELECT `+ruleColumns+` FROM rules WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("rule %d: %w", id, models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting rule: %w", err)
	}
	return &r, nil
}

func (s *Store) UpdateRule(rule *models.Rule) error {
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}
	configJSON := "{}"
	if len(rule.Config) > 0 {
		configJSON = string(rule.Config)
	}
	_, err := s.db.Exec(`UPDATE rules SET name = ?, type = ?, enabled = ?, config = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		rule.Name, rule.Type, boolToInt(rule.Enabled), configJSON, rule.ID)
	if err != nil {
		return fmt.Errorf("updating rule: %w", err)
	}
	return nil
}

func (s *Store) DeleteRule(id int64) error {
	_, err := s.db.Exec(`DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting rule: %w", err)
	}
	return nil
}

func (s *Store) ListRules() ([]models.Rule, error) {
	rows, err := s.db.Query(`SELECT ` + ruleColumns + ` FROM rules ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing rules: %w", err)
	}
	return scanRuleRows(rows)
}

func (s *Store) ListEnabledRules() ([]models.Rule, error) {
	rows, err := s.db.Query(`SELECT ` + ruleColumns + ` FROM rules WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing enabled rules: %w", err)
	}
	return scanRuleRows(rows)
}

func (s *Store) ListRulesByType(ruleType models.RuleType) ([]models.Rule, error) {
	rows, err := s.db.Query(`SELECT `+ruleColumns+` FROM rules WHERE type = ? AND enabled = 1 ORDER BY name`, ruleType)
	if err != nil {
		return nil, fmt.Errorf("listing rules by type: %w", err)
	}
	return scanRuleRows(rows)
}

const violationColumnsWithRule = `v.id, v.rule_id, r.name, r.type, v.user_name, v.severity, v.message, v.details, v.confidence_score, v.session_key, v.occurred_at, v.created_at`

func unmarshalViolationDetails(v *models.RuleViolation, detailsJSON string) {
	if detailsJSON != "" && detailsJSON != "{}" {
		if err := json.Unmarshal([]byte(detailsJSON), &v.Details); err != nil {
			log.Printf("warning: failed to unmarshal violation details (id=%d): %v", v.ID, err)
		}
	}
}

func scanViolationWithRule(scanner interface{ Scan(...any) error }) (models.RuleViolation, error) {
	var v models.RuleViolation
	var detailsJSON string
	err := scanner.Scan(&v.ID, &v.RuleID, &v.RuleName, &v.RuleType, &v.UserName, &v.Severity, &v.Message, &detailsJSON, &v.ConfidenceScore, &v.SessionKey, &v.OccurredAt, &v.CreatedAt)
	if err != nil {
		return v, err
	}
	unmarshalViolationDetails(&v, detailsJSON)
	return v, nil
}

func (s *Store) InsertViolation(v *models.RuleViolation) error {
	if err := v.Validate(); err != nil {
		return fmt.Errorf("invalid violation: %w", err)
	}
	detailsJSON := "{}"
	if v.Details != nil {
		b, _ := json.Marshal(v.Details)
		detailsJSON = string(b)
	}
	result, err := s.db.Exec(`INSERT INTO rule_violations (rule_id, user_name, severity, message, details, confidence_score, session_key, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.RuleID, v.UserName, v.Severity, v.Message, detailsJSON, v.ConfidenceScore, v.SessionKey, v.OccurredAt)
	if err != nil {
		return fmt.Errorf("inserting violation: %w", err)
	}
	id, _ := result.LastInsertId()
	v.ID = id
	return nil
}

type ViolationFilters struct {
	UserName      string
	RuleID        int64
	RuleType      models.RuleType
	Severity      models.Severity
	MinConfidence float64
	Since         time.Time
}

func (s *Store) ListViolations(page, perPage int, filters ViolationFilters) (*models.PaginatedResult[models.RuleViolation], error) {
	where := " WHERE 1=1"
	var args []any

	if filters.UserName != "" {
		where += " AND v.user_name = ?"
		args = append(args, filters.UserName)
	}
	if filters.RuleID > 0 {
		where += " AND v.rule_id = ?"
		args = append(args, filters.RuleID)
	}
	if filters.RuleType != "" {
		where += " AND r.type = ?"
		args = append(args, filters.RuleType)
	}
	if filters.Severity != "" {
		where += " AND v.severity = ?"
		args = append(args, filters.Severity)
	}
	if filters.MinConfidence > 0 {
		where += " AND v.confidence_score >= ?"
		args = append(args, filters.MinConfidence)
	}
	if !filters.Since.IsZero() {
		where += " AND v.occurred_at >= ?"
		args = append(args, filters.Since)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM rule_violations v JOIN rules r ON v.rule_id = r.id` + where
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting violations: %w", err)
	}

	offset := (page - 1) * perPage
	query := `SELECT ` + violationColumnsWithRule + ` FROM rule_violations v JOIN rules r ON v.rule_id = r.id` +
		where + ` ORDER BY v.occurred_at DESC LIMIT ? OFFSET ?`
	queryArgs := append(args, perPage, offset)

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("listing violations: %w", err)
	}
	defer rows.Close()

	items := []models.RuleViolation{}
	for rows.Next() {
		v, err := scanViolationWithRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}

	return &models.PaginatedResult[models.RuleViolation]{
		Items:   items,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

func (s *Store) GetViolationCountByUser(userName string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM rule_violations WHERE user_name = ? AND occurred_at >= ?`, userName, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting violations by user: %w", err)
	}
	return count, nil
}

func (s *Store) GetRecentViolationsForUser(userName string, limit int) ([]models.RuleViolation, error) {
	rows, err := s.db.Query(`SELECT `+violationColumnsWithRule+` FROM rule_violations v
		JOIN rules r ON v.rule_id = r.id
		WHERE v.user_name = ? ORDER BY v.occurred_at DESC LIMIT ?`, userName, limit)
	if err != nil {
		return nil, fmt.Errorf("getting recent violations: %w", err)
	}
	defer rows.Close()

	violations := []models.RuleViolation{}
	for rows.Next() {
		v, err := scanViolationWithRule(rows)
		if err != nil {
			return nil, err
		}
		violations = append(violations, v)
	}
	return violations, rows.Err()
}

const householdColumns = `id, user_name, ip_address, city, country, latitude, longitude, auto_learned, trusted, session_count, first_seen, last_seen, created_at`

func scanHousehold(scanner interface{ Scan(...any) error }) (models.HouseholdLocation, error) {
	var h models.HouseholdLocation
	var autoLearned, trusted int
	err := scanner.Scan(&h.ID, &h.UserName, &h.IPAddress, &h.City, &h.Country, &h.Latitude, &h.Longitude,
		&autoLearned, &trusted, &h.SessionCount, &h.FirstSeen, &h.LastSeen, &h.CreatedAt)
	if err != nil {
		return h, err
	}
	h.AutoLearned = autoLearned != 0
	h.Trusted = trusted != 0
	return h, nil
}

func (s *Store) UpsertHouseholdLocation(h *models.HouseholdLocation) error {
	if err := h.Validate(); err != nil {
		return fmt.Errorf("invalid household location: %w", err)
	}
	_, err := s.db.Exec(`INSERT INTO household_locations
		(user_name, ip_address, city, country, latitude, longitude, auto_learned, trusted, session_count, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_name, ip_address, city, country) DO UPDATE SET
		latitude = excluded.latitude, longitude = excluded.longitude,
		trusted = excluded.trusted, session_count = session_count + 1, last_seen = excluded.last_seen`,
		h.UserName, h.IPAddress, h.City, h.Country, h.Latitude, h.Longitude,
		boolToInt(h.AutoLearned), boolToInt(h.Trusted), h.SessionCount, h.FirstSeen, h.LastSeen)
	if err != nil {
		return fmt.Errorf("upserting household location: %w", err)
	}
	return nil
}

func (s *Store) ListHouseholdLocations(userName string) ([]models.HouseholdLocation, error) {
	rows, err := s.db.Query(`SELECT `+householdColumns+` FROM household_locations
		WHERE user_name = ? ORDER BY last_seen DESC`, userName)
	if err != nil {
		return nil, fmt.Errorf("listing household locations: %w", err)
	}
	defer rows.Close()

	locations := []models.HouseholdLocation{}
	for rows.Next() {
		h, err := scanHousehold(rows)
		if err != nil {
			return nil, err
		}
		locations = append(locations, h)
	}
	return locations, rows.Err()
}

func (s *Store) ListTrustedHouseholdLocations(userName string) ([]models.HouseholdLocation, error) {
	rows, err := s.db.Query(`SELECT `+householdColumns+` FROM household_locations
		WHERE user_name = ? AND trusted = 1 ORDER BY last_seen DESC`, userName)
	if err != nil {
		return nil, fmt.Errorf("listing trusted household locations: %w", err)
	}
	defer rows.Close()

	locations := []models.HouseholdLocation{}
	for rows.Next() {
		h, err := scanHousehold(rows)
		if err != nil {
			return nil, err
		}
		locations = append(locations, h)
	}
	return locations, rows.Err()
}

func (s *Store) UpdateHouseholdTrusted(id int64, trusted bool) error {
	_, err := s.db.Exec(`UPDATE household_locations SET trusted = ? WHERE id = ?`, boolToInt(trusted), id)
	if err != nil {
		return fmt.Errorf("updating household trusted: %w", err)
	}
	return nil
}

func (s *Store) DeleteHouseholdLocation(id int64) error {
	_, err := s.db.Exec(`DELETE FROM household_locations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting household location: %w", err)
	}
	return nil
}

// AutoLearnHouseholdLocation checks if an IP has been used enough times by a user
// to be automatically added as a household location. Returns true if a new location was created.
func (s *Store) AutoLearnHouseholdLocation(userName, ipAddress string, minSessions int) (bool, error) {
	if ipAddress == "" {
		return false, nil
	}

	var city, country string
	var lat, lng sql.NullFloat64
	err := s.db.QueryRow(`SELECT city, country, lat, lng FROM ip_geo_cache WHERE ip = ?`, ipAddress).Scan(&city, &country, &lat, &lng)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("getting geo data: %w", err)
	}

	// Try to update an existing household location matching the full unique key
	result, err := s.db.Exec(`UPDATE household_locations SET session_count = session_count + 1, last_seen = CURRENT_TIMESTAMP
		WHERE user_name = ? AND ip_address = ? AND city = ? AND country = ?`,
		userName, ipAddress, city, country)
	if err != nil {
		return false, fmt.Errorf("updating household location: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		return false, nil
	}

	var sessionCount int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM watch_history WHERE user_name = ? AND ip_address = ?`,
		userName, ipAddress).Scan(&sessionCount)
	if err != nil {
		return false, fmt.Errorf("counting sessions: %w", err)
	}

	if sessionCount < minSessions {
		return false, nil
	}

	var firstSeenStr, lastSeenStr string
	err = s.db.QueryRow(`SELECT MIN(started_at), MAX(COALESCE(stopped_at, started_at)) FROM watch_history WHERE user_name = ? AND ip_address = ?`,
		userName, ipAddress).Scan(&firstSeenStr, &lastSeenStr)
	if err != nil {
		return false, fmt.Errorf("getting time range: %w", err)
	}
	firstSeen, err := parseSQLiteTime(firstSeenStr)
	if err != nil {
		log.Printf("warning: failed to parse first_seen time %q for user %s: %v", firstSeenStr, userName, err)
	}
	lastSeen, err := parseSQLiteTime(lastSeenStr)
	if err != nil {
		log.Printf("warning: failed to parse last_seen time %q for user %s: %v", lastSeenStr, userName, err)
	}
	if firstSeen.IsZero() {
		firstSeen = time.Now().UTC()
	}
	if lastSeen.IsZero() {
		lastSeen = firstSeen
	}

	result, err = s.db.Exec(`INSERT OR IGNORE INTO household_locations
		(user_name, ip_address, city, country, latitude, longitude, auto_learned, trusted, session_count, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, 1, 0, ?, ?, ?)`,
		userName, ipAddress, city, country, lat.Float64, lng.Float64, sessionCount, firstSeen, lastSeen)
	if err != nil {
		return false, fmt.Errorf("creating household location: %w", err)
	}

	rowsAffected, _ = result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("auto-learned household location for user %s from IP %s (%d sessions)", userName, ipAddress, sessionCount)
		return true, nil
	}

	return false, nil
}

// CalculateAllHouseholdLocations scans watch_history for all user/IP combinations
// with at least minSessions and auto-learns them as household locations.
// Returns the number of new locations created.
func (s *Store) CalculateAllHouseholdLocations(ctx context.Context, minSessions int) (int, error) {
	if minSessions <= 0 {
		minSessions = 10
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT user_name, ip_address, COUNT(*) as session_count
		FROM watch_history
		WHERE ip_address != ''
		GROUP BY user_name, ip_address
		HAVING COUNT(*) >= ?`, minSessions)
	if err != nil {
		return 0, fmt.Errorf("querying user/IP combinations: %w", err)
	}
	defer rows.Close()

	var created int
	for rows.Next() {
		var userName, ipAddress string
		var sessionCount int
		if err := rows.Scan(&userName, &ipAddress, &sessionCount); err != nil {
			continue
		}

		wasCreated, err := s.AutoLearnHouseholdLocation(userName, ipAddress, minSessions)
		if err != nil {
			log.Printf("failed to auto-learn household for %s/%s: %v", userName, ipAddress, err)
			continue
		}
		if wasCreated {
			created++
		}
	}

	if err := rows.Err(); err != nil {
		return created, err
	}

	s.BackfillHouseholdGeo()
	return created, nil
}

// BackfillHouseholdGeo updates household locations that have an IP but no city
// with geo data from the cache, merging duplicates where a city-row already exists.
func (s *Store) BackfillHouseholdGeo() {
	// Phase 1: Merge duplicates — absorb empty-city rows into resolved-city rows.
	// Uses SUM/MIN aggregates to handle multiple empty-city rows per user/IP.
	result, err := s.db.Exec(`
		UPDATE household_locations SET
			session_count = session_count + (
				SELECT COALESCE(SUM(e.session_count), 0)
				FROM household_locations e
				WHERE e.user_name = household_locations.user_name
				  AND e.ip_address = household_locations.ip_address
				  AND e.city = '' AND e.id != household_locations.id
			),
			first_seen = MIN(first_seen, (
				SELECT MIN(e.first_seen) FROM household_locations e
				WHERE e.user_name = household_locations.user_name
				  AND e.ip_address = household_locations.ip_address
				  AND e.city = '' AND e.id != household_locations.id
			))
		WHERE city != '' AND EXISTS (
			SELECT 1 FROM household_locations e
			WHERE e.user_name = household_locations.user_name
			  AND e.ip_address = household_locations.ip_address
			  AND e.city = '' AND e.id != household_locations.id
		)`)
	if err != nil {
		log.Printf("backfill household geo merge: %v", err)
	} else if n, _ := result.RowsAffected(); n > 0 {
		if _, err := s.db.Exec(`
			DELETE FROM household_locations WHERE city = '' AND ip_address != ''
			  AND EXISTS (
				SELECT 1 FROM household_locations h2
				WHERE h2.user_name = household_locations.user_name
				  AND h2.ip_address = household_locations.ip_address
				  AND h2.city != '' AND h2.id != household_locations.id
			  )`); err != nil {
			log.Printf("backfill household geo delete: %v", err)
		} else {
			log.Printf("merged %d duplicate household locations", n)
		}
	}

	// Phase 2: Backfill remaining empty-city rows from geo cache.
	// Collects IDs first to avoid holding an open cursor during writes.
	type backfillRow struct {
		id                int64
		city, country     string
		lat, lng          float64
	}
	rows, err := s.db.Query(`
		SELECT h.id, g.city, g.country, g.lat, g.lng
		FROM household_locations h
		JOIN ip_geo_cache g ON h.ip_address = g.ip
		WHERE h.ip_address != '' AND h.city = '' AND g.city != ''`)
	if err != nil {
		log.Printf("backfill household geo query: %v", err)
		return
	}
	var pending []backfillRow
	for rows.Next() {
		var r backfillRow
		if err := rows.Scan(&r.id, &r.city, &r.country, &r.lat, &r.lng); err != nil {
			continue
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("backfill household geo iterate: %v", err)
	}

	var updated int
	for _, r := range pending {
		res, err := s.db.Exec(`UPDATE OR IGNORE household_locations SET city = ?, country = ?, latitude = ?, longitude = ? WHERE id = ?`,
			r.city, r.country, r.lat, r.lng, r.id)
		if err != nil {
			log.Printf("backfill household geo update id=%d: %v", r.id, err)
			continue
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if _, delErr := s.db.Exec(`DELETE FROM household_locations WHERE id = ?`, r.id); delErr != nil {
				log.Printf("backfill household geo delete orphan id=%d: %v", r.id, delErr)
			}
		} else {
			updated++
		}
	}
	if updated > 0 {
		log.Printf("backfilled geo data for %d household locations", updated)
	}
}

func (s *Store) GetUserTrustScore(userName string) (*models.UserTrustScore, error) {
	var ts models.UserTrustScore
	err := s.db.QueryRow(`SELECT user_name, score, violation_count, last_violation_at, updated_at
		FROM user_trust_scores WHERE user_name = ?`, userName).Scan(
		&ts.UserName, &ts.Score, &ts.ViolationCount, &ts.LastViolationAt, &ts.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return &models.UserTrustScore{
			UserName:       userName,
			Score:          100,
			ViolationCount: 0,
			UpdatedAt:      time.Now().UTC(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting trust score: %w", err)
	}
	return &ts, nil
}

func (s *Store) UpsertTrustScore(ts *models.UserTrustScore) error {
	_, err := s.db.Exec(`INSERT INTO user_trust_scores (user_name, score, violation_count, last_violation_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_name) DO UPDATE SET
		score = excluded.score, violation_count = excluded.violation_count,
		last_violation_at = excluded.last_violation_at, updated_at = CURRENT_TIMESTAMP`,
		ts.UserName, ts.Score, ts.ViolationCount, ts.LastViolationAt)
	if err != nil {
		return fmt.Errorf("upserting trust score: %w", err)
	}
	return nil
}

func (s *Store) DecrementTrustScore(userName string, amount int, violationTime time.Time) error {
	_, err := s.db.Exec(`INSERT INTO user_trust_scores (user_name, score, violation_count, last_violation_at, updated_at)
		VALUES (?, ?, 1, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_name) DO UPDATE SET
		score = MAX(0, score - ?), violation_count = violation_count + 1,
		last_violation_at = ?, updated_at = CURRENT_TIMESTAMP`,
		userName, 100-amount, violationTime, amount, violationTime)
	if err != nil {
		return fmt.Errorf("decrementing trust score: %w", err)
	}
	return nil
}

const channelColumns = `id, name, channel_type, config, enabled, created_at, updated_at`

func scanChannel(scanner interface{ Scan(...any) error }) (models.NotificationChannel, error) {
	var c models.NotificationChannel
	var enabled int
	var configJSON string
	err := scanner.Scan(&c.ID, &c.Name, &c.ChannelType, &configJSON, &enabled, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	c.Enabled = enabled != 0
	c.Config = json.RawMessage(configJSON)
	return c, nil
}

func (s *Store) CreateNotificationChannel(c *models.NotificationChannel) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid channel: %w", err)
	}
	result, err := s.db.Exec(`INSERT INTO notification_channels (name, channel_type, config, enabled) VALUES (?, ?, ?, ?)`,
		c.Name, c.ChannelType, string(c.Config), boolToInt(c.Enabled))
	if err != nil {
		return fmt.Errorf("creating channel: %w", err)
	}
	id, _ := result.LastInsertId()
	c.ID = id
	return nil
}

func (s *Store) GetNotificationChannel(id int64) (*models.NotificationChannel, error) {
	c, err := scanChannel(s.db.QueryRow(`SELECT `+channelColumns+` FROM notification_channels WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("channel %d: %w", id, models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting channel: %w", err)
	}
	return &c, nil
}

func (s *Store) UpdateNotificationChannel(c *models.NotificationChannel) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid channel: %w", err)
	}
	_, err := s.db.Exec(`UPDATE notification_channels SET name = ?, channel_type = ?, config = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		c.Name, c.ChannelType, string(c.Config), boolToInt(c.Enabled), c.ID)
	if err != nil {
		return fmt.Errorf("updating channel: %w", err)
	}
	return nil
}

func (s *Store) DeleteNotificationChannel(id int64) error {
	_, err := s.db.Exec(`DELETE FROM notification_channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting channel: %w", err)
	}
	return nil
}

func (s *Store) ListNotificationChannels() ([]models.NotificationChannel, error) {
	rows, err := s.db.Query(`SELECT ` + channelColumns + ` FROM notification_channels ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing channels: %w", err)
	}
	defer rows.Close()

	channels := []models.NotificationChannel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

func (s *Store) ListEnabledNotificationChannels() ([]models.NotificationChannel, error) {
	rows, err := s.db.Query(`SELECT ` + channelColumns + ` FROM notification_channels WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing enabled channels: %w", err)
	}
	defer rows.Close()

	channels := []models.NotificationChannel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

func (s *Store) LinkRuleToChannel(ruleID, channelID int64) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO rule_notifications (rule_id, channel_id) VALUES (?, ?)`, ruleID, channelID)
	if err != nil {
		return fmt.Errorf("linking rule to channel: %w", err)
	}
	return nil
}

func (s *Store) UnlinkRuleFromChannel(ruleID, channelID int64) error {
	_, err := s.db.Exec(`DELETE FROM rule_notifications WHERE rule_id = ? AND channel_id = ?`, ruleID, channelID)
	if err != nil {
		return fmt.Errorf("unlinking rule from channel: %w", err)
	}
	return nil
}

func (s *Store) GetChannelsForRule(ruleID int64) ([]models.NotificationChannel, error) {
	rows, err := s.db.Query(`SELECT `+channelColumns+` FROM notification_channels c
		JOIN rule_notifications rn ON c.id = rn.channel_id
		WHERE rn.rule_id = ? AND c.enabled = 1 ORDER BY c.name`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("getting channels for rule: %w", err)
	}
	defer rows.Close()

	channels := []models.NotificationChannel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

// ViolationExistsRecent checks if a recent violation exists.
// If sessionKey is provided, it checks for an existing violation with that session key (ignoring time).
// Otherwise, it falls back to time-based deduplication using the within duration.
func (s *Store) ViolationExistsRecent(ruleID int64, userName, sessionKey string, within time.Duration) (bool, error) {
	var exists bool
	var err error

	if sessionKey != "" {
		// Session-based deduplication: check if this exact session already has a violation
		err = s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM rule_violations WHERE rule_id = ? AND user_name = ? AND session_key = ? LIMIT 1)`,
			ruleID, userName, sessionKey).Scan(&exists)
	} else {
		// Time-based deduplication fallback
		since := time.Now().UTC().Add(-within)
		err = s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM rule_violations WHERE rule_id = ? AND user_name = ? AND occurred_at >= ? LIMIT 1)`,
			ruleID, userName, since).Scan(&exists)
	}

	if err != nil {
		return false, fmt.Errorf("checking recent violation: %w", err)
	}
	return exists, nil
}

func (s *Store) InsertViolationWithTx(ctx context.Context, v *models.RuleViolation, trustDecrement int) error {
	if err := v.Validate(); err != nil {
		return fmt.Errorf("invalid violation: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	detailsJSON := "{}"
	if v.Details != nil {
		b, _ := json.Marshal(v.Details)
		detailsJSON = string(b)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO rule_violations (rule_id, user_name, severity, message, details, confidence_score, session_key, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.RuleID, v.UserName, v.Severity, v.Message, detailsJSON, v.ConfidenceScore, v.SessionKey, v.OccurredAt)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil
		}
		return fmt.Errorf("inserting violation: %w", err)
	}
	id, _ := result.LastInsertId()
	v.ID = id

	if trustDecrement > 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO user_trust_scores (user_name, score, violation_count, last_violation_at, updated_at)
			VALUES (?, ?, 1, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(user_name) DO UPDATE SET
			score = MAX(0, score - ?), violation_count = violation_count + 1,
			last_violation_at = ?, updated_at = CURRENT_TIMESTAMP`,
			v.UserName, 100-trustDecrement, v.OccurredAt, trustDecrement, v.OccurredAt)
		if err != nil {
			return fmt.Errorf("updating trust score: %w", err)
		}
	}

	return tx.Commit()
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "UNIQUE constraint failed")
}
