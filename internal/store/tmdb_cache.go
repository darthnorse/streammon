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

// PruneTMDBCache deletes cache rows past tmdbCacheTTL, using the same cutoff
// GetCachedTMDB reads against so a row is never unreadable without also being
// eligible for deletion.
func (s *Store) PruneTMDBCache() (int64, error) {
	result, err := s.db.Exec(
		`DELETE FROM tmdb_cache WHERE cached_at <= ?`,
		time.Now().UTC().Add(-tmdbCacheTTL),
	)
	if err != nil {
		return 0, fmt.Errorf("pruning tmdb cache: %w", err)
	}
	return result.RowsAffected()
}

// BackdateTMDBCache sets the cached_at timestamp for a given key (test helper).
func (s *Store) BackdateTMDBCache(cacheKey string, t time.Time) error {
	_, err := s.db.Exec(`UPDATE tmdb_cache SET cached_at = ? WHERE cache_key = ?`, t, cacheKey)
	return err
}

// CountTMDBCacheRows returns the total row count in tmdb_cache, ignoring TTL
// (test helper). Unlike GetCachedTMDB, this proves whether a row was actually
// deleted rather than merely being past its read-time TTL cutoff.
func (s *Store) CountTMDBCacheRows() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tmdb_cache`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting tmdb cache rows: %w", err)
	}
	return n, nil
}
