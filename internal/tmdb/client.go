package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"golang.org/x/time/rate"

	"streammon/internal/httputil"
	"streammon/internal/store"
)

const (
	defaultBaseURL = "https://api.themoviedb.org/3"

	defaultAPIKey = "276b2b50d57b53c3a05796e412a7445e"
)

type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
	store   *store.Store
	limiter *rate.Limiter
}

func New(apiKey string, store *store.Store) *Client {
	if apiKey == "" {
		apiKey = defaultAPIKey
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    httputil.NewClientWithTimeout(httputil.IntegrationTimeout),
		store:   store,
		limiter: rate.NewLimiter(35, 10),
	}
}

func NewWithBaseURL(apiKey string, store *store.Store, baseURL string) *Client {
	c := New(apiKey, store)
	c.baseURL = baseURL
	c.limiter = rate.NewLimiter(rate.Inf, 0)
	return c
}

func (c *Client) do(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("api_key", c.apiKey)
	u := c.baseURL + path + "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer httputil.DrainBody(resp)

	body, err := io.ReadAll(io.LimitReader(resp.Body, httputil.MaxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("TMDB returned status %d: %s", resp.StatusCode, httputil.Truncate(body, 200))
	}

	return json.RawMessage(body), nil
}

func (c *Client) cached(ctx context.Context, cacheKey, path string, query url.Values) (json.RawMessage, error) {
	if c.store != nil {
		if data, err := c.store.GetCachedTMDB(cacheKey); err == nil && data != nil {
			return data, nil
		}
	}

	data, err := c.do(ctx, path, query)
	if err != nil {
		return nil, err
	}

	if c.store != nil {
		_ = c.store.SetCachedTMDB(cacheKey, data)
	}

	return data, nil
}

func (c *Client) pagedList(ctx context.Context, cachePrefix, path string, page int) (json.RawMessage, error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	return c.cached(ctx, fmt.Sprintf("%s:%d", cachePrefix, page), path, params)
}

func (c *Client) Search(ctx context.Context, query string, page int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("query", query)
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	cacheKey := fmt.Sprintf("search:%s:%d", url.QueryEscape(query), page)
	return c.cached(ctx, cacheKey, "/search/multi", params)
}

func (c *Client) Trending(ctx context.Context, page int) (json.RawMessage, error) {
	return c.pagedList(ctx, "trending", "/trending/all/week", page)
}

func (c *Client) PopularMovies(ctx context.Context, page int) (json.RawMessage, error) {
	return c.pagedList(ctx, "movies/popular", "/movie/popular", page)
}

func (c *Client) UpcomingMovies(ctx context.Context, page int) (json.RawMessage, error) {
	return c.pagedList(ctx, "movies/upcoming", "/movie/upcoming", page)
}

func (c *Client) PopularTV(ctx context.Context, page int) (json.RawMessage, error) {
	return c.pagedList(ctx, "tv/popular", "/tv/popular", page)
}

func (c *Client) UpcomingTV(ctx context.Context, page int) (json.RawMessage, error) {
	return c.pagedList(ctx, "tv/upcoming", "/tv/on_the_air", page)
}

func (c *Client) GetMovie(ctx context.Context, id int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("append_to_response", "credits")
	return c.cached(ctx, fmt.Sprintf("movie:%d", id), fmt.Sprintf("/movie/%d", id), params)
}

func (c *Client) GetTV(ctx context.Context, id int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("append_to_response", "credits")
	return c.cached(ctx, fmt.Sprintf("tv:%d", id), fmt.Sprintf("/tv/%d", id), params)
}

func (c *Client) GetPerson(ctx context.Context, id int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("append_to_response", "combined_credits")
	return c.cached(ctx, fmt.Sprintf("person:%d", id), fmt.Sprintf("/person/%d", id), params)
}

func (c *Client) GetCollection(ctx context.Context, id int) (json.RawMessage, error) {
	return c.cached(ctx, fmt.Sprintf("collection:%d", id), fmt.Sprintf("/collection/%d", id), nil)
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.do(ctx, "/configuration", nil)
	return err
}
