package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"streammon/internal/httputil"
	"streammon/internal/models"
)

type recentlyAddedContainer struct {
	XMLName xml.Name            `xml:"MediaContainer"`
	Videos  []recentlyAddedItem `xml:"Video"`
}

type recentlyAddedItem struct {
	Title            string     `xml:"title,attr"`
	Year             string     `xml:"year,attr"`
	Type             string     `xml:"type,attr"`
	Thumb            string     `xml:"thumb,attr"`
	GrandparentThumb string     `xml:"grandparentThumb,attr"`
	AddedAt          string     `xml:"addedAt,attr"`
	GrandparentTitle string     `xml:"grandparentTitle,attr"`
	RatingKey        string     `xml:"ratingKey,attr"`
	GrandparentKey   string     `xml:"grandparentRatingKey,attr"`
	ParentIndex      string     `xml:"parentIndex,attr"`
	Index            string     `xml:"index,attr"`
	Guids            []plexGuid `xml:"Guid"`
}

type plexGuid struct {
	ID string `xml:"id,attr"`
}

// Plex hub media types
const (
	plexHubTypeMovie = "1"
	plexHubTypeShow  = "2"
)

func (s *Server) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	var allItems []models.LibraryItem

	for _, mediaType := range []string{plexHubTypeMovie, plexHubTypeShow} {
		items, err := s.fetchHubRecentlyAdded(ctx, mediaType, limit)
		if err != nil {
			log.Printf("plex recently added type=%s: %v", mediaType, err)
			continue
		}
		allItems = append(allItems, items...)
	}

	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].AddedAt.After(allItems[j].AddedAt)
	})

	if len(allItems) > limit {
		allItems = allItems[:limit]
	}

	return allItems, nil
}

func (s *Server) fetchHubRecentlyAdded(ctx context.Context, mediaType string, limit int) ([]models.LibraryItem, error) {
	url := fmt.Sprintf("%s/hubs/home/recentlyAdded?X-Plex-Container-Start=0&X-Plex-Container-Size=%d&type=%s",
		s.url, limit, mediaType)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plex hub recently added: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex hub recently added: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	var data recentlyAddedContainer
	if err := xml.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("plex parse hub recently added: %w", err)
	}

	items := make([]models.LibraryItem, 0, len(data.Videos))
	for _, item := range data.Videos {
		title := item.Title
		if item.GrandparentTitle != "" {
			title = item.GrandparentTitle + " - " + item.Title
		}

		var thumbURL string
		if item.GrandparentThumb != "" && item.GrandparentKey != "" {
			thumbURL = item.GrandparentKey
		} else if item.Thumb != "" {
			thumbURL = item.RatingKey
		}

		items = append(items, models.LibraryItem{
			ItemID:        item.RatingKey,
			Title:         title,
			SeriesTitle:   item.GrandparentTitle,
			Year:          atoi(item.Year),
			MediaType:     plexMediaType(item.Type),
			ThumbURL:      thumbURL,
			AddedAt:       time.Unix(atoi64(item.AddedAt), 0).UTC(),
			ServerID:      s.serverID,
			ServerName:    s.serverName,
			ServerType:    models.ServerTypePlex,
			SeasonNumber:  atoi(item.ParentIndex),
			EpisodeNumber: atoi(item.Index),
			ExternalIDs:   parsePlexGuids(item.Guids),
		})
	}

	return items, nil
}

func parsePlexGuids(guids []plexGuid) models.ExternalIDs {
	var ids models.ExternalIDs
	for _, g := range guids {
		switch {
		case strings.HasPrefix(g.ID, "imdb://"):
			ids.IMDB = strings.TrimPrefix(g.ID, "imdb://")
		case strings.HasPrefix(g.ID, "tmdb://"):
			ids.TMDB = strings.TrimPrefix(g.ID, "tmdb://")
		case strings.HasPrefix(g.ID, "tvdb://"):
			ids.TVDB = strings.TrimPrefix(g.ID, "tvdb://")
		}
	}
	return ids
}
