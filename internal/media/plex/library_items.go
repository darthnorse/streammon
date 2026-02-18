package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"streammon/internal/httputil"
	"streammon/internal/mediautil"
	"streammon/internal/models"
)

const (
	plexTypeMovie     = "1"
	plexTypeShow      = "2"
	itemBatchSize     = 100
	historyBatchSize  = 2000
	historyMaxEntries = 500000
	maxResponseBody   = 50 << 20 // 50 MB
)

type libraryItemsContainer struct {
	XMLName     xml.Name         `xml:"MediaContainer"`
	TotalSize   int              `xml:"totalSize,attr"`
	Videos      []libraryItemXML `xml:"Video"`
	Directories []libraryItemXML `xml:"Directory"`
}

type libraryItemXML struct {
	RatingKey    string         `xml:"ratingKey,attr"`
	Type         string         `xml:"type,attr"`
	Title        string         `xml:"title,attr"`
	Year         string         `xml:"year,attr"`
	AddedAt      string         `xml:"addedAt,attr"`
	LastViewedAt string         `xml:"lastViewedAt,attr"`
	LeafCount    string         `xml:"leafCount,attr"`
	Guids        []plexGuid     `xml:"Guid"`
	Media        []mediaInfoXML `xml:"Media"`
}

type mediaInfoXML struct {
	VideoResolution string        `xml:"videoResolution,attr"`
	Height          string        `xml:"height,attr"`
	Parts           []partInfoXML `xml:"Part"`
}

type partInfoXML struct {
	Size int64 `xml:"size,attr"`
}

type historyContainer struct {
	XMLName xml.Name         `xml:"MediaContainer"`
	Size    int              `xml:"totalSize,attr"`
	Videos  []historyItemXML `xml:"Video"`
}

type historyItemXML struct {
	RatingKey string `xml:"ratingKey,attr"`
	// GrandparentKey is a full metadata path (e.g. "/library/metadata/151929").
	// This differs from the sessions endpoint which returns grandparentRatingKey
	// as a plain numeric ID. Use ratingKeyFromPath() to extract the ID.
	GrandparentKey string `xml:"grandparentKey,attr"`
	ViewedAt       string `xml:"viewedAt,attr"`
	AccountID      string `xml:"accountID,attr"`
}

func (s *Server) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	movies, err := s.fetchLibraryItemsPage(ctx, libraryID, plexTypeMovie)
	if err != nil {
		return nil, fmt.Errorf("fetch movies: %w", err)
	}

	series, err := s.fetchLibraryItemsPage(ctx, libraryID, plexTypeShow)
	if err != nil {
		return nil, fmt.Errorf("fetch series: %w", err)
	}
	mediautil.EnrichSeriesData(ctx, series, libraryID, "plex", s.getSeriesEpisodeSize)

	movieHistory, seriesHistory, err := s.fetchAllWatchHistory(ctx, libraryID, len(movies)+len(series))
	if err != nil {
		slog.Warn("failed to fetch watch history, using per-item data only",
			"server_type", "plex", "error", err)
	} else {
		mediautil.EnrichLastWatched(movies, movieHistory)
		mediautil.EnrichLastWatched(series, seriesHistory)
	}

	result := slices.Concat(movies, series)
	if result == nil {
		return []models.LibraryItemCache{}, nil
	}

	mediautil.LogSyncSummary("plex", libraryID, len(movies), len(series), result)
	return result, nil
}

func (s *Server) fetchLibraryItemsPage(ctx context.Context, libraryID, typeFilter string) ([]models.LibraryItemCache, error) {
	var allItems []models.LibraryItemCache
	var total int
	offset := 0

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		items, totalCount, err := s.fetchLibraryBatch(ctx, libraryID, typeFilter, offset, itemBatchSize)
		if err != nil {
			return nil, err
		}
		if total == 0 {
			total = totalCount
		}

		if len(items) == 0 {
			break
		}

		allItems = append(allItems, items...)
		offset += len(items)
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseItems,
			Current: offset,
			Total:   total,
			Library: libraryID,
		})

		if len(items) < itemBatchSize {
			break
		}
	}

	return allItems, nil
}

func (s *Server) fetchLibraryBatch(ctx context.Context, libraryID, typeFilter string, offset, batchSize int) ([]models.LibraryItemCache, int, error) {
	u, err := url.Parse(fmt.Sprintf("%s/library/sections/%s/all", s.url, url.PathEscape(libraryID)))
	if err != nil {
		return nil, 0, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("type", typeFilter)
	q.Set("includeGuids", "1")
	q.Set("X-Plex-Container-Start", strconv.Itoa(offset))
	q.Set("X-Plex-Container-Size", strconv.Itoa(batchSize))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("plex library items: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("plex library items: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, 0, err
	}

	var container libraryItemsContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, 0, fmt.Errorf("parse library items: %w", err)
	}

	xmlItems := append(container.Videos, container.Directories...)
	if len(xmlItems) == 0 {
		return nil, container.TotalSize, nil
	}

	var items []models.LibraryItemCache
	for _, item := range xmlItems {
		var resolution string
		var fileSize int64
		for _, media := range item.Media {
			if resolution == "" {
				resolution = normalizeResolution(media.VideoResolution)
				if resolution == "" && media.Height != "" {
					resolution = heightToResolution(media.Height)
				}
			}
			for _, part := range media.Parts {
				fileSize += part.Size
			}
		}

		mediaType := plexMediaType(item.Type)
		episodeCount := atoi(item.LeafCount)

		var lastWatchedAt *time.Time
		if ts := atoi64(item.LastViewedAt); ts > 0 {
			t := time.Unix(ts, 0).UTC()
			lastWatchedAt = &t
		}

		externalIDs := parsePlexGuids(item.Guids)
		items = append(items, models.LibraryItemCache{
			ServerID:        s.serverID,
			LibraryID:       libraryID,
			ItemID:          item.RatingKey,
			MediaType:       mediaType,
			Title:           item.Title,
			Year:            atoi(item.Year),
			AddedAt:         time.Unix(atoi64(item.AddedAt), 0).UTC(),
			LastWatchedAt:   lastWatchedAt,
			VideoResolution: resolution,
			FileSize:        fileSize,
			EpisodeCount:    episodeCount,
			ThumbURL:        item.RatingKey,
			TMDBID:          externalIDs.TMDB,
			TVDBID:          externalIDs.TVDB,
			IMDBID:          externalIDs.IMDB,
		})
	}

	return items, container.TotalSize, nil
}

