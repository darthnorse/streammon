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
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

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
	Width           string        `xml:"width,attr"`
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
	GrandparentKey string `xml:"grandparentKey,attr"`
	// GrandparentRatingKey is a plain numeric ID (e.g. "151929").
	// Some Plex versions return one or the other; we check both.
	GrandparentRatingKey string `xml:"grandparentRatingKey,attr"`
	ViewedAt             string `xml:"viewedAt,attr"`
	AccountID            string `xml:"accountID,attr"`
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
	s.applyCachedSeriesSizes(series)
	mediautil.EnrichSeriesData(ctx, series, libraryID, "plex", s.getSeriesEpisodeSize)

	movieHistory, seriesHistory, err := s.fetchAllWatchHistory(ctx, libraryID)
	if err != nil {
		slog.Warn("failed to fetch watch history, using per-item data only",
			"server_type", "plex", "error", err)
	} else {
		mediautil.EnrichLastWatched(movies, movieHistory)
		mediautil.EnrichLastWatched(series, seriesHistory)
	}

	// Per-item fallback: only when the bulk history mapped NOTHING of that kind.
	// That signals Plex returned history we couldn't parse (version XML
	// differences) — the case this fallback exists for. When the bulk fetch
	// mapped any items, parsing works and the unmatched items are genuinely
	// unwatched, so probing them per-item is pure waste (it recovered 0 across
	// every real library measured). Uses metadataItemID like Maintainerr.
	if len(movieHistory) == 0 {
		s.enrichMissedWatchHistory(ctx, movies, libraryID)
	}
	if len(seriesHistory) == 0 {
		s.enrichMissedWatchHistory(ctx, series, libraryID)
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
		var videoWidth, videoHeight int
		var fileSize int64
		for _, media := range item.Media {
			if resolution == "" {
				resolution = normalizeResolution(media.VideoResolution)
				if resolution == "" && media.Height != "" {
					resolution = heightToResolution(media.Height)
				}
			}
			if videoHeight == 0 && media.Height != "" {
				videoHeight = atoi(media.Height)
			}
			if videoWidth == 0 && media.Width != "" {
				videoWidth = atoi(media.Width)
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
			VideoWidth:      videoWidth,
			VideoHeight:     videoHeight,
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

// fetchAllWatchHistory includes ALL users (admin, managed, and shared).
// totalItems is used for early termination: once every library item has
// been seen in history, further pages are skipped.
// historyConcurrency bounds parallel history-page fetches. A library with a
// large watch history (tens of thousands of entries) would otherwise serialize
// dozens of page requests.
const historyConcurrency = 8

func (s *Server) fetchAllWatchHistory(ctx context.Context, libraryID string) (movieHistory, seriesHistory map[string]time.Time, err error) {
	movieHistory = make(map[string]time.Time)
	seriesHistory = make(map[string]time.Time)
	accountIDs := make(map[string]bool)

	// Fetch page 0 first to learn the total, then fan out the remaining pages.
	first, err := s.fetchHistoryBatch(ctx, libraryID, 0)
	if err != nil {
		return nil, nil, err
	}
	mergeHistoryPage(first.Videos, movieHistory, seriesHistory, accountIDs)

	total := first.Size
	if total > historyMaxEntries {
		total = historyMaxEntries
	}

	if len(first.Videos) >= historyBatchSize && len(first.Videos) < total {
		var offsets []int
		for off := historyBatchSize; off < total; off += historyBatchSize {
			offsets = append(offsets, off)
		}
		pages := make([][]historyItemXML, len(offsets))
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(historyConcurrency)
		for idx := range offsets {
			idx := idx
			off := offsets[idx]
			g.Go(func() error {
				c, ferr := s.fetchHistoryBatch(gctx, libraryID, off)
				if ferr != nil {
					return ferr
				}
				pages[idx] = c.Videos
				mediautil.SendProgress(gctx, mediautil.SyncProgress{
					Phase: mediautil.PhaseHistory, Current: off, Total: total, Library: libraryID,
				})
				return nil
			})
		}
		if werr := g.Wait(); werr != nil {
			return nil, nil, werr
		}
		// Merge sequentially (single goroutine) so the shared maps stay race-free.
		for _, vids := range pages {
			mergeHistoryPage(vids, movieHistory, seriesHistory, accountIDs)
		}
	}

	slog.Info("plex watch history fetched",
		"library", libraryID,
		"accounts", len(accountIDs),
		"movies", len(movieHistory),
		"series", len(seriesHistory),
		"total_entries", total)
	return movieHistory, seriesHistory, nil
}

// mergeHistoryPage folds one page of history Videos into the running maps,
// keeping the most recent ViewedAt per movie ratingKey / series grandparent.
func mergeHistoryPage(videos []historyItemXML, movieHistory, seriesHistory map[string]time.Time, accountIDs map[string]bool) {
	for _, v := range videos {
		ts := atoi64(v.ViewedAt)
		if ts <= 0 {
			continue
		}
		t := time.Unix(ts, 0).UTC()
		if v.AccountID != "" {
			accountIDs[v.AccountID] = true
		}

		gpRatingKey := ratingKeyFromPath(v.GrandparentKey)
		if gpRatingKey == "" {
			gpRatingKey = v.GrandparentRatingKey
		}
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

// fetchHistory makes a GET request to /status/sessions/history/all with the given
// query parameters and returns the parsed XML container.
func (s *Server) fetchHistory(ctx context.Context, params url.Values) (*historyContainer, error) {
	u, err := url.Parse(fmt.Sprintf("%s/status/sessions/history/all", s.url))
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("sort", "viewedAt:desc")
	for k, vs := range params {
		for _, v := range vs {
			q.Set(k, v)
		}
	}
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

func (s *Server) fetchHistoryBatch(ctx context.Context, libraryID string, offset int) (*historyContainer, error) {
	return s.fetchHistory(ctx, url.Values{
		"librarySectionID":       {libraryID},
		"X-Plex-Container-Start": {strconv.Itoa(offset)},
		"X-Plex-Container-Size":  {strconv.Itoa(historyBatchSize)},
	})
}

// enrichConcurrency bounds the parallel per-item history lookups. A large
// library can have thousands of never-watched items, each needing its own
// round-trip; running them sequentially dominated sync time, so we fan out
// with a small worker pool.
const enrichConcurrency = 12

// enrichMissedWatchHistory queries Plex per-item for any items that still have
// nil LastWatchedAt after the bulk history fetch. Uses metadataItemID to let
// Plex match episodes to shows server-side, avoiding grandparentKey parsing.
func (s *Server) enrichMissedWatchHistory(ctx context.Context, items []models.LibraryItemCache, libraryID string) {
	var todo []int
	for i := range items {
		if items[i].LastWatchedAt == nil {
			todo = append(todo, i)
		}
	}
	if len(todo) == 0 {
		return
	}

	var g errgroup.Group
	g.SetLimit(enrichConcurrency)
	var recovered, done int64
	total := len(todo)

	for _, n := range todo {
		if ctx.Err() != nil {
			break
		}
		i := n // each goroutine writes a distinct items[i], so no shared-element race
		g.Go(func() error {
			t, err := s.fetchItemLastWatched(ctx, items[i].ItemID)
			if err != nil {
				slog.Warn("per-item history check failed", "item", items[i].Title, "error", err)
			} else if t != nil {
				items[i].LastWatchedAt = t
				atomic.AddInt64(&recovered, 1)
			}
			mediautil.SendProgress(ctx, mediautil.SyncProgress{
				Phase:   mediautil.PhaseEnriching,
				Current: int(atomic.AddInt64(&done, 1)),
				Total:   total,
				Library: libraryID,
			})
			return nil
		})
	}
	_ = g.Wait()

	if recovered > 0 {
		slog.Info("per-item history recovered watches",
			"library", libraryID, "recovered", recovered, "lookups", total)
	}
}

// fetchItemLastWatched returns the most recent watch time for a specific item
// across all users, or nil if never watched. Uses the metadataItemID parameter
// which Plex resolves to include child items (episodes for a show).
func (s *Server) fetchItemLastWatched(ctx context.Context, itemID string) (*time.Time, error) {
	container, err := s.fetchHistory(ctx, url.Values{
		"metadataItemID":         {itemID},
		"X-Plex-Container-Start": {"0"},
		"X-Plex-Container-Size":  {"1"},
	})
	if err != nil {
		return nil, err
	}

	if len(container.Videos) == 0 {
		return nil, nil
	}

	ts := atoi64(container.Videos[0].ViewedAt)
	if ts <= 0 {
		return nil, nil
	}
	t := time.Unix(ts, 0).UTC()
	return &t, nil
}
