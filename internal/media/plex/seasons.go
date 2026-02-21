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

type seasonsContainer struct {
	XMLName     xml.Name        `xml:"MediaContainer"`
	Directories []seasonDirXML  `xml:"Directory"`
}

type seasonDirXML struct {
	RatingKey string `xml:"ratingKey,attr"`
	Index     int    `xml:"index,attr"`
	Title     string `xml:"title,attr"`
}

func (s *Server) GetSeasons(ctx context.Context, showID string) ([]models.Season, error) {
	reqURL := fmt.Sprintf("%s/library/metadata/%s/children", s.url, url.PathEscape(showID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plex seasons: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex seasons: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, err
	}

	var container seasonsContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("parse seasons: %w", err)
	}

	seasons := make([]models.Season, 0, len(container.Directories))
	for _, dir := range container.Directories {
		seasons = append(seasons, models.Season{
			ID:     dir.RatingKey,
			Number: dir.Index,
			Title:  dir.Title,
		})
	}
	return seasons, nil
}
