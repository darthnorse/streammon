package plex

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"streammon/internal/httputil"
	"streammon/internal/models"
)

func (s *Server) TerminateSession(ctx context.Context, sessionID string, message string) error {
	params := url.Values{
		"sessionId": {sessionID},
	}
	if message != "" {
		params.Set("reason", message)
	}

	// Plex's terminate endpoint requires GET despite being a state-changing action.
	reqURL := fmt.Sprintf("%s/status/sessions/terminate?%s", s.url, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("plex terminate: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("plex terminate: unauthorized — %w", models.ErrPlexPassRequired)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plex terminate: status %d", resp.StatusCode)
	}
	return nil
}
