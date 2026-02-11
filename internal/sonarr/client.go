package sonarr

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

func (c *Client) doGet(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	u := c.baseURL + "/api/v3" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer httputil.DrainBody(resp)

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Sonarr returned status %d: %s", resp.StatusCode, truncate(body, 200))
	}

	return json.RawMessage(body), nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.doGet(ctx, "/system/status", nil)
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
	return c.doGet(ctx, "/calendar", params)
}

func truncate(b []byte, max int) string {
	r := []rune(string(b))
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return string(r)
}
