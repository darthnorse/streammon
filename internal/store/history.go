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

const historyColumns = `id, server_id, item_id, grandparent_item_id, user_name, media_type, title, parent_title, grandparent_title,
	year, duration_ms, watched_ms, player, platform, ip_address, started_at, stopped_at, created_at,
	season_number, episode_number, thumb_url, video_resolution, transcode_decision,
	video_codec, audio_codec, audio_channels, bandwidth, video_decision, audio_decision,
	transcode_hw_decode, transcode_hw_encode, dynamic_range, paused_ms, watched`

const historyColumnsWithGeo = `h.id, h.server_id, h.item_id, h.grandparent_item_id, h.user_name, h.media_type, h.title, h.parent_title,
	h.grandparent_title, h.year, h.duration_ms, h.watched_ms, h.player, h.platform, h.ip_address,
	h.started_at, h.stopped_at, h.created_at, h.season_number, h.episode_number, h.thumb_url,
	h.video_resolution, h.transcode_decision,
	h.video_codec, h.audio_codec, h.audio_channels, h.bandwidth, h.video_decision, h.audio_decision,
	h.transcode_hw_decode, h.transcode_hw_encode, h.dynamic_range, h.paused_ms, h.watched,
	COALESCE(g.city, ''), COALESCE(g.country, ''), COALESCE(g.isp, '')`

