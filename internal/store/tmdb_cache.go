package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const tmdbCacheTTL = 24 * time.Hour

func (s *Store) GetCachedTMDB(cacheKey string) (json.RawMessage, error) {
	var data []byte
	err := s.db.QueryRow(
		`SELECT response FROM tmdb_cache WHERE cache_key = ? AND cached_at > ?`,
		cacheKey, time.Now().UTC().Add(-tmdbCacheTTL),
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cached tmdb: %w", err)
	}
	return json.RawMessage(data), nil
}

func (s *Store) SetCachedTMDB(cacheKey string, data json.RawMessage) error {
	_, err := s.db.Exec(
		`INSERT INTO tmdb_cache (cache_key, response, cached_at)
		VALUES (?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
			response=excluded.response, cached_at=excluded.cached_at`,
		cacheKey, []byte(data), time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("set cached tmdb: %w", err)
	}
	return nil
}

// BackdateTMDBCache sets the cached_at timestamp for a given key (test helper).
func (s *Store) BackdateTMDBCache(cacheKey string, t time.Time) error {
	_, err := s.db.Exec(`UPDATE tmdb_cache SET cached_at = ? WHERE cache_key = ?`, t, cacheKey)
	return err
}
