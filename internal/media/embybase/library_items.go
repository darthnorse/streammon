package embybase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	itemBatchSize    = 100
	historyBatchSize = 2000
	maxResponseBody  = 50 << 20 // 50 MB
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
	mediautil.EnrichSeriesData(ctx, series, libraryID, string(c.serverType), c.getSeriesEpisodeSize)

	movieHistory, seriesHistory, err := c.fetchAllUsersWatchData(ctx, libraryID)
	if err != nil {
		slog.Warn("failed to fetch all-user watch data, using per-item data only",
			"server_type", string(c.serverType), "error", err)
	} else {
		mediautil.EnrichLastWatched(movies, movieHistory)
		mediautil.EnrichLastWatched(series, seriesHistory)
	}

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
	var total int
	offset := 0

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		items, totalCount, err := c.fetchLibraryBatch(ctx, libraryID, itemType, offset, itemBatchSize)
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

func (c *Client) fetchLibraryBatch(ctx context.Context, libraryID, itemType string, offset, batchSize int) ([]models.LibraryItemCache, int, error) {
	params := url.Values{
		"ParentId":         {libraryID},
		"Recursive":        {"true"},
		"IncludeItemTypes": {itemType},
		"Fields":           {"DateCreated,ProductionYear,MediaSources,RecursiveItemCount,ChildCount,UserData,ProviderIds"},
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

// fetchAllUsersWatchData iterates all server users and collects the latest
// watch time for each movie and series across ALL users, not just the API user.
func (c *Client) fetchAllUsersWatchData(ctx context.Context, libraryID string) (movieHistory, seriesHistory map[string]time.Time, err error) {
	users, err := c.fetchUsers(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch users: %w", err)
	}

	movieHistory = make(map[string]time.Time)
	seriesHistory = make(map[string]time.Time)

	for i, user := range users {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseHistory,
			Current: i + 1,
			Total:   len(users),
			Library: libraryID,
		})

		movies, err := c.fetchPlayedItems(ctx, libraryID, user.ID, "Movie")
		if err != nil {
			slog.Warn("failed to fetch played movies for user",
				"user", user.Name, "server_type", string(c.serverType), "error", err)
		} else {
			for _, item := range movies {
				if item.UserData == nil || item.UserData.LastPlayedDate == "" {
					continue
				}
				t := parseEmbyTime(item.UserData.LastPlayedDate)
				if !t.IsZero() {
					if existing, ok := movieHistory[item.ID]; !ok || t.After(existing) {
						movieHistory[item.ID] = t
					}
				}
			}
		}

		episodes, err := c.fetchPlayedItems(ctx, libraryID, user.ID, "Episode")
		if err != nil {
			slog.Warn("failed to fetch played episodes for user",
				"user", user.Name, "server_type", string(c.serverType), "error", err)
		} else {
			for _, ep := range episodes {
				if ep.SeriesId == "" || ep.UserData == nil || ep.UserData.LastPlayedDate == "" {
					continue
				}
				t := parseEmbyTime(ep.UserData.LastPlayedDate)
				if !t.IsZero() {
					if existing, ok := seriesHistory[ep.SeriesId]; !ok || t.After(existing) {
						seriesHistory[ep.SeriesId] = t
					}
				}
			}
		}
	}

	slog.Info("all-user watch data fetched",
		"server_type", string(c.serverType),
		"library", libraryID,
		"users", len(users),
		"movies", len(movieHistory),
		"series", len(seriesHistory))
	return movieHistory, seriesHistory, nil
}

// fetchPlayedItems returns all played items of a given type for a specific user.
func (c *Client) fetchPlayedItems(ctx context.Context, libraryID, userID, itemType string) ([]embyLibraryItem, error) {
	var allItems []embyLibraryItem
	offset := 0

	fields := "UserData"
	if itemType == "Episode" {
		fields = "UserData,SeriesId"
	}

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		params := url.Values{
			"ParentId":         {libraryID},
			"Recursive":        {"true"},
			"IncludeItemTypes": {itemType},
			"Filters":          {"IsPlayed"},
			"Fields":           {fields},
			"UserId":           {userID},
			"StartIndex":       {strconv.Itoa(offset)},
			"Limit":            {strconv.Itoa(historyBatchSize)},
		}

		var resp libraryItemsCacheResponse
		if err := c.fetchItemsPage(ctx, params, &resp); err != nil {
			return nil, err
		}

		if len(resp.Items) == 0 {
			break
		}

		allItems = append(allItems, resp.Items...)
		offset += len(resp.Items)
		if offset >= resp.TotalRecordCount {
			break
		}
	}

	return allItems, nil
}

