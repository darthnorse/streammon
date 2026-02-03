package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"streammon/internal/models"
)

const (
	// DefaultConcurrentPeakDays limits the concurrent streams calculation to recent history
	DefaultConcurrentPeakDays = 90
	// DefaultSharerWindowDays is the time window for detecting potential password sharing
	DefaultSharerWindowDays = 30
	// DefaultSharerMinIPs is the minimum unique IPs to flag as potential sharing
	DefaultSharerMinIPs = 3
)

// buildTimeFilter returns SQL clause and args for filtering by time window.
// Returns empty strings/slice if days <= 0 (all time).
func buildTimeFilter(days int) (string, []any) {
	if days <= 0 {
		return "", nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	return " AND started_at >= ?", []any{cutoff}
}

func (s *Store) TopMovies(limit int, days int) ([]models.MediaStat, error) {
	query := `SELECT title, year, COUNT(*) as play_count,
		SUM(watched_ms) / 3600000.0 as total_hours
	FROM watch_history
	WHERE media_type = ?`
	args := []any{models.MediaTypeMovie}

	clause, filterArgs := buildTimeFilter(days)
	query += clause
	args = append(args, filterArgs...)

	query += ` GROUP BY title, year ORDER BY play_count DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("top movies: %w", err)
	}
	defer rows.Close()

	return scanMediaStats(rows)
}

func (s *Store) TopTVShows(limit int, days int) ([]models.MediaStat, error) {
	query := `SELECT grandparent_title, 0 as year, COUNT(*) as play_count,
		SUM(watched_ms) / 3600000.0 as total_hours
	FROM watch_history
	WHERE media_type = ? AND grandparent_title != ''`
	args := []any{models.MediaTypeTV}

	clause, filterArgs := buildTimeFilter(days)
	query += clause
	args = append(args, filterArgs...)

	query += ` GROUP BY grandparent_title ORDER BY play_count DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("top tv shows: %w", err)
	}
	defer rows.Close()

	return scanMediaStats(rows)
}

