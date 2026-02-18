package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"streammon/internal/models"
)

const libraryItemColumns = `id, server_id, library_id, item_id, media_type, title, year,
	added_at, last_watched_at, video_resolution, file_size, episode_count, thumb_url,
	tmdb_id, tvdb_id, imdb_id, synced_at`

const libraryItemUpsertSQL = `
	INSERT INTO library_items (server_id, library_id, item_id, media_type, title, year,
		added_at, last_watched_at, video_resolution, file_size, episode_count, thumb_url,
		tmdb_id, tvdb_id, imdb_id, synced_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(server_id, item_id) DO UPDATE SET
		library_id = excluded.library_id,
		media_type = excluded.media_type,
		title = excluded.title,
		year = excluded.year,
		last_watched_at = CASE
			WHEN excluded.last_watched_at IS NOT NULL AND
				(library_items.last_watched_at IS NULL OR excluded.last_watched_at > library_items.last_watched_at)
			THEN excluded.last_watched_at
			ELSE library_items.last_watched_at
		END,
		video_resolution = excluded.video_resolution,
		file_size = excluded.file_size,
		episode_count = excluded.episode_count,
		thumb_url = excluded.thumb_url,
		tmdb_id = excluded.tmdb_id,
		tvdb_id = excluded.tvdb_id,
		imdb_id = excluded.imdb_id,
		synced_at = excluded.synced_at`

func execLibraryItemUpsert(ctx context.Context, stmt *sql.Stmt, item models.LibraryItemCache, syncTime time.Time) error {
	_, err := stmt.ExecContext(ctx, item.ServerID, item.LibraryID, item.ItemID,
		item.MediaType, item.Title, item.Year, item.AddedAt, item.LastWatchedAt,
		item.VideoResolution, item.FileSize, item.EpisodeCount, normalizeThumbURL(item.ThumbURL),
		item.TMDBID, item.TVDBID, item.IMDBID, syncTime)
	return err
}

func scanLibraryItem(scanner interface{ Scan(...any) error }) (models.LibraryItemCache, error) {
	var item models.LibraryItemCache
	var lastWatchedAt sql.NullString
	err := scanner.Scan(&item.ID, &item.ServerID, &item.LibraryID, &item.ItemID,
		&item.MediaType, &item.Title, &item.Year, &item.AddedAt, &lastWatchedAt,
		&item.VideoResolution, &item.FileSize, &item.EpisodeCount, &item.ThumbURL,
		&item.TMDBID, &item.TVDBID, &item.IMDBID, &item.SyncedAt)
	if err != nil {
		return item, err
	}
	if lastWatchedAt.Valid && lastWatchedAt.String != "" {
		t, parseErr := parseSQLiteTime(lastWatchedAt.String)
		if parseErr == nil {
			item.LastWatchedAt = &t
		}
	}
	return item, nil
}

func (s *Store) UpsertLibraryItems(ctx context.Context, items []models.LibraryItemCache) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, libraryItemUpsertSQL)
	if err != nil {
		return 0, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	count := 0
	now := time.Now().UTC()
	for _, item := range items {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		if err := execLibraryItemUpsert(ctx, stmt, item, now); err != nil {
			return count, fmt.Errorf("upsert item %s: %w", item.ItemID, err)
		}
		count++
	}

	return count, tx.Commit()
}

