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
		"Fields":           {"IndexNumber"},
	}

	var resp struct {
		Items []struct {
			ID          string `json:"Id"`
			Name        string `json:"Name"`
			IndexNumber int    `json:"IndexNumber"`
		} `json:"Items"`
	}
	if err := c.fetchItemsPage(ctx, params, &resp); err != nil {
		return nil, fmt.Errorf("%s seasons: %w", c.serverType, err)
	}

	seasons := make([]models.Season, 0, len(resp.Items))
	for _, item := range resp.Items {
		seasons = append(seasons, models.Season{
			ID:     item.ID,
			Number: item.IndexNumber,
			Title:  item.Name,
		})
	}
	return seasons, nil
}