const historyInsertSQL = `INSERT INTO watch_history (server_id, item_id, grandparent_item_id, user_name, media_type, title, parent_title, grandparent_title,
	year, duration_ms, watched_ms, player, platform, ip_address, started_at, stopped_at,
	season_number, episode_number, thumb_url, video_resolution, transcode_decision,
	video_codec, audio_codec, audio_channels, bandwidth, video_decision, audio_decision,
	transcode_hw_decode, transcode_hw_encode, dynamic_range, paused_ms, watched, tautulli_reference_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func scanHistoryEntry(scanner interface{ Scan(...any) error }) (models.WatchHistoryEntry, error) {
	var e models.WatchHistoryEntry
	var hwDecode, hwEncode, watched int
	err := scanner.Scan(&e.ID, &e.ServerID, &e.ItemID, &e.GrandparentItemID, &e.UserName, &e.MediaType, &e.Title,
		&e.ParentTitle, &e.GrandparentTitle, &e.Year, &e.DurationMs, &e.WatchedMs,
		&e.Player, &e.Platform, &e.IPAddress, &e.StartedAt, &e.StoppedAt, &e.CreatedAt,
		&e.SeasonNumber, &e.EpisodeNumber, &e.ThumbURL, &e.VideoResolution, &e.TranscodeDecision,
		&e.VideoCodec, &e.AudioCodec, &e.AudioChannels, &e.Bandwidth, &e.VideoDecision, &e.AudioDecision,
		&hwDecode, &hwEncode, &e.DynamicRange, &e.PausedMs, &watched)
	e.TranscodeHWDecode = hwDecode != 0
	e.TranscodeHWEncode = hwEncode != 0
	e.Watched = watched != 0
	return e, err
}

func scanHistoryEntryWithGeo(scanner interface{ Scan(...any) error }) (models.WatchHistoryEntry, error) {
	var e models.WatchHistoryEntry
	var hwDecode, hwEncode, watched int
	err := scanner.Scan(&e.ID, &e.ServerID, &e.ItemID, &e.GrandparentItemID, &e.UserName, &e.MediaType, &e.Title,
		&e.ParentTitle, &e.GrandparentTitle, &e.Year, &e.DurationMs, &e.WatchedMs,
		&e.Player, &e.Platform, &e.IPAddress, &e.StartedAt, &e.StoppedAt, &e.CreatedAt,
		&e.SeasonNumber, &e.EpisodeNumber, &e.ThumbURL, &e.VideoResolution, &e.TranscodeDecision,
		&e.VideoCodec, &e.AudioCodec, &e.AudioChannels, &e.Bandwidth, &e.VideoDecision, &e.AudioDecision,
		&hwDecode, &hwEncode, &e.DynamicRange, &e.PausedMs, &watched,
		&e.City, &e.Country, &e.ISP)
	e.TranscodeHWDecode = hwDecode != 0
	e.TranscodeHWEncode = hwEncode != 0
	e.Watched = watched != 0
	return e, err
}

func (s *Store) InsertHistory(entry *models.WatchHistoryEntry) error {
	hwDecode := boolToInt(entry.TranscodeHWDecode)
	hwEncode := boolToInt(entry.TranscodeHWEncode)
	watched := boolToInt(entry.Watched)
	result, err := s.db.Exec(historyInsertSQL,
		entry.ServerID, entry.ItemID, entry.GrandparentItemID, entry.UserName, entry.MediaType, entry.Title,
		entry.ParentTitle, entry.GrandparentTitle, entry.Year,
		entry.DurationMs, entry.WatchedMs, entry.Player, entry.Platform,
		entry.IPAddress, entry.StartedAt, entry.StoppedAt,
		entry.SeasonNumber, entry.EpisodeNumber, entry.ThumbURL,
		entry.VideoResolution, entry.TranscodeDecision,
		entry.VideoCodec, entry.AudioCodec, entry.AudioChannels, entry.Bandwidth,
		entry.VideoDecision, entry.AudioDecision, hwDecode, hwEncode, entry.DynamicRange,
		entry.PausedMs, watched, entry.TautulliReferenceID,
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

var validHistorySortColumns = map[string]bool{
	"h.started_at":  true,
	"h.stopped_at":  true,
	"h.title":       true,
	"h.user_name":   true,
	"h.duration_ms": true,
	"h.watched_ms":  true,
	"h.media_type":  true,
	"h.platform":    true,
	"h.player":      true,
	"h.created_at":  true,
	"g.city":        true,
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
	if sortColumn != "" && validHistorySortColumns[sortColumn] {
		order := "DESC"
		if sortOrder == "ASC" || sortOrder == "asc" {
			order = "ASC"
		}
		orderBy = sortColumn + " " + order
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
	return s.DailyWatchCountsForUser(start, end, "", nil)
}

func (s *Store) DailyWatchCountsForUser(start, end time.Time, userFilter string, serverIDs []int64) ([]models.DayStat, error) {
	conditions := []string{"started_at >= ?", "started_at < ?"}
	args := []any{start, end}

	if userFilter != "" {
		conditions = append(conditions, "user_name = ?")
		args = append(args, userFilter)
	}
	if len(serverIDs) > 0 {
		placeholders := strings.Repeat(",?", len(serverIDs))[1:]
		conditions = append(conditions, fmt.Sprintf("server_id IN (%s)", placeholders))
		for _, id := range serverIDs {
			args = append(args, id)
		}
	}

	query := `SELECT date(started_at) AS day, media_type, COUNT(*) AS cnt
		FROM watch_history
		WHERE ` + strings.Join(conditions, " AND ") + `
		GROUP BY day, media_type
		ORDER BY day`

	rows, err := s.db.Query(query, args...)
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
	return s.HistoryForTitleByUser(title, "", limit)
}

func (s *Store) HistoryForTitleByUser(title, userName string, limit int) ([]models.WatchHistoryEntry, error) {
	var query string
	var args []any

	if userName != "" {
		query = `SELECT ` + historyColumns + ` FROM watch_history
			WHERE (title = ? OR grandparent_title = ?) AND user_name = ?
			ORDER BY started_at DESC LIMIT ?`
		args = []any{title, title, userName, limit}
	} else {
		query = `SELECT ` + historyColumns + ` FROM watch_history
			WHERE title = ? OR grandparent_title = ?
			ORDER BY started_at DESC LIMIT ?`
		args = []any{title, title, limit}
	}

	rows, err := s.db.Query(query, args...)
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

func (s *Store) UpdateHistoryStreamDetails(id int64, entry *models.WatchHistoryEntry) error {
	hwDecode := boolToInt(entry.TranscodeHWDecode)
	hwEncode := boolToInt(entry.TranscodeHWEncode)
	_, err := s.db.Exec(`UPDATE watch_history SET
		video_resolution = ?, video_codec = ?, audio_codec = ?, audio_channels = ?,
		bandwidth = ?, transcode_decision = ?, video_decision = ?, audio_decision = ?,
		transcode_hw_decode = ?, transcode_hw_encode = ?, dynamic_range = ?
		WHERE id = ?`,
		entry.VideoResolution, entry.VideoCodec, entry.AudioCodec, entry.AudioChannels,
		entry.Bandwidth, entry.TranscodeDecision, entry.VideoDecision, entry.AudioDecision,
		hwDecode, hwEncode, entry.DynamicRange, id,
	)
	if err != nil {
		return fmt.Errorf("updating history stream details: %w", err)
	}
	return nil
}

func (s *Store) ListHistoryNeedingEnrichment(serverID int64, limit int) ([]models.WatchHistoryEntry, error) {
	rows, err := s.db.Query(`SELECT `+historyColumns+` FROM watch_history
		WHERE server_id = ? AND (video_resolution = '' OR video_resolution IS NULL)
		ORDER BY started_at DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing history for enrichment: %w", err)
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

