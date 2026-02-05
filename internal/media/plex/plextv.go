package plex

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"streammon/internal/httputil"
	"streammon/internal/models"
)

const plexTVBaseURL = "https://plex.tv"

var plexTVClient = httputil.NewClientWithTimeout(httputil.ExtendedTimeout)

type plexTVAccountResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Title    string `json:"title"`
	Email    string `json:"email"`
	Thumb    string `json:"thumb"`
}

type plexTVUsersXMLResponse struct {
	XMLName xml.Name        `xml:"MediaContainer"`
	Users   []plexTVUserXML `xml:"User"`
}

type plexTVUserXML struct {
	ID       string `xml:"id,attr"`
	Username string `xml:"username,attr"`
	Title    string `xml:"title,attr"`
	Thumb    string `xml:"thumb,attr"`
}

func (s *Server) GetUsers(ctx context.Context) ([]models.MediaUser, error) {
	owner, err := s.getPlexTVOwner(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching owner: %w", err)
	}

	friends, err := s.getPlexTVFriends(ctx)
	if err != nil {
		slog.Warn("failed to fetch plex friends", "error", err)
		return []models.MediaUser{*owner}, nil
	}

	users := make([]models.MediaUser, 0, 1+len(friends))
	users = append(users, *owner)
	users = append(users, friends...)

	return users, nil
}

func (s *Server) getPlexTVOwner(ctx context.Context) (*models.MediaUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, plexTVBaseURL+"/api/v2/user", nil)
	if err != nil {
		return nil, err
	}
	s.setPlexTVHeaders(req)

	resp, err := plexTVClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex.tv returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var account plexTVAccountResponse
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("parsing account: %w", err)
	}

	return &models.MediaUser{
		Name:     plexUsername(account.Username, account.Title),
		ThumbURL: account.Thumb,
	}, nil
}

func (s *Server) getPlexTVFriends(ctx context.Context) ([]models.MediaUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, plexTVBaseURL+"/api/users", nil)
	if err != nil {
		return nil, err
	}
	s.setPlexTVHeaders(req)
	req.Header.Set("Accept", "application/xml")

	resp, err := plexTVClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex.tv returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	var usersResp plexTVUsersXMLResponse
	if err := xml.Unmarshal(body, &usersResp); err != nil {
		return nil, fmt.Errorf("parsing users XML: %w", err)
	}

	users := make([]models.MediaUser, 0, len(usersResp.Users))
	for _, u := range usersResp.Users {
		users = append(users, models.MediaUser{
			Name:     plexUsername(u.Username, u.Title),
			ThumbURL: u.Thumb,
		})
	}

	return users, nil
}

func (s *Server) setPlexTVHeaders(req *http.Request) {
	req.Header.Set("X-Plex-Token", s.token)
	req.Header.Set("X-Plex-Client-Identifier", "streammon")
	req.Header.Set("X-Plex-Product", "StreamMon")
	req.Header.Set("X-Plex-Version", "1.0.0")
	req.Header.Set("Accept", "application/json")
}

// plexUsername falls back to title if username is empty (some Plex accounts only have title)
func plexUsername(username, title string) string {
	if username != "" {
		return username
	}
	return title
}