func (s *Server) getSeriesEpisodeSize(ctx context.Context, showRatingKey string) (int64, error) {
	var totalSize int64
	offset := 0

	for {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}

		size, n, err := s.fetchEpisodeSizeBatch(ctx, showRatingKey, offset, itemBatchSize)
		if err != nil {
			return 0, err
		}

		totalSize += size

		if n == 0 || n < itemBatchSize {
			break
		}
		offset += n
	}

	return totalSize, nil
}

func (s *Server) fetchEpisodeSizeBatch(ctx context.Context, showRatingKey string, offset, batchSize int) (int64, int, error) {
	u, err := url.Parse(fmt.Sprintf("%s/library/metadata/%s/allLeaves", s.url, url.PathEscape(showRatingKey)))
	if err != nil {
		return 0, 0, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("X-Plex-Container-Start", strconv.Itoa(offset))
	q.Set("X-Plex-Container-Size", strconv.Itoa(batchSize))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, 0, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("plex episodes: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("plex episodes: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return 0, 0, err
	}

	var container libraryItemsContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return 0, 0, fmt.Errorf("parse episodes: %w", err)
	}

	var totalSize int64
	for _, ep := range container.Videos {
		for _, media := range ep.Media {
			for _, part := range media.Parts {
				totalSize += part.Size
			}
		}
	}

	return totalSize, len(container.Videos), nil
}

// fetchAllWatchHistory fetches the complete watch history for a library
// from /status/sessions/history/all, which includes ALL users (admin,
// managed, and shared). Returns separate maps for movies and TV series.
// totalItems is used for early termination: once every library item has
// been seen in history, further pages are skipped.
func (s *Server) fetchAllWatchHistory(ctx context.Context, libraryID string, totalItems int) (movieHistory, seriesHistory map[string]time.Time, err error) {
	movieHistory = make(map[string]time.Time)
	seriesHistory = make(map[string]time.Time)
	offset := 0
	accountIDs := make(map[string]bool)

	for offset < historyMaxEntries {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		container, err := s.fetchHistoryBatch(ctx, libraryID, offset)
		if err != nil {
			return nil, nil, err
		}

		if len(container.Videos) == 0 {
			break
		}

		for _, v := range container.Videos {
			ts := atoi64(v.ViewedAt)
			if ts <= 0 {
				continue
			}
			t := time.Unix(ts, 0).UTC()
			if v.AccountID != "" {
				accountIDs[v.AccountID] = true
			}

			gpRatingKey := ratingKeyFromPath(v.GrandparentKey)
			if gpRatingKey != "" {
				if existing, exists := seriesHistory[gpRatingKey]; !exists || t.After(existing) {
					seriesHistory[gpRatingKey] = t
				}
			} else if v.RatingKey != "" {
				if existing, exists := movieHistory[v.RatingKey]; !exists || t.After(existing) {
					movieHistory[v.RatingKey] = t
				}
			}
		}

		offset += len(container.Videos)
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseHistory,
			Current: offset,
			Total:   container.Size,
			Library: libraryID,
		})

		// Early exit: every library item has at least one watch entry,
		// so older pages can't contribute new data (sorted viewedAt:desc).
		matched := len(movieHistory) + len(seriesHistory)
		if totalItems > 0 && matched >= totalItems {
			break
		}

		if offset >= container.Size {
			break
		}
	}

	slog.Info("plex watch history fetched",
		"library", libraryID,
		"accounts", len(accountIDs),
		"movies", len(movieHistory),
		"series", len(seriesHistory),
		"total_entries", offset)
	return movieHistory, seriesHistory, nil
}

// ratingKeyFromPath extracts the numeric ID from a Plex metadata path.
// e.g. "/library/metadata/151929" → "151929"
// Returns "" if the trailing segment is not a valid numeric ID.
func ratingKeyFromPath(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 || i+1 >= len(path) {
		return ""
	}
	seg := path[i+1:]
	if _, err := strconv.Atoi(seg); err != nil {
		return ""
	}
	return seg
}

func (s *Server) fetchHistoryBatch(ctx context.Context, libraryID string, offset int) (*historyContainer, error) {
	u, err := url.Parse(fmt.Sprintf("%s/status/sessions/history/all", s.url))
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("sort", "viewedAt:desc")
	q.Set("librarySectionID", libraryID)
	q.Set("X-Plex-Container-Start", strconv.Itoa(offset))
	q.Set("X-Plex-Container-Size", strconv.Itoa(historyBatchSize))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plex history: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex history: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, err
	}

	var container historyContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("parse history: %w", err)
	}

	return &container, nil
}
