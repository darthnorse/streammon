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
	DefaultConcurrentPeakDays = 90
	DefaultSharerWindowDays   = 30
	DefaultSharerMinIPs       = 3
)

func cutoffTime(days int) time.Time {
	if days <= 0 {
		return time.Time{}
	}
	return time.Now().UTC().AddDate(0, 0, -days)
}

type topMediaConfig struct {
	selectCol  string
	yearExpr   string
	thumbMatch string
	extraWhere string
	groupBy    string
	mediaType  models.MediaType
	errMsg     string
	itemIDCol  string
}

func (s *Store) topMedia(limit int, days int, cfg topMediaConfig) ([]models.MediaStat, error) {
	cutoff := cutoffTime(days)
	hasTimeFilter := !cutoff.IsZero()

	timeClause := ""
	subqueryTimeClause := ""
	if hasTimeFilter {
		timeClause = " AND started_at >= ?"
		subqueryTimeClause = " AND h2.started_at >= ?"
	}

	itemIDCol := cfg.itemIDCol
	if itemIDCol == "" {
		itemIDCol = "item_id"
	}

	query := fmt.Sprintf(`SELECT %s, %s, COUNT(*) as play_count,
		SUM(watched_ms) / 3600000.0 as total_hours,
		(SELECT thumb_url FROM watch_history h2
		 WHERE %s AND h2.thumb_url != ''%s ORDER BY h2.started_at DESC LIMIT 1) as thumb_url,
		(SELECT server_id FROM watch_history h2
		 WHERE %s AND h2.thumb_url != ''%s ORDER BY h2.started_at DESC LIMIT 1) as server_id,
		(SELECT %s FROM watch_history h2
		 WHERE %s AND h2.%s != ''%s ORDER BY h2.started_at DESC LIMIT 1) as item_id
	FROM watch_history
	WHERE media_type = ?%s%s
	GROUP BY %s
	ORDER BY play_count DESC
	LIMIT ?`,
		cfg.selectCol, cfg.yearExpr,
		cfg.thumbMatch, subqueryTimeClause,
		cfg.thumbMatch, subqueryTimeClause,
		itemIDCol, cfg.thumbMatch, itemIDCol, subqueryTimeClause,
		cfg.extraWhere, timeClause,
		cfg.groupBy)

	var args []any
	if hasTimeFilter {
		args = append(args, cutoff, cutoff, cutoff) // for all three subqueries
	}
	args = append(args, cfg.mediaType)
	if hasTimeFilter {
		args = append(args, cutoff) // for main WHERE
	}
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cfg.errMsg, err)
	}
	defer rows.Close()

	return scanMediaStats(rows)
}

func (s *Store) TopMovies(limit int, days int) ([]models.MediaStat, error) {
	return s.topMedia(limit, days, topMediaConfig{
		selectCol:  "title",
		yearExpr:   "year",
		thumbMatch: "h2.title = watch_history.title AND h2.year = watch_history.year",
		extraWhere: "",
		groupBy:    "title, year",
		mediaType:  models.MediaTypeMovie,
		errMsg:     "top movies",
	})
}

func (s *Store) TopTVShows(limit int, days int) ([]models.MediaStat, error) {
	return s.topMedia(limit, days, topMediaConfig{
		selectCol:  "grandparent_title",
		yearExpr:   "0 as year",
		thumbMatch: "h2.grandparent_title = watch_history.grandparent_title",
		extraWhere: " AND grandparent_title != ''",
		groupBy:    "grandparent_title",
		mediaType:  models.MediaTypeTV,
		errMsg:     "top tv shows",
		itemIDCol:  "grandparent_item_id",
	})
}

func scanMediaStats(rows *sql.Rows) ([]models.MediaStat, error) {
	stats := []models.MediaStat{}
	for rows.Next() {
		var stat models.MediaStat
		var totalHours sql.NullFloat64
		var thumbURL sql.NullString
		var serverID sql.NullInt64
		var itemID sql.NullString
		if err := rows.Scan(&stat.Title, &stat.Year, &stat.PlayCount, &totalHours, &thumbURL, &serverID, &itemID); err != nil {
			return nil, fmt.Errorf("scanning media stats: %w", err)
		}
		if totalHours.Valid {
			stat.TotalHours = totalHours.Float64
		}
		if thumbURL.Valid {
			stat.ThumbURL = thumbURL.String
		}
		if serverID.Valid {
			stat.ServerID = serverID.Int64
		}
		if itemID.Valid {
			stat.ItemID = itemID.String
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating media stats: %w", err)
	}
	return stats, nil
}

func (s *Store) TopUsers(limit int, days int) ([]models.UserStat, error) {
	cutoff := cutoffTime(days)
	hasTimeFilter := !cutoff.IsZero()

	query := `SELECT user_name, COUNT(*) as play_count,
		SUM(watched_ms) / 3600000.0 as total_hours
	FROM watch_history`

	var args []any
	if hasTimeFilter {
		query += ` WHERE started_at >= ?`
		args = append(args, cutoff)
	}

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
