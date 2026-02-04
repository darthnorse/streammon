package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"streammon/internal/models"
)

const historyColumns = `id, server_id, item_id, grandparent_item_id, user_name, media_type, title, parent_title, grandparent_title,
	year, duration_ms, watched_ms, player, platform, ip_address, started_at, stopped_at, created_at,
	season_number, episode_number, thumb_url`

const historyColumnsWithGeo = `h.id, h.server_id, h.item_id, h.grandparent_item_id, h.user_name, h.media_type, h.title, h.parent_title,
	h.grandparent_title, h.year, h.duration_ms, h.watched_ms, h.player, h.platform, h.ip_address,
	h.started_at, h.stopped_at, h.created_at, h.season_number, h.episode_number, h.thumb_url,
	COALESCE(g.city, ''), COALESCE(g.country, ''), COALESCE(g.isp, '')`

const historyInsertSQL = `INSERT INTO watch_history (server_id, item_id, grandparent_item_id, user_name, media_type, title, parent_title, grandparent_title,
	year, duration_ms, watched_ms, player, platform, ip_address, started_at, stopped_at,
	season_number, episode_number, thumb_url)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func scanHistoryEntry(scanner interface{ Scan(...any) error }) (models.WatchHistoryEntry, error) {
	var e models.WatchHistoryEntry
	err := scanner.Scan(&e.ID, &e.ServerID, &e.ItemID, &e.GrandparentItemID, &e.UserName, &e.MediaType, &e.Title,
		&e.ParentTitle, &e.GrandparentTitle, &e.Year, &e.DurationMs, &e.WatchedMs,
		&e.Player, &e.Platform, &e.IPAddress, &e.StartedAt, &e.StoppedAt, &e.CreatedAt,
		&e.SeasonNumber, &e.EpisodeNumber, &e.ThumbURL)
	return e, err
}

func scanHistoryEntryWithGeo(scanner interface{ Scan(...any) error }) (models.WatchHistoryEntry, error) {
	var e models.WatchHistoryEntry
	err := scanner.Scan(&e.ID, &e.ServerID, &e.ItemID, &e.GrandparentItemID, &e.UserName, &e.MediaType, &e.Title,
		&e.ParentTitle, &e.GrandparentTitle, &e.Year, &e.DurationMs, &e.WatchedMs,
		&e.Player, &e.Platform, &e.IPAddress, &e.StartedAt, &e.StoppedAt, &e.CreatedAt,
		&e.SeasonNumber, &e.EpisodeNumber, &e.ThumbURL, &e.City, &e.Country, &e.ISP)
	return e, err
}

func (s *Store) InsertHistory(entry *models.WatchHistoryEntry) error {
	result, err := s.db.Exec(historyInsertSQL,
		entry.ServerID, entry.ItemID, entry.GrandparentItemID, entry.UserName, entry.MediaType, entry.Title,
		entry.ParentTitle, entry.GrandparentTitle, entry.Year,
		entry.DurationMs, entry.WatchedMs, entry.Player, entry.Platform,
		entry.IPAddress, entry.StartedAt, entry.StoppedAt,
		entry.SeasonNumber, entry.EpisodeNumber, entry.ThumbURL,
	)
	if err != nil {
		return fmt.Errorf("inserting history: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	entry.ID = id
	return nil
}

func (s *Store) ListHistory(page, perPage int, userFilter, sortColumn, sortOrder string) (*models.PaginatedResult[models.WatchHistoryEntry], error) {
	where := ""
	var args []any
	if userFilter != "" {
		where = " WHERE h.user_name = ?"
		args = append(args, userFilter)
	}

	countWhere := where
	if countWhere != "" {
		countWhere = " WHERE user_name = ?"
	}
	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM watch_history"+countWhere, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("counting history: %w", err)
	}

	orderBy := "h.started_at DESC"
	if sortColumn != "" {
		orderBy = sortColumn + " " + sortOrder
	}

	offset := (page - 1) * perPage
	query := `SELECT ` + historyColumnsWithGeo + `
		FROM watch_history h
		LEFT JOIN ip_geo_cache g ON h.ip_address = g.ip` +
		where + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
	queryArgs := append(args, perPage, offset)

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("listing history: %w", err)
	}
	defer rows.Close()

	items := []models.WatchHistoryEntry{}
	for rows.Next() {
		e, err := scanHistoryEntryWithGeo(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &models.PaginatedResult[models.WatchHistoryEntry]{
		Items:   items,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, nil
}

func (s *Store) DailyWatchCounts(start, end time.Time) ([]models.DayStat, error) {
	rows, err := s.db.Query(
		`SELECT date(started_at) AS day, media_type, COUNT(*) AS cnt
		FROM watch_history
		WHERE started_at >= ? AND started_at < ?
		GROUP BY day, media_type
		ORDER BY day`, start, end,
	)
	if err != nil {
		return nil, fmt.Errorf("daily watch counts: %w", err)
	}
	defer rows.Close()

	dayMap := map[string]*models.DayStat{}
	var order []string
	for rows.Next() {
		var day string
		var mediaType models.MediaType
		var cnt int
		if err := rows.Scan(&day, &mediaType, &cnt); err != nil {
			return nil, err
		}
		if _, ok := dayMap[day]; !ok {
			dayMap[day] = &models.DayStat{Date: day}
			order = append(order, day)
		}
		switch mediaType {
		case models.MediaTypeMovie:
			dayMap[day].Movies += cnt
		case models.MediaTypeTV:
			dayMap[day].TV += cnt
		case models.MediaTypeLiveTV:
			dayMap[day].LiveTV += cnt
		case models.MediaTypeMusic:
			dayMap[day].Music += cnt
		case models.MediaTypeAudiobook:
			dayMap[day].Audiobooks += cnt
		case models.MediaTypeBook:
			dayMap[day].Books += cnt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	stats := make([]models.DayStat, 0, len(order))
	for _, d := range order {
		stats = append(stats, *dayMap[d])
	}
	return stats, nil
}

func (s *Store) HistoryForTitle(title string, limit int) ([]models.WatchHistoryEntry, error) {
	query := `SELECT ` + historyColumns + ` FROM watch_history
		WHERE title = ? OR grandparent_title = ?
		ORDER BY started_at DESC LIMIT ?`
	rows, err := s.db.Query(query, title, title, limit)
	if err != nil {
		return nil, fmt.Errorf("history for title: %w", err)
	}
	defer rows.Close()

	items := []models.WatchHistoryEntry{}
	for rows.Next() {
		e, err := scanHistoryEntry(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func (s *Store) HistoryExists(serverID int64, userName, title string, startedAt time.Time) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT 1 FROM watch_history WHERE server_id = ? AND user_name = ? AND title = ? AND started_at = ? LIMIT 1`,
		serverID, userName, title, startedAt,
	).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("checking history exists: %w", err)
	}
	return true, nil
}

