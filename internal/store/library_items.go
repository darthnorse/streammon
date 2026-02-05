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
func (s *Store) GetLibraryItem(id int64) (*models.LibraryItemCache, error) {
	item, err := scanLibraryItem(s.db.QueryRow(
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
func (s *Store) ListLibraryItems(serverID int64, libraryID string) ([]models.LibraryItemCache, error) {
	rows, err := s.db.Query(
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
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetLastSyncTime returns the most recent sync time for a library
func (s *Store) GetLastSyncTime(serverID int64, libraryID string) (*time.Time, error) {
	var syncedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT MAX(synced_at) FROM library_items WHERE server_id = ? AND library_id = ?`,
		serverID, libraryID).Scan(&syncedAt)
	if err != nil {
		return nil, err
	}
	if !syncedAt.Valid {
		return nil, nil
	}
	return &syncedAt.Time, nil
}

// DeleteStaleLibraryItems removes items not seen since the given time
func (s *Store) DeleteStaleLibraryItems(serverID int64, libraryID string, before time.Time) (int64, error) {
	result, err := s.db.Exec(
		`DELETE FROM library_items WHERE server_id = ? AND library_id = ? AND synced_at < ?`,
		serverID, libraryID, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CountLibraryItems returns the count of cached items for a library
func (s *Store) CountLibraryItems(serverID int64, libraryID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM library_items WHERE server_id = ? AND library_id = ?`,
		serverID, libraryID).Scan(&count)
	return count, err
}
