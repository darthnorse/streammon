package overseerr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"streammon/internal/httputil"
)

const maxResponseBody = 2 << 20 // 2 MiB

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if err := ValidateURL(baseURL); err != nil {
		return nil, err
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader) (json.RawMessage, error) {
	u := c.baseURL + "/api/v1" + path
	if len(query) > 0 {
		u += "?" + strings.ReplaceAll(query.Encode(), "+", "%20")
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer httputil.DrainBody(resp)

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Overseerr returned status %d: %s", resp.StatusCode, truncate(respBody, 200))
	}

	return json.RawMessage(respBody), nil
}

func (c *Client) doGet(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) doPost(ctx context.Context, path string, payload json.RawMessage) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		body = strings.NewReader(string(payload))
	}
	return c.do(ctx, http.MethodPost, path, nil, body)
}

func (c *Client) doDelete(ctx context.Context, path string) error {
	_, err := c.do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doGet(ctx, "/status", nil)
	return err
}

func (c *Client) Search(ctx context.Context, query string, page int) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("query", query)
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	return c.doGet(ctx, "/search", params)
}

func (c *Client) Discover(ctx context.Context, category string, page int) (json.RawMessage, error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	return c.doGet(ctx, "/discover/"+category, params)
}

func (c *Client) GetMovie(ctx context.Context, movieID int) (json.RawMessage, error) {
	return c.doGet(ctx, fmt.Sprintf("/movie/%d", movieID), nil)
}

func (c *Client) GetTV(ctx context.Context, tvID int) (json.RawMessage, error) {
	return c.doGet(ctx, fmt.Sprintf("/tv/%d", tvID), nil)
}

func (c *Client) GetTVSeason(ctx context.Context, tvID, seasonNumber int) (json.RawMessage, error) {
	return c.doGet(ctx, fmt.Sprintf("/tv/%d/season/%d", tvID, seasonNumber), nil)
}

func (c *Client) ListRequests(ctx context.Context, take, skip int, filter, sort string) (json.RawMessage, error) {
	params := url.Values{}
	if take > 0 {
		params.Set("take", fmt.Sprintf("%d", take))
	}
	if skip > 0 {
		params.Set("skip", fmt.Sprintf("%d", skip))
	}
	if filter != "" {
		params.Set("filter", filter)
	}
	if sort != "" {
		params.Set("sort", sort)
	}
	return c.doGet(ctx, "/request", params)
}

func (c *Client) RequestCount(ctx context.Context) (json.RawMessage, error) {
	return c.doGet(ctx, "/request/count", nil)
}

func (c *Client) CreateRequest(ctx context.Context, reqBody json.RawMessage) (json.RawMessage, error) {
	return c.doPost(ctx, "/request", reqBody)
}

func (c *Client) UpdateRequestStatus(ctx context.Context, requestID int, status string) (json.RawMessage, error) {
	if status != "approve" && status != "decline" {
		return nil, fmt.Errorf("invalid status: must be 'approve' or 'decline'")
	}
	return c.doPost(ctx, fmt.Sprintf("/request/%d/%s", requestID, status), nil)
}

func (c *Client) DeleteRequest(ctx context.Context, requestID int) error {
	return c.doDelete(ctx, fmt.Sprintf("/request/%d", requestID))
}

func truncate(b []byte, max int) string {
	r := []rune(string(b))
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return string(r)
}
