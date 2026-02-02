package store

import (
	"database/sql"
	"fmt"
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
		WHERE ip = ? AND cached_at > ?`, ip, time.Now().Add(-geoCacheTTL),
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
		geo.IP, geo.Lat, geo.Lng, geo.City, geo.Country, time.Now(),
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
	args = append(args, time.Now().Add(-geoCacheTTL))

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

func (s *Store) DistinctIPsForUser(userName string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT ip_address FROM watch_history WHERE user_name = ? AND ip_address != ''`,
		userName,
	)
	if err != nil {
		return nil, fmt.Errorf("distinct ips: %w", err)
	}
	defer rows.Close()

	var ips []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		ips = append(ips, ip)
	}
	return ips, rows.Err()
}
