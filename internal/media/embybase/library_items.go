package embybase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"streammon/internal/httputil"
	"streammon/internal/mediautil"
	"streammon/internal/models"
)

type libraryItemsCacheResponse struct {
	Items            []embyLibraryItem `json:"Items"`
	TotalRecordCount int               `json:"TotalRecordCount"`
}

type embyLibraryItem struct {
	ID                 string            `json:"Id"`
	Name               string            `json:"Name"`
	Type               string            `json:"Type"`
	ProductionYear     int               `json:"ProductionYear"`
	DateCreated        string            `json:"DateCreated"`
	RecursiveItemCount int               `json:"RecursiveItemCount"`
	ChildCount         int               `json:"ChildCount"`
	MediaSources       []embyMediaSource `json:"MediaSources,omitempty"`
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

	// For series without file sizes, fetch episode sizes
	for i := range series {
		if series[i].FileSize == 0 {
			size, err := c.getSeriesEpisodeSize(ctx, series[i].ItemID)
			if err == nil {
				series[i].FileSize = size
			}
		}
	}

	result := make([]models.LibraryItemCache, 0, len(movies)+len(series))
	result = append(result, movies...)
	result = append(result, series...)
	return result, nil
}

func (c *Client) getSeriesEpisodeSize(ctx context.Context, seriesID string) (int64, error) {
	var totalSize int64
	offset := 0
	const batchSize = 100

	for {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}

		u, err := url.Parse(fmt.Sprintf("%s/Items", c.url))
		if err != nil {
			return 0, err
		}
		q := u.Query()
		q.Set("ParentId", seriesID)
		q.Set("Recursive", "true")
		q.Set("IncludeItemTypes", "Episode")
		q.Set("Fields", "MediaSources")
		q.Set("StartIndex", strconv.Itoa(offset))
		q.Set("Limit", strconv.Itoa(batchSize))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return 0, err
		}

		resp, err := c.client.Do(c.addAuth(req))
		if err != nil {
			return 0, err
		}

		if resp.StatusCode != http.StatusOK {
			httputil.DrainBody(resp)
			return 0, fmt.Errorf("fetch episodes: status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
		httputil.DrainBody(resp)
		if err != nil {
			return 0, err
		}

		var episodesResp libraryItemsCacheResponse
		if err := json.Unmarshal(body, &episodesResp); err != nil {
			return 0, err
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

func (c *Client) fetchLibraryItemsByType(ctx context.Context, libraryID, itemType string) ([]models.LibraryItemCache, error) {
	const batchSize = 100
	var allItems []models.LibraryItemCache
	offset := 0

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		items, totalCount, err := c.fetchLibraryBatch(ctx, libraryID, itemType, offset, batchSize)
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

func (c *Client) fetchLibraryBatch(ctx context.Context, libraryID, itemType string, offset, batchSize int) ([]models.LibraryItemCache, int, error) {
	u, err := url.Parse(fmt.Sprintf("%s/Items", c.url))
	if err != nil {
		return nil, 0, fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("ParentId", libraryID)
	q.Set("Recursive", "true")
	q.Set("IncludeItemTypes", itemType)
	q.Set("Fields", "DateCreated,MediaSources,RecursiveItemCount,ChildCount")
	q.Set("StartIndex", strconv.Itoa(offset))
	q.Set("Limit", strconv.Itoa(batchSize))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return nil, 0, fmt.Errorf("%s library items: %w", c.serverType, err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("%s library items: status %d", c.serverType, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, 0, err
	}

	var itemsResp libraryItemsCacheResponse
	if err := json.Unmarshal(body, &itemsResp); err != nil {
		return nil, 0, fmt.Errorf("parse library items: %w", err)
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
			VideoResolution: resolution,
			FileSize:        fileSize,
			EpisodeCount:    episodeCount,
			ThumbURL:        item.ID,
		})
	}

	return items, itemsResp.TotalRecordCount, nil
}
