package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"streammon/internal/models"
)

func calcPercentage(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total) * 100
}

var allowedDistributionColumns = map[string]bool{
	"platform":         true,
	"player":           true,
	"video_resolution": true,
}

func formatLastSeen(s sql.NullString) string {
	if s.Valid {
		if t, _ := parseSQLiteTime(s.String); !t.IsZero() {
			return t.Format(time.RFC3339)
		}
	}
	return ""
}

func cutoffTime(days int) time.Time {
	if days <= 0 {
		return time.Time{}
	}
	return time.Now().UTC().AddDate(0, 0, -days)
}

type StatsFilter struct {
	Days      int
	StartDate time.Time
	EndDate   time.Time
	ServerIDs []int64
}

func (f StatsFilter) timeConditionWith(alias string) (string, []any) {
	col := "started_at"
	if alias != "" {
		col = alias + ".started_at"
	}
	if !f.StartDate.IsZero() && !f.EndDate.IsZero() {
		return col + " >= ? AND " + col + " < ?", []any{f.StartDate, f.EndDate}
	}
	cutoff := cutoffTime(f.Days)
	if !cutoff.IsZero() {
		return col + " >= ?", []any{cutoff}
	}
	return "", nil
}

func (f StatsFilter) serverConditionWith(alias string) (string, []any) {
	if len(f.ServerIDs) == 0 {
		return "", nil
	}
	col := "server_id"
	if alias != "" {
		col = alias + ".server_id"
	}
	placeholders := strings.Repeat(",?", len(f.ServerIDs))[1:]
	args := make([]any, len(f.ServerIDs))
	for i, id := range f.ServerIDs {
		args[i] = id
	}
	return fmt.Sprintf("%s IN (%s)", col, placeholders), args
}

func buildConditions(alias string, f StatsFilter) (conds []string, args []any) {
	if tc, ta := f.timeConditionWith(alias); tc != "" {
		conds = append(conds, tc)
		args = append(args, ta...)
	}
	if sc, sa := f.serverConditionWith(alias); sc != "" {
		conds = append(conds, sc)
		args = append(args, sa...)
	}
	return
}

func (f StatsFilter) conditionsWithPrefix(prefix, alias string) (string, []any) {
	conds, args := buildConditions(alias, f)
	if len(conds) == 0 {
		return "", nil
	}
	return prefix + strings.Join(conds, " AND "), args
}

func (f StatsFilter) conditions() (string, []any)                  { return f.conditionsWithPrefix(" WHERE ", "") }
func (f StatsFilter) andConditions() (string, []any)               { return f.conditionsWithPrefix(" AND ", "") }
func (f StatsFilter) andConditionsWith(alias string) (string, []any) { return f.conditionsWithPrefix(" AND ", alias) }

type topMediaConfig struct {
	selectCol  string
	yearExpr   string
	extraWhere string
	groupBy    string
	mediaType  models.MediaType
	errMsg     string
	itemIDCol  string
	metaWhere  string
	metaArgs   func(stat models.MediaStat) []any
}

func (s *Store) topMedia(ctx context.Context, limit int, filter StatsFilter, cfg topMediaConfig) ([]models.MediaStat, error) {
	filterClause, filterArgs := filter.andConditions()

	itemIDCol := cfg.itemIDCol
	if itemIDCol == "" {
		itemIDCol = "item_id"
	}

	query := fmt.Sprintf(`SELECT %s, %s, COUNT(*) as play_count,
		SUM(watched_ms) / 3600000.0 as total_hours
	FROM watch_history
	WHERE media_type = ?%s%s
	GROUP BY %s
	ORDER BY play_count DESC
	LIMIT ?`,
		cfg.selectCol, cfg.yearExpr,
		cfg.extraWhere, filterClause,
		cfg.groupBy)

	var args []any
	args = append(args, cfg.mediaType)
	args = append(args, filterArgs...)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cfg.errMsg, err)
	}
	defer rows.Close()

	stats := []models.MediaStat{}
	for rows.Next() {
		var stat models.MediaStat
		var totalHours sql.NullFloat64
		if err := rows.Scan(&stat.Title, &stat.Year, &stat.PlayCount, &totalHours); err != nil {
			return nil, fmt.Errorf("scanning %s: %w", cfg.errMsg, err)
		}
		if totalHours.Valid {
			stat.TotalHours = totalHours.Float64
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating %s: %w", cfg.errMsg, err)
	}

	if len(stats) == 0 {
		return stats, nil
	}

	metaQuery := fmt.Sprintf(`SELECT thumb_url, server_id, %s
		FROM watch_history
		WHERE media_type = ? AND %s AND thumb_url != ''%s
		ORDER BY started_at DESC LIMIT 1`,
		itemIDCol, cfg.metaWhere, filterClause)

	for i := range stats {
		var thumbURL sql.NullString
		var serverID sql.NullInt64
		var itemID sql.NullString
		metaArgs := append([]any{cfg.mediaType}, cfg.metaArgs(stats[i])...)
		metaArgs = append(metaArgs, filterArgs...)
		err := s.db.QueryRowContext(ctx, metaQuery, metaArgs...).Scan(&thumbURL, &serverID, &itemID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("%s metadata: %w", cfg.errMsg, err)
		}
		if thumbURL.Valid {
			stats[i].ThumbURL = thumbURL.String
		}
		if serverID.Valid {
			stats[i].ServerID = serverID.Int64
		}
		if itemID.Valid {
			stats[i].ItemID = itemID.String
		}
	}

	return stats, nil
}

