package embybase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"streammon/internal/httputil"
)

func (c *Client) TerminateSession(ctx context.Context, sessionID string, message string) error {
	// Best-effort: send message first, then stop playback
	if message != "" {
		if err := c.sendSessionMessage(ctx, sessionID, message); err != nil {
			slog.Warn("failed to send session message, proceeding with stop",
				"server_type", c.serverType, "session_id", sessionID, "error", err)
		}
	}
	return c.stopSession(ctx, sessionID)
}

func (c *Client) sendSessionMessage(ctx context.Context, sessionID string, message string) error {
	msgURL := fmt.Sprintf("%s/Sessions/%s/Message", c.url, url.PathEscape(sessionID))
	payload := struct {
		Text      string `json:"Text"`
		TimeoutMs int    `json:"TimeoutMs"`
	}{Text: message, TimeoutMs: 5000}
	b, _ := json.Marshal(payload)
	body := string(b)
	return c.doPost(ctx, msgURL, body)
}

func (c *Client) stopSession(ctx context.Context, sessionID string) error {
	stopURL := fmt.Sprintf("%s/Sessions/%s/Playing/Stop", c.url, url.PathEscape(sessionID))
	return c.doPost(ctx, stopURL, "")
}

func (c *Client) doPost(ctx context.Context, fullURL string, body string) error {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bodyReader)
	if err != nil {
		return err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(c.addAuth(req))
	if err != nil {
		return fmt.Errorf("%s post: %w", c.serverType, err)
	}
	defer httputil.DrainBody(resp)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s post: status %d", c.serverType, resp.StatusCode)
	}
	return nil
}
