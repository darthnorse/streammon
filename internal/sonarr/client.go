package sonarr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"streammon/internal/arrutil"
	"streammon/internal/httputil"
)

// ValidateURL checks that the given URL is valid for use as a Sonarr endpoint.
var ValidateURL = httputil.ValidateIntegrationURL

type Client struct {
	arrutil.Client
}

func NewClient(baseURL, apiKey string) (*Client, error) {
	arr, err := arrutil.New("Sonarr", baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return &Client{Client: *arr}, nil
}

type seriesResult struct {
	ID int `json:"id"`
}

// LookupSeriesByTVDB finds a series in Sonarr by its TVDB ID.
// Returns the Sonarr internal ID, or 0 if not found.
func (c *Client) LookupSeriesByTVDB(ctx context.Context, tvdbID string) (int, error) {
	raw, err := c.DoGet(ctx, "/series", url.Values{"tvdbId": {tvdbID}})
	if err != nil {
		return 0, err
	}

	var series []seriesResult
	if err := json.Unmarshal(raw, &series); err != nil {
		return 0, fmt.Errorf("parsing series list: %w", err)
	}
	if len(series) == 0 {
		return 0, nil
	}
	return series[0].ID, nil
}

// DeleteSeries removes a series from Sonarr, optionally deleting files.
func (c *Client) DeleteSeries(ctx context.Context, seriesID int, deleteFiles bool) error {
	q := url.Values{}
	if deleteFiles {
		q.Set("deleteFiles", "true")
	}
	return c.DoDelete(ctx, fmt.Sprintf("/series/%d", seriesID), q)
}

func (c *Client) GetSeries(ctx context.Context, seriesID int) (json.RawMessage, error) {
	return c.DoGet(ctx, fmt.Sprintf("/series/%d", seriesID), nil)
}

func (c *Client) UpdateSeries(ctx context.Context, seriesID int, data json.RawMessage) error {
	_, err := c.DoPut(ctx, fmt.Sprintf("/series/%d", seriesID), data)
	return err
}

func (c *Client) GetCalendar(ctx context.Context, start, end string) (json.RawMessage, error) {
	params := url.Values{}
	if start != "" {
		params.Set("start", start)
	}
	if end != "" {
		params.Set("end", end)
	}
	params.Set("includeSeries", "true")
	params.Set("includeEpisodeImages", "true")
	return c.DoGet(ctx, "/calendar", params)
}