func (s *Store) TopMovies(ctx context.Context, limit int, filter StatsFilter) ([]models.MediaStat, error) {
	return s.topMedia(ctx, limit, filter, topMediaConfig{
		selectCol:  "title",
		yearExpr:   "year",
		extraWhere: "",
		groupBy:    "title, year",
		mediaType:  models.MediaTypeMovie,
		errMsg:     "top movies",
		metaWhere:  "title = ? AND year = ?",
		metaArgs:   func(s models.MediaStat) []any { return []any{s.Title, s.Year} },
	})
}

func (s *Store) TopTVShows(ctx context.Context, limit int, filter StatsFilter) ([]models.MediaStat, error) {
	return s.topMedia(ctx, limit, filter, topMediaConfig{
		selectCol:  "grandparent_title",
		yearExpr:   "0 as year",
		extraWhere: " AND grandparent_title != ''",
		groupBy:    "grandparent_title",
		mediaType:  models.MediaTypeTV,
		errMsg:     "top tv shows",
		itemIDCol:  "grandparent_item_id",
		metaWhere:  "grandparent_title = ?",
		metaArgs:   func(s models.MediaStat) []any { return []any{s.Title} },
	})
}

func (s *Store) TopUsers(ctx context.Context, limit int, filter StatsFilter) ([]models.UserStat, error) {
	whereClause, filterArgs := filter.conditions()

	query := `SELECT user_name, COUNT(*) as play_count,
		SUM(watched_ms) / 3600000.0 as total_hours
	FROM watch_history` + whereClause + ` GROUP BY user_name ORDER BY total_hours DESC LIMIT ?`

	args := append(filterArgs, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("top users: %w", err)
	}
	defer rows.Close()

	stats := []models.UserStat{}
	for rows.Next() {
		var stat models.UserStat
		var totalHours sql.NullFloat64
		if err := rows.Scan(&stat.UserName, &stat.PlayCount, &totalHours); err != nil {
			return nil, fmt.Errorf("scanning user stats: %w", err)
		}
		if totalHours.Valid {
			stat.TotalHours = totalHours.Float64
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user stats: %w", err)
	}
	return stats, nil
}

func (s *Store) LibraryStats(ctx context.Context, filter StatsFilter) (*models.LibraryStat, error) {
	var stats models.LibraryStat
	var totalHours sql.NullFloat64

	whereClause, filterArgs := filter.conditions()

	query := `SELECT COUNT(*) as total_plays,
		SUM(watched_ms) / 3600000.0 as total_hours,
		COUNT(DISTINCT user_name) as unique_users,
		COUNT(DISTINCT CASE WHEN media_type = ? THEN title || '|' || COALESCE(year, 0) END) as unique_movies,
		COUNT(DISTINCT CASE WHEN media_type = ? AND grandparent_title != '' THEN grandparent_title END) as unique_tv_shows
	FROM watch_history` + whereClause

	args := []any{models.MediaTypeMovie, models.MediaTypeTV}
	args = append(args, filterArgs...)
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalPlays, &totalHours, &stats.UniqueUsers,
		&stats.UniqueMovies, &stats.UniqueTVShows,
	); err != nil {
		return nil, fmt.Errorf("library stats: %w", err)
	}
	if totalHours.Valid {
		stats.TotalHours = totalHours.Float64
	}

	return &stats, nil
}

