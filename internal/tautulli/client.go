package tautulli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// FlexString handles JSON fields that can be either string or number
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*f = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = FlexString(n.String())
	return nil
}

// FlexInt handles JSON fields that can be either string or number
type FlexInt int

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" || string(data) == `""` {
		*f = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		var n int64
		if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
			*f = 0
			return nil
		}
		*f = FlexInt(n)
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = FlexInt(n)
	return nil
}

type HistoryRecord struct {
	User                 string     `json:"user"`
	Title                string     `json:"title"`
	MediaType            string     `json:"media_type"`
	GrandparentTitle     string     `json:"grandparent_title"`
	ParentTitle          string     `json:"parent_title"`
	Year                 FlexInt    `json:"year"`
	RatingKey            FlexString `json:"rating_key"`
	GrandparentRatingKey FlexString `json:"grandparent_rating_key"`
	Started              int64      `json:"started"`
	Stopped              int64      `json:"stopped"`
	Duration             int64      `json:"duration"`
	PlayDuration         int64      `json:"play_duration"`
	Player               string     `json:"player"`
	Platform             string     `json:"platform"`
	IPAddress            string     `json:"ip_address"`
	Thumb                string     `json:"thumb"`
	ParentMediaIndex     FlexInt    `json:"parent_media_index"`
	MediaIndex           FlexInt    `json:"media_index"`
}

type historyResponse struct {
	Response struct {
		Result  string `json:"result"`
		Message string `json:"message"`
		Data    struct {
			RecordsFiltered int             `json:"recordsFiltered"`
			RecordsTotal    int             `json:"recordsTotal"`
			Data            []HistoryRecord `json:"data"`
		} `json:"data"`
	} `json:"response"`
}

type serverInfoResponse struct {
	Response struct {
		Result  string `json:"result"`
		Message string `json:"message"`
	} `json:"response"`
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

func (c *Client) doRequest(ctx context.Context, params url.Values, maxBodySize int64) ([]byte, error) {
	u, err := url.Parse(c.baseURL + "/api/v2")
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	params.Set("apikey", c.apiKey)
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer drainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tautulli returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	params := url.Values{}
	params.Set("cmd", "get_server_info")

	body, err := c.doRequest(ctx, params, 1<<20)
	if err != nil {
		return err
	}

	var r serverInfoResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if r.Response.Result != "success" {
		return fmt.Errorf("Tautulli error: %s", r.Response.Message)
	}

	return nil
}

func (c *Client) GetHistory(ctx context.Context, start, length int) ([]HistoryRecord, int, error) {
	params := url.Values{}
	params.Set("cmd", "get_history")
	params.Set("start", fmt.Sprintf("%d", start))
	params.Set("length", fmt.Sprintf("%d", length))

	body, err := c.doRequest(ctx, params, 50<<20)
	if err != nil {
		return nil, 0, err
	}

	var r historyResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, 0, fmt.Errorf("parsing response: %w", err)
	}

	if r.Response.Result != "success" {
		return nil, 0, fmt.Errorf("Tautulli error: %s", r.Response.Message)
	}

	return r.Response.Data.Data, r.Response.Data.RecordsTotal, nil
}

type BatchResult struct {
	Records   []HistoryRecord
	Total     int
	Processed int
}

type BatchHandler func(batch BatchResult) error

func (c *Client) StreamHistory(ctx context.Context, batchSize int, handler BatchHandler) error {
	if batchSize <= 0 {
		batchSize = 1000
	}

	start := 0
	processed := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		records, total, err := c.GetHistory(ctx, start, batchSize)
		if err != nil {
			return err
		}

		processed += len(records)

		if err := handler(BatchResult{
			Records:   records,
			Total:     total,
			Processed: processed,
		}); err != nil {
			return err
		}

		if len(records) == 0 || len(records) < batchSize || processed >= total {
			break
		}

		start += len(records)
	}

	return nil
}

func drainBody(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
}