func (s *Store) GetLastStreamBeforeTime(userName string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	since := beforeTime.Add(-time.Duration(withinHours) * time.Hour)

	query := `SELECT ` + historyColumns + `
		FROM watch_history
		WHERE user_name = ? AND started_at < ? AND started_at >= ?
		ORDER BY started_at DESC LIMIT 1`

	row := s.db.QueryRow(query, userName, beforeTime, since)
	e, err := scanHistoryEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting last stream: %w", err)
	}
	return &e, nil
}

func (s *Store) GetDeviceLastStream(userName, player, platform string, beforeTime time.Time, withinHours int) (*models.WatchHistoryEntry, error) {
	since := beforeTime.Add(-time.Duration(withinHours) * time.Hour)

	query := `SELECT ` + historyColumns + `
		FROM watch_history
		WHERE user_name = ? AND player = ? AND platform = ?
		AND started_at < ? AND started_at >= ?
		ORDER BY started_at DESC LIMIT 1`

	row := s.db.QueryRow(query, userName, player, platform, beforeTime, since)
	e, err := scanHistoryEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting device last stream: %w", err)
	}
	return &e, nil
}

func (s *Store) HasDeviceBeenUsed(userName, player, platform string, beforeTime time.Time) (bool, error) {
	var dummy int
	err := s.db.QueryRow(
		`SELECT 1 FROM watch_history
		WHERE user_name = ? AND player = ? AND platform = ? AND started_at < ? LIMIT 1`,
		userName, player, platform, beforeTime,
	).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking device usage: %w", err)
	}
	return true, nil
}

