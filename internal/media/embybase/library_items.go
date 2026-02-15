package embybase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"streammon/internal/httputil"
	"streammon/internal/mediautil"
	"streammon/internal/models"
)

const (
	itemBatchSize     = 100
	historyBatchSize  = 2000
	historyMaxEntries = 5000000
	maxResponseBody   = 50 << 20 // 50 MB
)

type libraryItemsCacheResponse struct {
	Items            []embyLibraryItem `json:"Items"`
	TotalRecordCount int               `json:"TotalRecordCount"`
}

type embyLibraryItem struct {
	ID                 string            `json:"Id"`
	Name               string            `json:"Name"`
	Type               string            `json:"Type"`
	SeriesId           string            `json:"SeriesId"`
	ProductionYear     int               `json:"ProductionYear"`
	DateCreated        string            `json:"DateCreated"`
	RecursiveItemCount int               `json:"RecursiveItemCount"`
	ChildCount         int               `json:"ChildCount"`
	MediaSources       []embyMediaSource `json:"MediaSources,omitempty"`
	UserData           *embyUserData     `json:"UserData,omitempty"`
	ProviderIds        map[string]string `json:"ProviderIds,omitempty"`
}

type embyUserData struct {
	LastPlayedDate string `json:"LastPlayedDate"`
	Played         bool   `json:"Played"`
}

type embyMediaSource struct {
	Size    int64        `json:"Size"`
	Streams []embyStream `json:"MediaStreams,omitempty"`
}

type embyStream struct {
	Type   string `json:"Type"`
	Height int    `json:"Height"`
	Width  int    `json:"Width"`
}

func (c *Client) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	movies, err := c.fetchLibraryItemsByType(ctx, libraryID, "Movie")
	if err != nil {
		return nil, fmt.Errorf("fetch movies: %w", err)
	}

	series, err := c.fetchLibraryItemsByType(ctx, libraryID, "Series")
	if err != nil {
		return nil, fmt.Errorf("fetch series: %w", err)
	}
	mediautil.EnrichSeriesData(ctx, series, libraryID, string(c.serverType), c.getSeriesEpisodeSize, c.fetchSeriesWatchHistory)

	result := slices.Concat(movies, series)
	if result == nil {
		return []models.LibraryItemCache{}, nil
	}

	mediautil.LogSyncSummary(string(c.serverType), libraryID, len(movies), len(series), result)
	return result, nil
}

func (c *Client) getSeriesEpisodeSize(ctx context.Context, seriesID string) (int64, error) {
	var totalSize int64
	offset := 0

	for {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}

		params := url.Values{
			"ParentId":         {seriesID},
			"Recursive":        {"true"},
			"IncludeItemTypes": {"Episode"},
			"Fields":           {"MediaSources"},
			"StartIndex":       {strconv.Itoa(offset)},
			"Limit":            {strconv.Itoa(itemBatchSize)},
		}

		var episodesResp libraryItemsCacheResponse
		if err := c.fetchItemsPage(ctx, params, &episodesResp); err != nil {
			return 0, fmt.Errorf("fetch episodes: %w", err)
		}

		if len(episodesResp.Items) == 0 {
			break
		}

		for _, ep := range episodesResp.Items {
			if len(ep.MediaSources) > 0 {
				totalSize += ep.MediaSources[0].Size
			}
		}

		offset += len(episodesResp.Items)
		if offset >= episodesResp.TotalRecordCount {
			break
		}
	}

	return totalSize, nil
}

func (c *Client) fetchItemsPage(ctx context.Context, params url.Values, result any) error {
	u, err := url.Parse(fmt.Sprintf("%s/Items", c.url))
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return err
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return err
	}

	return json.Unmarshal(body, result)
}

func (c *Client) fetchLibraryItemsByType(ctx context.Context, libraryID, itemType string) ([]models.LibraryItemCache, error) {
	var allItems []models.LibraryItemCache
	offset := 0

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		items, _, err := c.fetchLibraryBatch(ctx, libraryID, itemType, offset, itemBatchSize)
		if err != nil {
			return nil, err
		}

		if len(items) == 0 {
			break
		}

		allItems = append(allItems, items...)
		offset += len(items)
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseItems,
			Current: offset,
			Library: libraryID,
		})

		if len(items) < itemBatchSize {
			break
		}
	}

	return allItems, nil
}