type concurrentEvent struct {
	t        time.Time
	delta    int
	decision string
}

// loadConcurrentEvents returns start/stop events sorted by time, stops before starts (half-open intervals).
func (s *Store) loadConcurrentEvents(ctx context.Context, filter StatsFilter) ([]concurrentEvent, error) {
	whereClause, filterArgs := filter.conditions()
	query := `SELECT started_at, stopped_at, transcode_decision FROM watch_history` + whereClause
	rows, err := s.db.QueryContext(ctx, query, filterArgs...)
	if err != nil {
		return nil, fmt.Errorf("loading concurrent events: %w", err)
	}
	defer rows.Close()

	var events []concurrentEvent
	for rows.Next() {
		var start, stop time.Time
		var decision string
		if err := rows.Scan(&start, &stop, &decision); err != nil {
			return nil, fmt.Errorf("scanning concurrent event: %w", err)
		}
		if stop.IsZero() || stop.Before(start) {
			continue
		}
		events = append(events, concurrentEvent{t: start, delta: 1, decision: decision})
		events = append(events, concurrentEvent{t: stop, delta: -1, decision: decision})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating concurrent events: %w", err)
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].t.Equal(events[j].t) {
			return events[i].delta < events[j].delta
		}
		return events[i].t.Before(events[j].t)
	})

	return events, nil
}


func (s *Store) ConcurrentStats(ctx context.Context, filter StatsFilter) ([]models.ConcurrentTimePoint, models.ConcurrentPeaks, error) {
	events, err := s.loadConcurrentEvents(ctx, filter)
	if err != nil {
		return nil, models.ConcurrentPeaks{}, err
	}
	if len(events) == 0 {
		return []models.ConcurrentTimePoint{}, models.ConcurrentPeaks{}, nil
	}

	var peaks models.ConcurrentPeaks
	var peakTime time.Time
	hourlyMax := make(map[time.Time]models.ConcurrentTimePoint)
	var directPlay, directStream, transcode int

	for _, ev := range events {
		switch models.TranscodeDecision(ev.decision) {
		case models.TranscodeDecisionCopy:
			directStream += ev.delta
		case models.TranscodeDecisionTranscode:
			transcode += ev.delta
		default:
			directPlay += ev.delta
		}
		total := directPlay + directStream + transcode

		if total > peaks.Total {
			peaks.Total = total
			peakTime = ev.t
		}
		if directPlay > peaks.DirectPlay {
			peaks.DirectPlay = directPlay
		}
		if directStream > peaks.DirectStream {
			peaks.DirectStream = directStream
		}
		if transcode > peaks.Transcode {
			peaks.Transcode = transcode
		}

		hourBucket := ev.t.Truncate(time.Hour)
		if existing, ok := hourlyMax[hourBucket]; !ok || total > existing.Total {
			hourlyMax[hourBucket] = models.ConcurrentTimePoint{
				Time:         hourBucket,
				DirectPlay:   directPlay,
				DirectStream: directStream,
				Transcode:    transcode,
				Total:        total,
			}
		}
	}

	if !peakTime.IsZero() {
		peaks.PeakAt = peakTime.Format(time.RFC3339)
	}

	points := make([]models.ConcurrentTimePoint, 0, len(hourlyMax))
	for _, p := range hourlyMax {
		points = append(points, p)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Time.Before(points[j].Time)
	})

	return points, peaks, nil
}

