package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"streammon/internal/models"
)

const libraryItemColumns = `id, server_id, library_id, item_id, media_type, title, year,
	added_at, video_resolution, file_size, episode_count, thumb_url, synced_at`

func scanLibraryItem(scanner interface{ Scan(...any) error }) (models.LibraryItemCache, error) {
	var item models.LibraryItemCache
	err := scanner.Scan(&item.ID, &item.ServerID, &item.LibraryID, &item.ItemID,
		&item.MediaType, &item.Title, &item.Year, &item.AddedAt, &item.VideoResolution,
		&item.FileSize, &item.EpisodeCount, &item.ThumbURL, &item.SyncedAt)
	return item, err
}

// UpsertLibraryItems batch inserts/updates library items
func (s *Store) UpsertLibraryItems(ctx context.Context, items []models.LibraryItemCache) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO library_items (server_id, library_id, item_id, media_type, title, year,
			added_at, video_resolution, file_size, episode_count, thumb_url, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, item_id) DO UPDATE SET
			library_id = excluded.library_id,
			media_type = excluded.media_type,
			title = excluded.title,
			year = excluded.year,
			video_resolution = excluded.video_resolution,
			file_size = excluded.file_size,
			episode_count = excluded.episode_count,
			thumb_url = excluded.thumb_url,
			synced_at = excluded.synced_at`)
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
		_, err := stmt.ExecContext(ctx, item.ServerID, item.LibraryID, item.ItemID,
			item.MediaType, item.Title, item.Year, item.AddedAt, item.VideoResolution,
			item.FileSize, item.EpisodeCount, item.ThumbURL, now)
		if err != nil {
			return count, fmt.Errorf("upsert item %s: %w", item.ItemID, err)
		}
		count++
	}

	return count, tx.Commit()
}

// GetLibraryItem returns a single cached library item
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

// ListLibraryItems returns cached items for a library
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

// GetLastSyncTime returns the most recent sync time for a library
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
	syncedAt, err := parseTimeString(syncedAtStr.String)
	if err != nil {
		return nil, fmt.Errorf("parse sync time: %w", err)
	}
	return &syncedAt, nil
}

// parseTimeString parses time strings from SQLite which may use space or T as separator.
// Times without timezone are assumed to be UTC (per CLAUDE.md: "UTC everywhere").
func parseTimeString(s string) (time.Time, error) {
	// Formats with explicit timezone - parse normally
	formatsWithTZ := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999+00:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05+00:00",
	}
	for _, f := range formatsWithTZ {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}

	// Formats without timezone - parse in UTC location explicitly
	formatsNoTZ := []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, f := range formatsNoTZ {
		if t, err := time.ParseInLocation(f, s, time.UTC); err == nil {
			return t, nil // Already UTC
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized time format: %s", s)
}

// DeleteStaleLibraryItems removes items not seen since the given time
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
		// Even with no items, we should delete stale items
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

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO library_items (server_id, library_id, item_id, media_type, title, year,
			added_at, video_resolution, file_size, episode_count, thumb_url, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, item_id) DO UPDATE SET
			library_id = excluded.library_id,
			media_type = excluded.media_type,
			title = excluded.title,
			year = excluded.year,
			video_resolution = excluded.video_resolution,
			file_size = excluded.file_size,
			episode_count = excluded.episode_count,
			thumb_url = excluded.thumb_url,
			synced_at = excluded.synced_at`)
	if err != nil {
		return 0, 0, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		if ctx.Err() != nil {
			return upserted, 0, ctx.Err()
		}
		_, err := stmt.ExecContext(ctx, item.ServerID, item.LibraryID, item.ItemID,
			item.MediaType, item.Title, item.Year, item.AddedAt, item.VideoResolution,
			item.FileSize, item.EpisodeCount, item.ThumbURL, syncTime)
		if err != nil {
			return upserted, 0, fmt.Errorf("upsert item %s: %w", item.ItemID, err)
		}
		upserted++
	}

	// Delete stale items within the same transaction
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

// CountLibraryItems returns the count of cached items for a library
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

// GetLibraryTotalSize returns the total file size in bytes for a library from cached items
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
