package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"streammon/internal/httputil"
	"streammon/internal/models"
)

type episodesContainer struct {
	XMLName xml.Name          `xml:"MediaContainer"`
	Videos  []episodeVideoXML `xml:"Video"`
}

type episodeVideoXML struct {
	RatingKey             string `xml:"ratingKey,attr"`
	Title                 string `xml:"title,attr"`
	Summary               string `xml:"summary,attr"`
	Thumb                 string `xml:"thumb,attr"`
	Duration              int64  `xml:"duration,attr"`
	Index                 int    `xml:"index,attr"`
	OriginallyAvailableAt string `xml:"originallyAvailableAt,attr"`
}

func (s *Server) GetEpisodes(ctx context.Context, seasonID string) ([]models.Episode, error) {
	reqURL := fmt.Sprintf("%s/library/metadata/%s/children", s.url, url.PathEscape(seasonID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plex episodes: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex episodes: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, err
	}

	var container episodesContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("parse episodes: %w", err)
	}

	episodes := make([]models.Episode, 0, len(container.Videos))
	for _, v := range container.Videos {
		episodes = append(episodes, models.Episode{
			ID:         v.RatingKey,
			Number:     v.Index,
			Title:      v.Title,
			ThumbURL:   v.RatingKey,
			Summary:    v.Summary,
			DurationMs: v.Duration,
			AirDate:    v.OriginallyAvailableAt,
		})
	}
	return episodes, nil
}