func (s *Store) AllWatchLocations(ctx context.Context, filter StatsFilter) ([]models.GeoResult, error) {
	filterClause, filterArgs := filter.andConditionsWith("h")

	query := `SELECT g.lat, g.lng, g.city, g.country, COALESCE(MAX(g.isp), '') as isp,
		COALESCE(GROUP_CONCAT(DISTINCT h.user_name), '') as users
	FROM watch_history h
	JOIN ip_geo_cache g ON h.ip_address = g.ip
	WHERE h.ip_address != ''` + filterClause + ` GROUP BY g.lat, g.lng, g.city, g.country
	ORDER BY g.country, g.city`
	rows, err := s.db.QueryContext(ctx, query, filterArgs...)
	if err != nil {
		return nil, fmt.Errorf("watch locations: %w", err)
	}
	defer rows.Close()

	results := []models.GeoResult{}
	for rows.Next() {
		var geo models.GeoResult
		var usersStr string
		if err := rows.Scan(&geo.Lat, &geo.Lng, &geo.City, &geo.Country, &geo.ISP, &usersStr); err != nil {
			return nil, fmt.Errorf("scanning watch location: %w", err)
		}
		if usersStr != "" {
			geo.Users = strings.Split(usersStr, ",")
		}
		results = append(results, geo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating watch locations: %w", err)
	}
	return results, nil
}

func (s *Store) UserDetailStats(ctx context.Context, userName string) (*models.UserDetailStats, error) {
	stats := &models.UserDetailStats{
		Locations: []models.LocationStat{},
		Devices:   []models.DeviceStat{},
		ISPs:      []models.ISPStat{},
	}

	var totalHours sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) as session_count,
			SUM(watched_ms) / 3600000.0 as total_hours
		FROM watch_history
		WHERE user_name = ?`,
		userName,
	).Scan(&stats.SessionCount, &totalHours)
	if err != nil {
		return nil, fmt.Errorf("user stats totals: %w", err)
	}
	if totalHours.Valid {
		stats.TotalHours = totalHours.Float64
	}

	locRows, err := s.db.QueryContext(ctx,
		`SELECT g.city, g.country, COUNT(*) as session_count,
			MAX(COALESCE(h.stopped_at, h.started_at)) as last_seen
		FROM watch_history h
		JOIN ip_geo_cache g ON h.ip_address = g.ip
		WHERE h.user_name = ?
		GROUP BY g.city, g.country
		ORDER BY session_count DESC
		LIMIT 10`,
		userName,
	)
	if err != nil {
		return nil, fmt.Errorf("user location stats: %w", err)
	}
	defer locRows.Close()

	var totalLocSessions int
	for locRows.Next() {
		var loc models.LocationStat
		var lastSeenStr sql.NullString
		if err := locRows.Scan(&loc.City, &loc.Country, &loc.SessionCount, &lastSeenStr); err != nil {
			return nil, fmt.Errorf("scanning location stat: %w", err)
		}
		loc.LastSeen = formatLastSeen(lastSeenStr)
		totalLocSessions += loc.SessionCount
		stats.Locations = append(stats.Locations, loc)
	}
	if err := locRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating location stats: %w", err)
	}
	for i := range stats.Locations {
		stats.Locations[i].Percentage = calcPercentage(stats.Locations[i].SessionCount, totalLocSessions)
	}

	devRows, err := s.db.QueryContext(ctx,
		`SELECT player, platform, COUNT(*) as session_count,
			MAX(COALESCE(stopped_at, started_at)) as last_seen
		FROM watch_history
		WHERE user_name = ?
		GROUP BY player, platform
		ORDER BY session_count DESC
		LIMIT 10`,
		userName,
	)
	if err != nil {
		return nil, fmt.Errorf("user device stats: %w", err)
	}
	defer devRows.Close()

	var totalDevSessions int
	for devRows.Next() {
		var dev models.DeviceStat
		var lastSeenStr sql.NullString
		if err := devRows.Scan(&dev.Player, &dev.Platform, &dev.SessionCount, &lastSeenStr); err != nil {
			return nil, fmt.Errorf("scanning device stat: %w", err)
		}
		dev.LastSeen = formatLastSeen(lastSeenStr)
		totalDevSessions += dev.SessionCount
		stats.Devices = append(stats.Devices, dev)
	}
	if err := devRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating device stats: %w", err)
	}
	for i := range stats.Devices {
		stats.Devices[i].Percentage = calcPercentage(stats.Devices[i].SessionCount, totalDevSessions)
	}

	ispRows, err := s.db.QueryContext(ctx,
		`SELECT g.isp, COUNT(*) as session_count,
			MAX(COALESCE(h.stopped_at, h.started_at)) as last_seen
		FROM watch_history h
		JOIN ip_geo_cache g ON h.ip_address = g.ip
		WHERE h.user_name = ? AND g.isp != ''
		GROUP BY g.isp
		ORDER BY session_count DESC
		LIMIT 10`,
		userName,
	)
	if err != nil {
		return nil, fmt.Errorf("user isp stats: %w", err)
	}
	defer ispRows.Close()

	var totalISPSessions int
	for ispRows.Next() {
		var isp models.ISPStat
		var lastSeenStr sql.NullString
		if err := ispRows.Scan(&isp.ISP, &isp.SessionCount, &lastSeenStr); err != nil {
			return nil, fmt.Errorf("scanning isp stat: %w", err)
		}
		isp.LastSeen = formatLastSeen(lastSeenStr)
		totalISPSessions += isp.SessionCount
		stats.ISPs = append(stats.ISPs, isp)
	}
	if err := ispRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating isp stats: %w", err)
	}
	for i := range stats.ISPs {
		stats.ISPs[i].Percentage = calcPercentage(stats.ISPs[i].SessionCount, totalISPSessions)
	}

	return stats, nil
}

var dayNames = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

var allowedStrftimeFormats = map[string]bool{
	"%w": true, // day of week (0-6)
	"%H": true, // hour (00-23)
}

func (s *Store) activityCounts(ctx context.Context, filter StatsFilter, strftimeFmt, errContext string) (map[int]int, error) {
	if !allowedStrftimeFormats[strftimeFmt] {
		return nil, fmt.Errorf("%s: invalid strftime format %q", errContext, strftimeFmt)
	}

	whereClause, filterArgs := filter.conditions()

	query := fmt.Sprintf(`SELECT CAST(strftime('%s', started_at) AS INTEGER) as bucket, COUNT(*) as play_count
		FROM watch_history`, strftimeFmt)
	query += whereClause
	query += ` GROUP BY bucket ORDER BY bucket`

	rows, err := s.db.QueryContext(ctx, query, filterArgs...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errContext, err)
	}
	defer rows.Close()

	counts := make(map[int]int)
	for rows.Next() {
		var bucket, cnt int
		if err := rows.Scan(&bucket, &cnt); err != nil {
			return nil, fmt.Errorf("scanning %s: %w", errContext, err)
		}
		counts[bucket] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating %s: %w", errContext, err)
	}

	return counts, nil
}

// Day/hour calculations are based on UTC timestamps, not user local time.
func (s *Store) ActivityByDayOfWeek(ctx context.Context, filter StatsFilter) ([]models.DayOfWeekStat, error) {
	counts, err := s.activityCounts(ctx, filter, "%w", "activity by day of week")
	if err != nil {
		return nil, err
	}

	stats := make([]models.DayOfWeekStat, 7)
	for i := 0; i < 7; i++ {
		stats[i] = models.DayOfWeekStat{
			DayOfWeek: i,
			DayName:   dayNames[i],
			PlayCount: counts[i],
		}
	}
	return stats, nil
}

func (s *Store) ActivityByHour(ctx context.Context, filter StatsFilter) ([]models.HourStat, error) {
	counts, err := s.activityCounts(ctx, filter, "%H", "activity by hour")
	if err != nil {
		return nil, err
	}

	stats := make([]models.HourStat, 24)
	for i := 0; i < 24; i++ {
		stats[i] = models.HourStat{
			Hour:      i,
			PlayCount: counts[i],
		}
	}
	return stats, nil
}

func (s *Store) PlatformDistribution(ctx context.Context, filter StatsFilter) ([]models.DistributionStat, error) {
	return s.distribution(ctx, filter, "platform", "platform distribution")
}

func (s *Store) PlayerDistribution(ctx context.Context, filter StatsFilter) ([]models.DistributionStat, error) {
	return s.distribution(ctx, filter, "player", "player distribution")
}

func (s *Store) QualityDistribution(ctx context.Context, filter StatsFilter) ([]models.DistributionStat, error) {
	return s.distribution(ctx, filter, "video_resolution", "quality distribution")
}

func (s *Store) distribution(ctx context.Context, filter StatsFilter, column, errMsg string) ([]models.DistributionStat, error) {
	if !allowedDistributionColumns[column] {
		return nil, fmt.Errorf("%s: invalid column %q", errMsg, column)
	}

	whereClause, filterArgs := filter.conditions()

	query := fmt.Sprintf(`SELECT COALESCE(NULLIF(%s, ''), 'Unknown') as name, COUNT(*) as cnt
		FROM watch_history`, column)
	query += whereClause
	query += ` GROUP BY name ORDER BY cnt DESC`

	rows, err := s.db.QueryContext(ctx, query, filterArgs...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errMsg, err)
	}
	defer rows.Close()

	stats := []models.DistributionStat{}
	var total int
	for rows.Next() {
		var stat models.DistributionStat
		if err := rows.Scan(&stat.Name, &stat.Count); err != nil {
			return nil, fmt.Errorf("scanning %s: %w", errMsg, err)
		}
		total += stat.Count
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating %s: %w", errMsg, err)
	}

	for i := range stats {
		stats[i].Percentage = calcPercentage(stats[i].Count, total)
	}

	return stats, nil
}


