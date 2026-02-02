package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"streammon/internal/models"
)

const geoCacheTTL = 30 * 24 * time.Hour

func (s *Store) GetCachedGeo(ip string) (*models.GeoResult, error) {
	var geo models.GeoResult
	err := s.db.QueryRow(
		`SELECT ip, lat, lng, city, country FROM ip_geo_cache
		WHERE ip = ? AND cached_at > ?`, ip, time.Now().Add(-geoCacheTTL),
	).Scan(&geo.IP, &geo.Lat, &geo.Lng, &geo.City, &geo.Country)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cached geo: %w", err)
	}
	return &geo, nil
}

func (s *Store) SetCachedGeo(geo *models.GeoResult) error {
	now := time.Now()
	_, err := s.db.Exec(
		`INSERT INTO ip_geo_cache (ip, lat, lng, city, country, cached_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET lat=?, lng=?, city=?, country=?, cached_at=?`,
		geo.IP, geo.Lat, geo.Lng, geo.City, geo.Country, now,
		geo.Lat, geo.Lng, geo.City, geo.Country, now,
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
	placeholders := strings.Repeat("?,", len(ips))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, 0, len(ips)+1)
	for _, ip := range ips {
		args = append(args, ip)
	}
	args = append(args, time.Now().Add(-geoCacheTTL))

	rows, err := s.db.Query(
		`SELECT ip, lat, lng, city, country FROM ip_geo_cache
		WHERE ip IN (`+placeholders+`) AND cached_at > ?`, args...,
	)
	if err != nil {
		return nil, fmt.Errorf("get cached geos: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*models.GeoResult, len(ips))
	for rows.Next() {
		var geo models.GeoResult
		if err := rows.Scan(&geo.IP, &geo.Lat, &geo.Lng, &geo.City, &geo.Country); err != nil {
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
