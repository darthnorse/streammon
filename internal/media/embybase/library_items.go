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
	const batchSize = 100
	var allItems []models.LibraryItemCache
	offset := 0

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		items, totalCount, err := c.fetchLibraryBatch(ctx, libraryID, offset, batchSize)
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

func (c *Client) fetchLibraryBatch(ctx context.Context, libraryID string, offset, batchSize int) ([]models.LibraryItemCache, int, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/Items", c.url))
	q := u.Query()
	q.Set("ParentId", libraryID)
	q.Set("Recursive", "true")
	q.Set("IncludeItemTypes", "Movie,Series")
	q.Set("Fields", "DateCreated,MediaSources,RecursiveItemCount")
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
					resolution = heightToRes(stream.Height)
					break
				}
			}
		}

		mediaType := models.MediaTypeMovie
		if item.Type == "Series" {
			mediaType = models.MediaTypeTV
		}

		addedAt := parseEmbyTime(item.DateCreated)

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
			EpisodeCount:    item.RecursiveItemCount,
			ThumbURL:        item.ID,
		})
	}

	return items, itemsResp.TotalRecordCount, nil
}

func heightToRes(height int) string {
	switch {
	case height >= 2160:
		return "4K"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 480:
		return "480p"
	default:
		return ""
	}
}
