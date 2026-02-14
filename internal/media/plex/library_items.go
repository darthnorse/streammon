package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"streammon/internal/httputil"
	"streammon/internal/mediautil"
	"streammon/internal/models"
)

const (
	plexTypeMovie     = "1"
	plexTypeShow      = "2"
	itemBatchSize     = 100
	historyBatchSize  = 200
	historyMaxEntries = 100000
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
	GrandparentRatingKey string `xml:"grandparentRatingKey,attr"`
	ViewedAt             string `xml:"viewedAt,attr"`
}

func (s *Server) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	var allItems []models.LibraryItemCache

	movies, err := s.fetchLibraryItemsPage(ctx, libraryID, plexTypeMovie)
	if err != nil {
		return nil, fmt.Errorf("fetch movies: %w", err)
	}
	allItems = append(allItems, movies...)

	shows, err := s.fetchLibraryItemsPage(ctx, libraryID, plexTypeShow)
	if err != nil {
		return nil, fmt.Errorf("fetch shows: %w", err)
	}
	for i := range shows {
		if shows[i].FileSize == 0 {
			size, err := s.getShowEpisodeSize(ctx, shows[i].ItemID)
			if err != nil {
				slog.Warn("plex: failed to get episode sizes", "title", shows[i].Title, "error", err)
				continue
			}
			shows[i].FileSize = size
		}
	}
	if len(shows) > 0 {
		historyMap, err := s.fetchShowWatchHistory(ctx)
		if err != nil {
			slog.Warn("plex: failed to fetch show watch history, using show-level data", "error", err)
		} else {
			mediautil.EnrichLastWatched(shows, historyMap)
		}
	}
	allItems = append(allItems, shows...)

	if allItems == nil {
		return []models.LibraryItemCache{}, nil
	}

	var totalSize int64
	var zeroSize int
	for _, item := range allItems {
		totalSize += item.FileSize
		if item.FileSize == 0 {
			zeroSize++
		}
	}
	slog.Info("plex: library sync complete",
		"library", libraryID, "movies", len(movies), "series", len(shows),
		"total", len(allItems), "zero_size", zeroSize, "total_bytes", totalSize)

	return allItems, nil
}

func (s *Server) fetchLibraryItemsPage(ctx context.Context, libraryID, typeFilter string) ([]models.LibraryItemCache, error) {
	var allItems []models.LibraryItemCache
	offset := 0

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		items, totalCount, err := s.fetchLibraryBatch(ctx, libraryID, typeFilter, offset, itemBatchSize)
		if err != nil {
			return nil, err
		}

		if len(items) == 0 {
			break
		}

		allItems = append(allItems, items...)
		offset += len(items)

		if offset >= totalCount {
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
		})
	}

	return items, container.TotalSize, nil
}

func (s *Server) getShowEpisodeSize(ctx context.Context, showRatingKey string) (int64, error) {
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

func (s *Server) fetchShowWatchHistory(ctx context.Context) (map[string]time.Time, error) {
	result := make(map[string]time.Time)
	offset := 0

	for offset < historyMaxEntries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		container, err := s.fetchHistoryBatch(ctx, offset)
		if err != nil {
			return nil, err
		}

		if len(container.Videos) == 0 {
			break
		}

		for _, v := range container.Videos {
			if v.GrandparentRatingKey == "" {
				continue
			}
			if _, exists := result[v.GrandparentRatingKey]; !exists {
				if ts := atoi64(v.ViewedAt); ts > 0 {
					result[v.GrandparentRatingKey] = time.Unix(ts, 0).UTC()
				}
			}
		}

		offset += len(container.Videos)
		if offset >= container.Size {
			break
		}
	}

	return result, nil
}

func (s *Server) fetchHistoryBatch(ctx context.Context, offset int) (*historyContainer, error) {
	u, err := url.Parse(fmt.Sprintf("%s/status/sessions/history/all", s.url))
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("sort", "viewedAt:desc")
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
