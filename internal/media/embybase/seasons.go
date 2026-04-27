package embybase

import (
	"context"
	"fmt"
	"net/url"

	"streammon/internal/models"
)

func (c *Client) GetSeasons(ctx context.Context, showID string) ([]models.Season, error) {
	params := url.Values{
		"ParentId":         {showID},
		"IncludeItemTypes": {"Season"},
		"Fields":           {"IndexNumber,ChildCount,ProductionYear,ImageTags"},
		"SortBy":           {"IndexNumber"},
	}

	var resp struct {
		Items []struct {
			ID             string            `json:"Id"`
			Name           string            `json:"Name"`
			IndexNumber    int               `json:"IndexNumber"`
			ChildCount     int               `json:"ChildCount"`
			ProductionYear int               `json:"ProductionYear"`
			ImageTags      map[string]string `json:"ImageTags"`
		} `json:"Items"`
	}
	if err := c.fetchItemsPage(ctx, params, &resp); err != nil {
		return nil, fmt.Errorf("%s seasons: %w", c.serverType, err)
	}

	seasons := make([]models.Season, 0, len(resp.Items))
	for _, item := range resp.Items {
		seasons = append(seasons, models.Season{
			ID:           item.ID,
			Number:       item.IndexNumber,
			Title:        item.Name,
			ThumbURL:     resolveThumbURL("", "", item.ID, item.ImageTags),
			EpisodeCount: item.ChildCount,
			Year:         item.ProductionYear,
		})
	}
	return seasons, nil
}
