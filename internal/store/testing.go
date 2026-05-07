package store

import (
	"context"
	"fmt"
)

// LibraryItemSeed is the minimal set of columns needed to seed library_items
// rows from cross-package tests. Server FK and added_at NOT NULL constraints
// are satisfied by the caller; episode_count defaults to 0 if zero.
type LibraryItemSeed struct {
	ServerID     int64
	LibraryID    string
	ItemID       string
	MediaType    string
	Title        string
	Year         int
	AddedAt      string // RFC 3339 / ISO 8601
	EpisodeCount int
}

// SeedLibraryItemsForTest inserts library_items rows directly. This is
// narrower than exposing arbitrary SQL: tests can only seed library_items,
// preserving the store-encapsulation guarantee for everything else.
//
// This file is intentionally not _test.go because Go cannot expose test-only
// helpers across package boundaries; the only escape valves are typed methods
// like this one.
func (s *Store) SeedLibraryItemsForTest(ctx context.Context, items []LibraryItemSeed) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO library_items
		   (server_id, library_id, item_id, media_type, title, year, added_at, episode_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, it := range items {
		if _, err := stmt.ExecContext(ctx,
			it.ServerID, it.LibraryID, it.ItemID, it.MediaType,
			it.Title, it.Year, it.AddedAt, it.EpisodeCount,
		); err != nil {
			return fmt.Errorf("inserting %s/%s: %w", it.MediaType, it.ItemID, err)
		}
	}
	return tx.Commit()
}
