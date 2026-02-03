package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"streammon/internal/models"
)

type recentlyAddedContainer struct {
	XMLName xml.Name             `xml:"MediaContainer"`
	Items   []recentlyAddedItem  `xml:"Video"`
}

type recentlyAddedItem struct {
	Title            string `xml:"title,attr"`
	Year             string `xml:"year,attr"`
	Type             string `xml:"type,attr"`
	Thumb            string `xml:"thumb,attr"`
	AddedAt          string `xml:"addedAt,attr"`
	GrandparentTitle string `xml:"grandparentTitle,attr"`
	RatingKey        string `xml:"ratingKey,attr"`
}

func (s *Server) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	url := fmt.Sprintf("%s/library/recentlyAdded?X-Plex-Container-Start=0&X-Plex-Container-Size=%d", s.url, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plex recently added: %w", err)
	}
	defer drainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex recently added: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	var data recentlyAddedContainer
	if err := xml.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("plex parse recently added: %w", err)
	}

	items := make([]models.LibraryItem, 0, len(data.Items))
	for _, item := range data.Items {
		title := item.Title
		if item.GrandparentTitle != "" {
			title = item.GrandparentTitle + " - " + item.Title
		}

		thumbURL := item.Thumb
		itemID := item.RatingKey

		items = append(items, models.LibraryItem{
			ItemID:     itemID,
			Title:      title,
			Year:       atoi(item.Year),
			MediaType:  plexMediaType(item.Type),
			ThumbURL:   thumbURL,
			AddedAt:    time.Unix(atoi64(item.AddedAt), 0).UTC(),
			ServerID:   s.serverID,
			ServerName: s.serverName,
			ServerType: models.ServerTypePlex,
		})
	}

	return items, nil
}
