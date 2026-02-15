package radarr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"streammon/internal/arrutil"
	"streammon/internal/httputil"
)

// ValidateURL checks that the given URL is valid for use as a Radarr endpoint.
var ValidateURL = httputil.ValidateIntegrationURL

type Client struct {
	arrutil.Client
}

func NewClient(baseURL, apiKey string) (*Client, error) {
	arr, err := arrutil.New("Radarr", baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return &Client{Client: *arr}, nil
}

type movieResult struct {
	ID int `json:"id"`
}

// LookupMovieByTMDB finds a movie in Radarr by its TMDB ID.
// Returns the Radarr internal ID, or 0 if not found.
func (c *Client) LookupMovieByTMDB(ctx context.Context, tmdbID string) (int, error) {
	raw, err := c.DoGet(ctx, "/movie", url.Values{"tmdbId": {tmdbID}})
	if err != nil {
		return 0, err
	}

	var movies []movieResult
	if err := json.Unmarshal(raw, &movies); err != nil {
		return 0, fmt.Errorf("parsing movie list: %w", err)
	}
	if len(movies) == 0 {
		return 0, nil
	}
	return movies[0].ID, nil
}

// DeleteMovie removes a movie from Radarr, optionally deleting files.
func (c *Client) DeleteMovie(ctx context.Context, movieID int, deleteFiles bool) error {
	q := url.Values{}
	if deleteFiles {
		q.Set("deleteFiles", "true")
	}
	return c.DoDelete(ctx, fmt.Sprintf("/movie/%d", movieID), q)
}
