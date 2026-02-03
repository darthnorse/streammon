package store

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"streammon/internal/models"
)

const (
	geoCacheTTL    = 30 * 24 * time.Hour
	geoColumns     = `ip, lat, lng, city, country`
)

func scanGeoResult(scanner interface{ Scan(...any) error }) (models.GeoResult, error) {
	var geo models.GeoResult
	err := scanner.Scan(&geo.IP, &geo.Lat, &geo.Lng, &geo.City, &geo.Country)
	return geo, err
}

func (s *Store) GetCachedGeo(ip string) (*models.GeoResult, error) {
	geo, err := scanGeoResult(s.db.QueryRow(
		`SELECT `+geoColumns+` FROM ip_geo_cache
		WHERE ip = ? AND cached_at > ?`, ip, time.Now().UTC().Add(-geoCacheTTL),
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cached geo: %w", err)
	}
	return &geo, nil
}

func (s *Store) SetCachedGeo(geo *models.GeoResult) error {
	_, err := s.db.Exec(
		`INSERT INTO ip_geo_cache (`+geoColumns+`, cached_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET
			lat=excluded.lat, lng=excluded.lng, city=excluded.city,
			country=excluded.country, cached_at=excluded.cached_at`,
		geo.IP, geo.Lat, geo.Lng, geo.City, geo.Country, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("set cached geo: %w", err)
	}
	return nil
}

func (s *Store) GetCachedGeos(ips []string) (map[string]*models.GeoResult, error) {
	if len(ips) == 0 {
		return map[string]*models.GeoResult{}, nil
	}
	placeholders := make([]string, len(ips))
	args := make([]any, 0, len(ips)+1)
	for i, ip := range ips {
		placeholders[i] = "?"
		args = append(args, ip)
	}
	args = append(args, time.Now().UTC().Add(-geoCacheTTL))

	rows, err := s.db.Query(
		`SELECT `+geoColumns+` FROM ip_geo_cache
		WHERE ip IN (`+strings.Join(placeholders, ",")+`) AND cached_at > ?`, args...,
	)
	if err != nil {
		return nil, fmt.Errorf("get cached geos: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*models.GeoResult, len(ips))
	for rows.Next() {
		geo, err := scanGeoResult(rows)
		if err != nil {
			return nil, err
		}
		result[geo.IP] = &geo
	}
	return result, rows.Err()
}

type IPWithLastSeen struct {
	IP       string
	LastSeen time.Time
}

func (s *Store) DistinctIPsForUser(userName string) ([]IPWithLastSeen, error) {
	rows, err := s.db.Query(
		`SELECT ip_address, COALESCE(MAX(stopped_at), MAX(started_at)) as last_seen
		FROM watch_history
		WHERE user_name = ? AND ip_address != ''
		GROUP BY ip_address
		ORDER BY last_seen DESC
		LIMIT 500`,
		userName,
	)
	if err != nil {
		return nil, fmt.Errorf("distinct ips: %w", err)
	}
	defer rows.Close()

	var results []IPWithLastSeen
	for rows.Next() {
		var r IPWithLastSeen
		var lastSeenStr sql.NullString
		if err := rows.Scan(&r.IP, &lastSeenStr); err != nil {
			return nil, fmt.Errorf("scanning ip result: %w", err)
		}
		if lastSeenStr.Valid && lastSeenStr.String != "" {
			r.LastSeen = parseSQLiteTimestamp(lastSeenStr.String)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

var sqliteTimeFormats = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05",
}

func parseSQLiteTimestamp(s string) time.Time {
	for _, format := range sqliteTimeFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}
	log.Printf("failed to parse timestamp: %q", s)
	return time.Time{}
}
