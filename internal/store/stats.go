package store

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"streammon/internal/models"
)

func (s *Store) TopMovies(limit int) ([]models.MediaStat, error) {
	rows, err := s.db.Query(
		`SELECT title, year, COUNT(*) as play_count,
			SUM(watched_ms) / 3600000.0 as total_hours
		FROM watch_history
		WHERE media_type = ?
		GROUP BY title, year
		ORDER BY play_count DESC
		LIMIT ?`,
		models.MediaTypeMovie, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("top movies: %w", err)
	}
	defer rows.Close()

	return scanMediaStats(rows)
}

func (s *Store) TopTVShows(limit int) ([]models.MediaStat, error) {
	rows, err := s.db.Query(
		`SELECT grandparent_title, 0 as year, COUNT(*) as play_count,
			SUM(watched_ms) / 3600000.0 as total_hours
		FROM watch_history
		WHERE media_type = ? AND grandparent_title != ''
		GROUP BY grandparent_title
		ORDER BY play_count DESC
		LIMIT ?`,
		models.MediaTypeTV, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("top tv shows: %w", err)
	}
	defer rows.Close()

	return scanMediaStats(rows)
}

func scanMediaStats(rows *sql.Rows) ([]models.MediaStat, error) {
	var stats []models.MediaStat
	for rows.Next() {
		var stat models.MediaStat
		var totalHours sql.NullFloat64
		if err := rows.Scan(&stat.Title, &stat.Year, &stat.PlayCount, &totalHours); err != nil {
			return nil, err
		}
		if totalHours.Valid {
			stat.TotalHours = totalHours.Float64
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if stats == nil {
		stats = []models.MediaStat{}
	}
	return stats, nil
}

func (s *Store) TopUsers(limit int) ([]models.UserStat, error) {
	rows, err := s.db.Query(
		`SELECT user_name, COUNT(*) as play_count,
			SUM(watched_ms) / 3600000.0 as total_hours
		FROM watch_history
		GROUP BY user_name
		ORDER BY total_hours DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("top users: %w", err)
	}
	defer rows.Close()

	var stats []models.UserStat
	for rows.Next() {
		var stat models.UserStat
		var totalHours sql.NullFloat64
		if err := rows.Scan(&stat.UserName, &stat.PlayCount, &totalHours); err != nil {
			return nil, err
		}
		if totalHours.Valid {
			stat.TotalHours = totalHours.Float64
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if stats == nil {
		stats = []models.UserStat{}
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
	rows, err := s.db.Query(
		`SELECT started_at, stopped_at FROM watch_history`,
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
			return 0, time.Time{}, err
		}
		events = append(events, event{t: start, delta: 1})
		events = append(events, event{t: stop, delta: -1})
	}
	if err := rows.Err(); err != nil {
		return 0, time.Time{}, err
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
		`SELECT DISTINCT g.ip, g.lat, g.lng, g.city, g.country
		FROM watch_history h
		JOIN ip_geo_cache g ON h.ip_address = g.ip
		WHERE h.ip_address != ''`,
	)
	if err != nil {
		return nil, fmt.Errorf("watch locations: %w", err)
	}
	defer rows.Close()

	var results []models.GeoResult
	for rows.Next() {
		geo, err := scanGeoResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, geo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []models.GeoResult{}
	}
	return results, nil
}

func (s *Store) PotentialSharers(minIPs int, windowDays int) ([]models.SharerAlert, error) {
	rows, err := s.db.Query(
		`SELECT user_name, COUNT(DISTINCT ip_address) as unique_ips,
			MAX(COALESCE(stopped_at, started_at)) as last_seen
		FROM watch_history
		WHERE started_at > datetime('now', ?)
		AND ip_address != ''
		GROUP BY user_name
		HAVING unique_ips >= ?
		ORDER BY unique_ips DESC`,
		fmt.Sprintf("-%d days", windowDays), minIPs,
	)
	if err != nil {
		return nil, fmt.Errorf("potential sharers: %w", err)
	}
	defer rows.Close()

	var alerts []models.SharerAlert
	for rows.Next() {
		var alert models.SharerAlert
		var lastSeen sql.NullString
		if err := rows.Scan(&alert.UserName, &alert.UniqueIPs, &lastSeen); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			alert.LastSeen = lastSeen.String
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range alerts {
		locs, err := s.getLocationsForUser(alerts[i].UserName, windowDays)
		if err != nil {
			return nil, err
		}
		alerts[i].Locations = locs
	}

	if alerts == nil {
		alerts = []models.SharerAlert{}
	}
	return alerts, nil
}

func (s *Store) getLocationsForUser(userName string, windowDays int) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT g.city || ', ' || g.country as location
		FROM watch_history h
		JOIN ip_geo_cache g ON h.ip_address = g.ip
		WHERE h.user_name = ?
		AND h.started_at > datetime('now', ?)
		AND g.city != ''`,
		userName, fmt.Sprintf("-%d days", windowDays),
	)
	if err != nil {
		return nil, fmt.Errorf("user locations: %w", err)
	}
	defer rows.Close()

	var locations []string
	for rows.Next() {
		var loc string
		if err := rows.Scan(&loc); err != nil {
			return nil, err
		}
		locations = append(locations, loc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if locations == nil {
		locations = []string{}
	}
	return locations, nil
}