func (s *Store) GetLibraryItem(ctx context.Context, id int64) (*models.LibraryItemCache, error) {
	item, err := scanLibraryItem(s.db.QueryRowContext(ctx,
		`SELECT `+libraryItemColumns+` FROM library_items WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("library item %d: %w", id, models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get library item: %w", err)
	}
	return &item, nil
}

func (s *Store) ListLibraryItems(ctx context.Context, serverID int64, libraryID string) ([]models.LibraryItemCache, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+libraryItemColumns+` FROM library_items WHERE server_id = ? AND library_id = ? ORDER BY added_at DESC`,
		serverID, libraryID)
	if err != nil {
		return nil, fmt.Errorf("list library items: %w", err)
	}
	defer rows.Close()

	var items []models.LibraryItemCache
	for rows.Next() {
		item, err := scanLibraryItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan library item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetLastSyncTime(ctx context.Context, serverID int64, libraryID string) (*time.Time, error) {
	var syncedAtStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT MAX(synced_at) FROM library_items WHERE server_id = ? AND library_id = ?`,
		serverID, libraryID).Scan(&syncedAtStr)
	if err != nil {
		return nil, fmt.Errorf("get last sync time: %w", err)
	}
	if !syncedAtStr.Valid || syncedAtStr.String == "" {
		return nil, nil
	}
	syncedAt, err := parseSQLiteTime(syncedAtStr.String)
	if err != nil {
		return nil, fmt.Errorf("parse sync time: %w", err)
	}
	return &syncedAt, nil
}

func (s *Store) DeleteStaleLibraryItems(ctx context.Context, serverID int64, libraryID string, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM library_items WHERE server_id = ? AND library_id = ? AND synced_at < ?`,
		serverID, libraryID, before)
	if err != nil {
		return 0, fmt.Errorf("delete stale library items: %w", err)
	}
	return result.RowsAffected()
}

// SyncLibraryItems atomically upserts items and deletes stale ones in a single transaction.
// This prevents race conditions between concurrent syncs for the same library.
func (s *Store) SyncLibraryItems(ctx context.Context, serverID int64, libraryID string, items []models.LibraryItemCache) (upserted int, deleted int64, err error) {
	if len(items) == 0 {
		deleted, err = s.DeleteStaleLibraryItems(ctx, serverID, libraryID, time.Now().UTC())
		return 0, deleted, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Record sync time BEFORE any operations - this is the cutoff for stale items
	syncTime := time.Now().UTC()

	stmt, err := tx.PrepareContext(ctx, libraryItemUpsertSQL)
	if err != nil {
		return 0, 0, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		if ctx.Err() != nil {
			return upserted, 0, ctx.Err()
		}
		if err := execLibraryItemUpsert(ctx, stmt, item, syncTime); err != nil {
			return upserted, 0, fmt.Errorf("upsert item %s: %w", item.ItemID, err)
		}
		upserted++
	}

	result, err := tx.ExecContext(ctx,
		`DELETE FROM library_items WHERE server_id = ? AND library_id = ? AND synced_at < ?`,
		serverID, libraryID, syncTime)
	if err != nil {
		return upserted, 0, fmt.Errorf("delete stale items: %w", err)
	}

	deleted, _ = result.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit: %w", err)
	}

	return upserted, deleted, nil
}

func (s *Store) CountLibraryItems(ctx context.Context, serverID int64, libraryID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM library_items WHERE server_id = ? AND library_id = ?`,
		serverID, libraryID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count library items: %w", err)
	}
	return count, nil
}

func (s *Store) GetLibraryTotalSize(ctx context.Context, serverID int64, libraryID string) (int64, error) {
	var total sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT SUM(file_size) FROM library_items WHERE server_id = ? AND library_id = ?`,
		serverID, libraryID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get library total size: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

func (s *Store) ListItemsForLibraries(ctx context.Context, libraries []models.RuleLibrary) ([]models.LibraryItemCache, error) {
	if len(libraries) == 0 {
		return []models.LibraryItemCache{}, nil
	}

	var clauses []string
	var args []any
	for _, lib := range libraries {
		clauses = append(clauses, "(server_id = ? AND library_id = ?)")
		args = append(args, lib.ServerID, lib.LibraryID)
	}

	query := `SELECT ` + libraryItemColumns + ` FROM library_items WHERE ` +
		strings.Join(clauses, " OR ") + ` ORDER BY added_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list items for libraries: %w", err)
	}
	defer rows.Close()

	var items []models.LibraryItemCache
	for rows.Next() {
		item, err := scanLibraryItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan library item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		return []models.LibraryItemCache{}, nil
	}
	return items, nil
}

