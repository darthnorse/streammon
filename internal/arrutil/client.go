package arrutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"streammon/internal/httputil"
)

// Client provides shared HTTP plumbing for Sonarr/Radarr v3 APIs.
type Client struct {
	BaseURL string
	APIKey  string
	Name    string
	HTTP    *http.Client
}

func New(name, baseURL, apiKey string) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	if err := httputil.ValidateIntegrationURL(baseURL); err != nil {
		return nil, err
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Name:    name,
		HTTP:    httputil.NewClientWithTimeout(httputil.IntegrationTimeout),
	}, nil
}

func (c *Client) DoGet(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	u := c.BaseURL + "/api/v3" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer httputil.DrainBody(resp)

	body, err := io.ReadAll(io.LimitReader(resp.Body, httputil.MaxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned status %d: %s", c.Name, resp.StatusCode, httputil.Truncate(body, 200))
	}

	return json.RawMessage(body), nil
}

func (c *Client) DoDelete(ctx context.Context, path string, query url.Values) error {
	u := c.BaseURL + "/api/v3" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, httputil.MaxResponseBody))
		return fmt.Errorf("%s returned status %d: %s", c.Name, resp.StatusCode, httputil.Truncate(body, 200))
	}

	return nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.DoGet(ctx, "/system/status", nil)
	return err
}