func (c *Client) fetchLibraryBatch(ctx context.Context, libraryID, itemType string, offset, batchSize int) ([]models.LibraryItemCache, int, error) {
	params := url.Values{
		"ParentId":         {libraryID},
		"Recursive":        {"true"},
		"IncludeItemTypes": {itemType},
		"Fields":           {"DateCreated,MediaSources,RecursiveItemCount,ChildCount,UserData,ProviderIds"},
		"StartIndex":       {strconv.Itoa(offset)},
		"Limit":            {strconv.Itoa(batchSize)},
	}

	var itemsResp libraryItemsCacheResponse
	if err := c.fetchItemsPage(ctx, params, &itemsResp); err != nil {
		return nil, 0, fmt.Errorf("%s library items: %w", c.serverType, err)
	}

	var items []models.LibraryItemCache
	for _, item := range itemsResp.Items {
		var resolution string
		var fileSize int64

		if len(item.MediaSources) > 0 {
			fileSize = item.MediaSources[0].Size
			for _, stream := range item.MediaSources[0].Streams {
				if stream.Type == "Video" && stream.Height > 0 {
					resolution = mediautil.HeightToResolution(stream.Height)
					break
				}
			}
		}

		mediaType := models.MediaTypeMovie
		if item.Type == "Series" {
			mediaType = models.MediaTypeTV
		}

		addedAt := parseEmbyTime(item.DateCreated)
		if addedAt.IsZero() {
			addedAt = time.Now().UTC()
		}

		var lastWatchedAt *time.Time
		if item.UserData != nil && item.UserData.LastPlayedDate != "" {
			t := parseEmbyTime(item.UserData.LastPlayedDate)
			if !t.IsZero() {
				lastWatchedAt = &t
			}
		}

		episodeCount := item.RecursiveItemCount
		if episodeCount == 0 {
			episodeCount = item.ChildCount
		}

		items = append(items, models.LibraryItemCache{
			ServerID:        c.serverID,
			LibraryID:       libraryID,
			ItemID:          item.ID,
			MediaType:       mediaType,
			Title:           item.Name,
			Year:            item.ProductionYear,
			AddedAt:         addedAt,
			LastWatchedAt:   lastWatchedAt,
			VideoResolution: resolution,
			FileSize:        fileSize,
			EpisodeCount:    episodeCount,
			ThumbURL:        item.ID,
			TMDBID:          item.ProviderIds["Tmdb"],
			TVDBID:          item.ProviderIds["Tvdb"],
			IMDBID:          item.ProviderIds["Imdb"],
		})
	}

	return items, itemsResp.TotalRecordCount, nil
}

func (c *Client) fetchSeriesWatchHistory(ctx context.Context, libraryID string) (map[string]time.Time, error) {
	result := make(map[string]time.Time)
	offset := 0

	for offset < historyMaxEntries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		params := url.Values{
			"ParentId":         {libraryID},
			"Recursive":        {"true"},
			"IncludeItemTypes": {"Episode"},
			"Filters":          {"IsPlayed"},
			"Fields":           {"UserData"},
			"SortBy":           {"DatePlayed"},
			"SortOrder":        {"Descending"},
			"StartIndex":       {strconv.Itoa(offset)},
			"Limit":            {strconv.Itoa(historyBatchSize)},
		}

		var resp libraryItemsCacheResponse
		if err := c.fetchItemsPage(ctx, params, &resp); err != nil {
			return nil, fmt.Errorf("fetch episode history: %w", err)
		}

		if len(resp.Items) == 0 {
			break
		}

		for _, ep := range resp.Items {
			if ep.SeriesId == "" {
				continue
			}
			if _, exists := result[ep.SeriesId]; !exists {
				if ep.UserData != nil && ep.UserData.LastPlayedDate != "" {
					t := parseEmbyTime(ep.UserData.LastPlayedDate)
					if !t.IsZero() {
						result[ep.SeriesId] = t
					}
				}
			}
		}

		offset += len(resp.Items)
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseHistory,
			Current: offset,
			Total:   resp.TotalRecordCount,
			Library: libraryID,
		})
		if offset >= resp.TotalRecordCount {
			break
		}
	}

	return result, nil
}