func (s *Store) InsertHistoryBatch(ctx context.Context, entries []*models.WatchHistoryEntry) (inserted, skipped int, err error) {
	if len(entries) == 0 {
		return 0, 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	insertStmt, err := tx.PrepareContext(ctx, historyInsertSQL)
	if err != nil {
		return 0, 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer insertStmt.Close()

	existsStmt, err := tx.PrepareContext(ctx,
		`SELECT 1 FROM watch_history WHERE server_id = ? AND user_name = ? AND title = ? AND started_at = ? LIMIT 1`,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("prepare exists check: %w", err)
	}
	defer existsStmt.Close()

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return inserted, skipped, ctx.Err()
		default:
		}

		var exists int
		err := existsStmt.QueryRowContext(ctx, entry.ServerID, entry.UserName, entry.Title, entry.StartedAt).Scan(&exists)
		if err == nil {
			skipped++
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return inserted, skipped, fmt.Errorf("checking if entry exists: %w", err)
		}

		_, err = insertStmt.ExecContext(ctx,
			entry.ServerID, entry.ItemID, entry.GrandparentItemID, entry.UserName, entry.MediaType, entry.Title,
			entry.ParentTitle, entry.GrandparentTitle, entry.Year,
			entry.DurationMs, entry.WatchedMs, entry.Player, entry.Platform,
			entry.IPAddress, entry.StartedAt, entry.StoppedAt,
			entry.SeasonNumber, entry.EpisodeNumber, entry.ThumbURL,
		)
		if err != nil {
			return 0, 0, fmt.Errorf("inserting entry: %w", err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit tx: %w", err)
	}

	return inserted, skipped, nil
}
