package store

import (
	"database/sql"
	"fmt"
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
	_, err := s.db.Exec(
		`INSERT INTO ip_geo_cache (ip, lat, lng, city, country, cached_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET lat=?, lng=?, city=?, country=?, cached_at=?`,
		geo.IP, geo.Lat, geo.Lng, geo.City, geo.Country, time.Now(),
		geo.Lat, geo.Lng, geo.City, geo.Country, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("set cached geo: %w", err)
	}
	return nil
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