func scanMediaStats(rows *sql.Rows) ([]models.MediaStat, error) {
	stats := []models.MediaStat{}
	for rows.Next() {
		var stat models.MediaStat
		var totalHours sql.NullFloat64
		if err := rows.Scan(&stat.Title, &stat.Year, &stat.PlayCount, &totalHours); err != nil {
			return nil, fmt.Errorf("scanning media stats: %w", err)
		}
		if totalHours.Valid {
			stat.TotalHours = totalHours.Float64
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating media stats: %w", err)
	}
	return stats, nil
}

func (s *Store) TopUsers(limit int, days int) ([]models.UserStat, error) {
	query := `SELECT user_name, COUNT(*) as play_count,
		SUM(watched_ms) / 3600000.0 as total_hours
	FROM watch_history
	WHERE 1=1`
	args := []any{}

	clause, filterArgs := buildTimeFilter(days)
	query += clause
	args = append(args, filterArgs...)

	query += ` GROUP BY user_name ORDER BY total_hours DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("top users: %w", err)
	}
	defer rows.Close()

	stats := []models.UserStat{}
	for rows.Next() {
		var stat models.UserStat
		var totalHours sql.NullFloat64
		if err := rows.Scan(&stat.UserName, &stat.PlayCount, &totalHours); err != nil {
			return nil, fmt.Errorf("scanning user stats: %w", err)
		}
		if totalHours.Valid {
			stat.TotalHours = totalHours.Float64
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user stats: %w", err)
	}
	return stats, nil
}

func (s *Store) LibraryStats() (*models.LibraryStat, error) {
	var stats models.LibraryStat
	var totalHours sql.NullFloat64

	err := s.db.QueryRow(
		`SELECT COUNT(*) as total_plays,
			SUM(watched_ms) / 3600000.0 as total_hours,
			COUNT(DISTINCT user_name) as unique_users
		FROM watch_history`,
	).Scan(&stats.TotalPlays, &totalHours, &stats.UniqueUsers)
	if err != nil {
		return nil, fmt.Errorf("library stats: %w", err)
	}
	if totalHours.Valid {
		stats.TotalHours = totalHours.Float64
	}

	err = s.db.QueryRow(
		`SELECT COUNT(DISTINCT title || '|' || COALESCE(year, 0))
		FROM watch_history
		WHERE media_type = ?`,
		models.MediaTypeMovie,
	).Scan(&stats.UniqueMovies)
	if err != nil {
		return nil, fmt.Errorf("unique movies: %w", err)
	}

	err = s.db.QueryRow(
		`SELECT COUNT(DISTINCT grandparent_title)
		FROM watch_history
		WHERE media_type = ? AND grandparent_title != ''`,
		models.MediaTypeTV,
	).Scan(&stats.UniqueTVShows)
	if err != nil {
		return nil, fmt.Errorf("unique tv shows: %w", err)
	}

	return &stats, nil
}

func (s *Store) ConcurrentStreamsPeak() (int, time.Time, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -DefaultConcurrentPeakDays)
	rows, err := s.db.Query(
		`SELECT started_at, stopped_at FROM watch_history WHERE started_at >= ?`,
		cutoff,
	)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("concurrent streams: %w", err)
	}
	defer rows.Close()

	type event struct {
		t     time.Time
		delta int
	}
	var events []event

	for rows.Next() {
		var start, stop time.Time
		if err := rows.Scan(&start, &stop); err != nil {
			return 0, time.Time{}, fmt.Errorf("scanning concurrent streams: %w", err)
		}
		events = append(events, event{t: start, delta: 1})
		events = append(events, event{t: stop, delta: -1})
	}
	if err := rows.Err(); err != nil {
		return 0, time.Time{}, fmt.Errorf("iterating concurrent streams: %w", err)
	}

	if len(events) == 0 {
		return 0, time.Time{}, nil
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].t.Equal(events[j].t) {
			return events[i].delta > events[j].delta
		}
		return events[i].t.Before(events[j].t)
	})

	var peak, current int
	var peakTime time.Time
	for _, ev := range events {
		current += ev.delta
		if current > peak {
			peak = current
			peakTime = ev.t
		}
	}

	return peak, peakTime, nil
}

func (s *Store) AllWatchLocations() ([]models.GeoResult, error) {
	rows, err := s.db.Query(
		`SELECT g.lat, g.lng, g.city, g.country, COALESCE(MAX(g.isp), '') as isp,
			COALESCE(GROUP_CONCAT(DISTINCT h.user_name), '') as users
		FROM watch_history h
		JOIN ip_geo_cache g ON h.ip_address = g.ip
		WHERE h.ip_address != ''
		GROUP BY g.lat, g.lng, g.city, g.country
		ORDER BY g.country, g.city`,
	)
	if err != nil {
		return nil, fmt.Errorf("watch locations: %w", err)
	}
	defer rows.Close()

	results := []models.GeoResult{}
	for rows.Next() {
		var geo models.GeoResult
		var usersStr string
		if err := rows.Scan(&geo.Lat, &geo.Lng, &geo.City, &geo.Country, &geo.ISP, &usersStr); err != nil {
			return nil, fmt.Errorf("scanning watch location: %w", err)
		}
		if usersStr != "" {
			geo.Users = strings.Split(usersStr, ",")
		}
		results = append(results, geo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating watch locations: %w", err)
	}
	return results, nil
}

func (s *Store) PotentialSharers(minIPs int, windowDays int) ([]models.SharerAlert, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -windowDays)
	rows, err := s.db.Query(
		`SELECT user_name, COUNT(DISTINCT ip_address) as unique_ips,
			MAX(COALESCE(stopped_at, started_at)) as last_seen
		FROM watch_history
		WHERE started_at >= ?
		AND ip_address != ''
		GROUP BY user_name
		HAVING unique_ips >= ?
		ORDER BY unique_ips DESC`,
		cutoff, minIPs,
	)
	if err != nil {
		return nil, fmt.Errorf("potential sharers: %w", err)
	}
	defer rows.Close()

	var alerts []models.SharerAlert
	var userNames []string
	for rows.Next() {
		var alert models.SharerAlert
		var lastSeenStr sql.NullString
		if err := rows.Scan(&alert.UserName, &alert.UniqueIPs, &lastSeenStr); err != nil {
			return nil, fmt.Errorf("scanning sharer alert: %w", err)
		}
		if lastSeenStr.Valid {
			if t := parseSQLiteTimestamp(lastSeenStr.String); !t.IsZero() {
				alert.LastSeen = t.Format(time.RFC3339)
			}
		}
		alert.Locations = []string{}
		alerts = append(alerts, alert)
		userNames = append(userNames, alert.UserName)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating sharer alerts: %w", err)
	}

	if len(alerts) == 0 {
		return []models.SharerAlert{}, nil
	}

	locationMap, err := s.getLocationsForUsers(userNames, cutoff)
	if err != nil {
		return nil, err
	}
	for i := range alerts {
		if locs, ok := locationMap[alerts[i].UserName]; ok {
			alerts[i].Locations = locs
		}
	}

	return alerts, nil
}

func (s *Store) getLocationsForUsers(userNames []string, cutoff time.Time) (map[string][]string, error) {
	if len(userNames) == 0 {
		return map[string][]string{}, nil
	}

	placeholders := make([]string, len(userNames))
	args := make([]any, 0, len(userNames)+1)
	args = append(args, cutoff)
	for i, name := range userNames {
		placeholders[i] = "?"
		args = append(args, name)
	}

	query := `SELECT h.user_name, g.city || ', ' || g.country as location
		FROM watch_history h
		JOIN ip_geo_cache g ON h.ip_address = g.ip
		WHERE h.started_at >= ?
		AND h.user_name IN (` + strings.Join(placeholders, ",") + `)
		AND g.city != ''
		GROUP BY h.user_name, location`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch user locations: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var userName, location string
		if err := rows.Scan(&userName, &location); err != nil {
			return nil, fmt.Errorf("scanning user location: %w", err)
		}
		result[userName] = append(result[userName], location)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user locations: %w", err)
	}

	return result, nil
}