func (s *Store) GetUserDistinctIPs(userName string, beforeTime time.Time, limit int) ([]string, error) {
	query := `SELECT ip_address FROM watch_history
		WHERE user_name = ? AND started_at < ? AND ip_address != ''
		GROUP BY ip_address
		ORDER BY MAX(started_at) DESC LIMIT ?`

	rows, err := s.db.Query(query, userName, beforeTime, limit)
	if err != nil {
		return nil, fmt.Errorf("getting distinct IPs: %w", err)
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

func (s *Store) GetRecentDevices(userName string, beforeTime time.Time, withinHours int) ([]models.DeviceInfo, error) {
	since := beforeTime.Add(-time.Duration(withinHours) * time.Hour)

	query := `SELECT DISTINCT player, platform FROM watch_history
		WHERE user_name = ? AND started_at >= ? AND started_at < ?`

	rows, err := s.db.Query(query, userName, since, beforeTime)
	if err != nil {
		return nil, fmt.Errorf("getting recent devices: %w", err)
	}
	defer rows.Close()

	devices := []models.DeviceInfo{}
	for rows.Next() {
		var d models.DeviceInfo
		if err := rows.Scan(&d.Player, &d.Platform); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Store) GetRecentISPs(userName string, beforeTime time.Time, withinHours int) ([]string, error) {
	since := beforeTime.Add(-time.Duration(withinHours) * time.Hour)

	query := `SELECT DISTINCT g.isp FROM watch_history h
		JOIN ip_geo_cache g ON h.ip_address = g.ip
		WHERE h.user_name = ? AND h.started_at >= ? AND h.started_at < ?
		AND g.isp != ''`

	rows, err := s.db.Query(query, userName, since, beforeTime)
	if err != nil {
		return nil, fmt.Errorf("getting recent ISPs: %w", err)
	}
	defer rows.Close()

	isps := []string{}
	for rows.Next() {
		var isp string
		if err := rows.Scan(&isp); err != nil {
			return nil, err
		}
		isps = append(isps, isp)
	}
	return isps, rows.Err()
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

		hwDecode := boolToInt(entry.TranscodeHWDecode)
		hwEncode := boolToInt(entry.TranscodeHWEncode)
		watched := boolToInt(entry.Watched)
		_, err = insertStmt.ExecContext(ctx,
			entry.ServerID, entry.ItemID, entry.GrandparentItemID, entry.UserName, entry.MediaType, entry.Title,
			entry.ParentTitle, entry.GrandparentTitle, entry.Year,
			entry.DurationMs, entry.WatchedMs, entry.Player, entry.Platform,
			entry.IPAddress, entry.StartedAt, entry.StoppedAt,
			entry.SeasonNumber, entry.EpisodeNumber, entry.ThumbURL,
			entry.VideoResolution, entry.TranscodeDecision,
			entry.VideoCodec, entry.AudioCodec, entry.AudioChannels, entry.Bandwidth,
			entry.VideoDecision, entry.AudioDecision, hwDecode, hwEncode, entry.DynamicRange,
			entry.PausedMs, watched, entry.TautulliReferenceID,
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

// UnenrichedRef is a minimal pair of (row ID, tautulli reference_id) for enrichment.
type UnenrichedRef struct {
	ID    int64
	RefID int64
}

func (s *Store) CountUnenrichedHistory(ctx context.Context, serverID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM watch_history
		 WHERE tautulli_reference_id > 0 AND enriched = 0 AND server_id = ?`,
		serverID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting unenriched history: %w", err)
	}
	return count, nil
}

func (s *Store) ListUnenrichedHistory(ctx context.Context, serverID int64, limit int) ([]UnenrichedRef, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tautulli_reference_id FROM watch_history
		 WHERE tautulli_reference_id > 0 AND enriched = 0 AND server_id = ?
		 ORDER BY id LIMIT ?`,
		serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing unenriched history: %w", err)
	}
	defer rows.Close()

	var refs []UnenrichedRef
	for rows.Next() {
		var ref UnenrichedRef
		if err := rows.Scan(&ref.ID, &ref.RefID); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (s *Store) UpdateHistoryEnrichment(ctx context.Context, id int64, entry *models.WatchHistoryEntry) error {
	hwDecode := boolToInt(entry.TranscodeHWDecode)
	hwEncode := boolToInt(entry.TranscodeHWEncode)
	_, err := s.db.ExecContext(ctx, `UPDATE watch_history SET
		video_resolution = COALESCE(NULLIF(?, ''), video_resolution),
		video_codec = ?, audio_codec = ?, audio_channels = ?,
		bandwidth = ?,
		transcode_decision = COALESCE(NULLIF(?, ''), transcode_decision),
		video_decision = ?, audio_decision = ?,
		transcode_hw_decode = ?, transcode_hw_encode = ?, dynamic_range = ?,
		enriched = 1
		WHERE id = ?`,
		entry.VideoResolution, entry.VideoCodec, entry.AudioCodec, entry.AudioChannels,
		entry.Bandwidth, entry.TranscodeDecision, entry.VideoDecision, entry.AudioDecision,
		hwDecode, hwEncode, entry.DynamicRange, id,
	)
	if err != nil {
		return fmt.Errorf("updating history enrichment: %w", err)
	}
	return nil
}

func (s *Store) MarkHistoryEnriched(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE watch_history SET enriched = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("marking history enriched: %w", err)
	}
	return nil
}