func (s *Store) batchQueryTimes(ctx context.Context, itemIDs []int64, queryFn func(placeholders string) string, errLabel string, result map[int64]*time.Time) error {
	const batchSize = 200
	for i := 0; i < len(itemIDs); i += batchSize {
		end := i + batchSize
		if end > len(itemIDs) {
			end = len(itemIDs)
		}
		batch := itemIDs[i:end]

		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for j, id := range batch {
			placeholders[j] = "?"
			args[j] = id
		}

		rows, err := s.db.QueryContext(ctx, queryFn(strings.Join(placeholders, ",")), args...)
		if err != nil {
			return fmt.Errorf("%s: %w", errLabel, err)
		}

		for rows.Next() {
			var id int64
			var ts sql.NullString
			if err := rows.Scan(&id, &ts); err != nil {
				rows.Close()
				return fmt.Errorf("scan %s: %w", errLabel, err)
			}
			if ts.Valid && ts.String != "" {
				t, parseErr := parseSQLiteTime(ts.String)
				if parseErr == nil {
					result[id] = &t
				}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("%s rows: %w", errLabel, err)
		}
	}
	return nil
}

func (s *Store) GetCrossServerWatchTimes(ctx context.Context, itemIDs []int64) (map[int64]*time.Time, error) {
	result := make(map[int64]*time.Time)
	if len(itemIDs) == 0 {
		return result, nil
	}

	err := s.batchQueryTimes(ctx, itemIDs, func(ph string) string {
		return `SELECT t.id,
			MAX(
				MAX(COALESCE(other.last_watched_at, '')),
				COALESCE(t.last_watched_at, '')
			) as max_watched
		FROM library_items t
		LEFT JOIN library_items other ON other.id != t.id
			AND other.last_watched_at IS NOT NULL
			AND (
				(t.tmdb_id != '' AND other.tmdb_id = t.tmdb_id) OR
				(t.tvdb_id != '' AND other.tvdb_id = t.tvdb_id) OR
				(t.imdb_id != '' AND other.imdb_id = t.imdb_id)
			)
		WHERE t.id IN (` + ph + `)
		GROUP BY t.id`
	}, "cross-server watch times", result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetStreamMonWatchTimes returns the most recent watch_history activity for a batch of library items.
// For movies it matches on item_id; for TV shows it also matches on grandparent_item_id (series ID).
// Title-based fallback covers legacy data where grandparent_item_id is empty; constrained to same
// server and media_type to avoid false positives from title collisions.
func (s *Store) GetStreamMonWatchTimes(ctx context.Context, itemIDs []int64) (map[int64]*time.Time, error) {
	result := make(map[int64]*time.Time)
	if len(itemIDs) == 0 {
		return result, nil
	}

	err := s.batchQueryTimes(ctx, itemIDs, func(ph string) string {
		return `SELECT li.id, MAX(wh.stopped_at) as last_activity
			FROM library_items li
			JOIN watch_history wh ON wh.server_id = li.server_id
				AND (
					wh.item_id = li.item_id
					OR wh.grandparent_item_id = li.item_id
					OR (li.title != '' AND wh.media_type = li.media_type AND (wh.title = li.title OR wh.grandparent_title = li.title))
				)
			WHERE li.id IN (` + ph + `)
			GROUP BY li.id`
	}, "streammon watch times", result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Store) FindMatchingItems(ctx context.Context, item *models.LibraryItemCache) ([]models.LibraryItemCache, error) {
	var clauses []string
	var args []any

	if item.TMDBID != "" {
		clauses = append(clauses, "tmdb_id = ?")
		args = append(args, item.TMDBID)
	}
	if item.TVDBID != "" {
		clauses = append(clauses, "tvdb_id = ?")
		args = append(args, item.TVDBID)
	}
	if item.IMDBID != "" {
		clauses = append(clauses, "imdb_id = ?")
		args = append(args, item.IMDBID)
	}

	if len(clauses) == 0 {
		return []models.LibraryItemCache{}, nil
	}

	query := `SELECT ` + libraryItemColumns + ` FROM library_items WHERE id != ? AND server_id != ? AND media_type = ? AND (` +
		strings.Join(clauses, " OR ") + `)`
	fullArgs := append([]any{item.ID, item.ServerID, item.MediaType}, args...)

	rows, err := s.db.QueryContext(ctx, query, fullArgs...)
	if err != nil {
		return nil, fmt.Errorf("find matching items: %w", err)
	}
	defer rows.Close()

	var items []models.LibraryItemCache
	for rows.Next() {
		matched, err := scanLibraryItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan matching item: %w", err)
		}
		items = append(items, matched)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if items == nil {
		return []models.LibraryItemCache{}, nil
	}
	return items, nil
}

// GetAllLibraryTotalSizes returns total sizes for all libraries, keyed by "serverID-libraryID"
func (s *Store) GetAllLibraryTotalSizes(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT server_id, library_id, COALESCE(SUM(file_size), 0) FROM library_items GROUP BY server_id, library_id`)
	if err != nil {
		return nil, fmt.Errorf("get all library sizes: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var serverID int64
		var libraryID string
		var totalSize int64
		if err := rows.Scan(&serverID, &libraryID, &totalSize); err != nil {
			return nil, fmt.Errorf("scan library size: %w", err)
		}
		result[fmt.Sprintf("%d-%s", serverID, libraryID)] = totalSize
	}
	return result, rows.Err()
}
