package arrutil

import (
	"bytes"
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

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader) (json.RawMessage, error) {
	u := c.BaseURL + "/api/v3" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer httputil.DrainBody(resp)

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, httputil.MaxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned status %d: %s", c.Name, resp.StatusCode, httputil.Truncate(respBody, 200))
	}

	return json.RawMessage(respBody), nil
}

func (c *Client) DoGet(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) DoPut(ctx context.Context, path string, body json.RawMessage) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPut, path, nil, bytes.NewReader(body))
}

func (c *Client) DoDelete(ctx context.Context, path string, query url.Values) error {
	_, err := c.do(ctx, http.MethodDelete, path, query, nil)
	return err
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.DoGet(ctx, "/system/status", nil)
	return err
}
