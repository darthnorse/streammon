package jellystat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"streammon/internal/httputil"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

type TranscodingInfo struct {
	VideoCodec    string `json:"VideoCodec"`
	AudioCodec    string `json:"AudioCodec"`
	AudioChannels int    `json:"AudioChannels"`
	Bitrate       int64  `json:"Bitrate"`
	Width         int    `json:"Width"`
	Height        int    `json:"Height"`
	IsVideoDirect bool   `json:"IsVideoDirect"`
	IsAudioDirect bool   `json:"IsAudioDirect"`
}

type PlayState struct {
	RuntimeTicks *int64 `json:"RuntimeTicks"`
}

type HistoryRecord struct {
	ID                   string           `json:"Id"`
	UserName             string           `json:"UserName"`
	NowPlayingItemName   string           `json:"NowPlayingItemName"`
	NowPlayingItemId     string           `json:"NowPlayingItemId"`
	SeriesName           *string          `json:"SeriesName"`
	SeasonNumber         *int             `json:"SeasonNumber"`
	EpisodeNumber        *int             `json:"EpisodeNumber"`
	PlaybackDuration     float64          `json:"PlaybackDuration"`
	ActivityDateInserted string           `json:"ActivityDateInserted"`
	Client               string           `json:"Client"`
	DeviceName           string           `json:"DeviceName"`
	RemoteEndPoint       string           `json:"RemoteEndPoint"`
	PlayMethod           string           `json:"PlayMethod"`
	TranscodingInfo      *TranscodingInfo `json:"TranscodingInfo"`
	PlayState            *PlayState       `json:"PlayState"`
}

type historyPage struct {
	CurrentPage int             `json:"current_page"`
	Pages       int             `json:"pages"`
	Size        int             `json:"size"`
	Results     []HistoryRecord `json:"results"`
}

type BatchResult struct {
	Records []HistoryRecord
	Total   int
}

type BatchHandler func(batch BatchResult) error

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
	return httputil.ValidateIntegrationURL(rawURL)
}

func (c *Client) doRequest(ctx context.Context, path string, maxBodySize int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("x-api-token", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellystat returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	body, err := c.doRequest(ctx, "/api/getconfig", 1<<20)
	if err != nil {
		return err
	}

	var result struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if result.Error != "" {
		return fmt.Errorf("jellystat error: %s", result.Error)
	}

	return nil
}

func (c *Client) GetHistory(ctx context.Context, page, size int) ([]HistoryRecord, int, error) {
	path := fmt.Sprintf("/api/getHistory?page=%d&size=%d&sort=ActivityDateInserted&desc=false", page, size)

	body, err := c.doRequest(ctx, path, 50<<20)
	if err != nil {
		return nil, 0, err
	}

	var p historyPage
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, 0, fmt.Errorf("parsing response: %w", err)
	}

	return p.Results, p.Pages, nil
}

func (c *Client) StreamHistory(ctx context.Context, pageSize int, handler BatchHandler) error {
	if pageSize <= 0 {
		pageSize = 1000
	}

	page := 1
	var processed int

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		records, totalPages, err := c.GetHistory(ctx, page, pageSize)
		if err != nil {
			return err
		}

		processed += len(records)
		isLastPage := len(records) == 0 || page >= totalPages

		// Use actual count on the last page; estimate otherwise
		total := totalPages * pageSize
		if isLastPage {
			total = processed
		}

		if err := handler(BatchResult{
			Records: records,
			Total:   total,
		}); err != nil {
			return err
		}

		if isLastPage {
			break
		}
		page++
	}

	return nil
}
