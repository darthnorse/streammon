package store

import (
	"fmt"
	"time"

	"streammon/internal/models"
)

const historyColumns = `id, server_id, user_name, media_type, title, parent_title, grandparent_title,
	year, duration_ms, watched_ms, player, platform, ip_address, started_at, stopped_at, created_at,
	season_number, episode_number`

func scanHistoryEntry(scanner interface{ Scan(...any) error }) (models.WatchHistoryEntry, error) {
	var e models.WatchHistoryEntry
	err := scanner.Scan(&e.ID, &e.ServerID, &e.UserName, &e.MediaType, &e.Title,
		&e.ParentTitle, &e.GrandparentTitle, &e.Year, &e.DurationMs, &e.WatchedMs,
		&e.Player, &e.Platform, &e.IPAddress, &e.StartedAt, &e.StoppedAt, &e.CreatedAt,
		&e.SeasonNumber, &e.EpisodeNumber)
	return e, err
}

func (s *Store) InsertHistory(entry *models.WatchHistoryEntry) error {
	result, err := s.db.Exec(
		`INSERT INTO watch_history (server_id, user_name, media_type, title, parent_title, grandparent_title,
			year, duration_ms, watched_ms, player, platform, ip_address, started_at, stopped_at,
			season_number, episode_number)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ServerID, entry.UserName, entry.MediaType, entry.Title,
		entry.ParentTitle, entry.GrandparentTitle, entry.Year,
		entry.DurationMs, entry.WatchedMs, entry.Player, entry.Platform,
		entry.IPAddress, entry.StartedAt, entry.StoppedAt,
		entry.SeasonNumber, entry.EpisodeNumber,
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

func (s *Store) ListHistory(page, perPage int, userFilter string) (*models.PaginatedResult[models.WatchHistoryEntry], error) {
	where := ""
	var args []any
	if userFilter != "" {
		where = " WHERE user_name = ?"
		args = append(args, userFilter)
	}

	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM watch_history"+where, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("counting history: %w", err)
	}

	offset := (page - 1) * perPage
	query := `SELECT ` + historyColumns + ` FROM watch_history` + where + ` ORDER BY started_at DESC LIMIT ? OFFSET ?`
	queryArgs := append(args, perPage, offset)

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("listing history: %w", err)
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
