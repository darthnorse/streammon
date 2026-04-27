package embybase

import (
	"context"
	"fmt"
	"net/url"

	"streammon/internal/models"
)

func (c *Client) GetEpisodes(ctx context.Context, seasonID string) ([]models.Episode, error) {
	params := url.Values{
		"ParentId":         {seasonID},
		"IncludeItemTypes": {"Episode"},
		"Fields":           {"Overview,IndexNumber,RunTimeTicks,PremiereDate,ImageTags"},
		"SortBy":           {"IndexNumber"},
	}

	var resp struct {
		Items []struct {
			ID           string            `json:"Id"`
			Name         string            `json:"Name"`
			IndexNumber  int               `json:"IndexNumber"`
			Overview     string            `json:"Overview"`
			RunTimeTicks int64             `json:"RunTimeTicks"`
			PremiereDate string            `json:"PremiereDate"`
			ImageTags    map[string]string `json:"ImageTags"`
		} `json:"Items"`
	}
	if err := c.fetchItemsPage(ctx, params, &resp); err != nil {
		return nil, fmt.Errorf("%s episodes: %w", c.serverType, err)
	}

	episodes := make([]models.Episode, 0, len(resp.Items))
	for _, item := range resp.Items {
		airDate := ""
		if len(item.PremiereDate) >= 10 {
			airDate = item.PremiereDate[:10]
		}
		episodes = append(episodes, models.Episode{
			ID:         item.ID,
			Number:     item.IndexNumber,
			Title:      item.Name,
			Summary:    item.Overview,
			ThumbURL:   resolveThumbURL("", "", item.ID, item.ImageTags),
			DurationMs: item.RunTimeTicks / 10000,
			AirDate:    airDate,
		})
	}
	return episodes, nil
}
