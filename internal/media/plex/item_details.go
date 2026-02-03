package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"streammon/internal/models"
)

type itemDetailsContainer struct {
	XMLName xml.Name         `xml:"MediaContainer"`
	Videos  []itemDetailItem `xml:"Video"`
}

type itemDetailItem struct {
	RatingKey        string             `xml:"ratingKey,attr"`
	Title            string             `xml:"title,attr"`
	Year             string             `xml:"year,attr"`
	Summary          string             `xml:"summary,attr"`
	Type             string             `xml:"type,attr"`
	Thumb            string             `xml:"thumb,attr"`
	Rating           string             `xml:"rating,attr"`
	ContentRating    string             `xml:"contentRating,attr"`
	Duration         string             `xml:"duration,attr"`
	Studio           string             `xml:"studio,attr"`
	GrandparentTitle string             `xml:"grandparentTitle,attr"`
	ParentIndex      string             `xml:"parentIndex,attr"`
	Index            string             `xml:"index,attr"`
	Genres           []genreItem        `xml:"Genre"`
	Directors        []directorItem     `xml:"Director"`
	Roles            []roleItem         `xml:"Role"`
}

type genreItem struct {
	Tag string `xml:"tag,attr"`
}

type directorItem struct {
	Tag string `xml:"tag,attr"`
}

type roleItem struct {
	Tag   string `xml:"tag,attr"`
	Role  string `xml:"role,attr"`
	Thumb string `xml:"thumb,attr"`
}

func (s *Server) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	url := fmt.Sprintf("%s/library/metadata/%s", s.url, itemID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plex item details: %w", err)
	}
	defer drainBody(resp)

	if resp.StatusCode == http.StatusNotFound {
		return nil, models.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex item details: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}

	return parseItemDetails(body, s.serverID, s.serverName)
}

func parseItemDetails(data []byte, serverID int64, serverName string) (*models.ItemDetails, error) {
	var container itemDetailsContainer
	if err := xml.Unmarshal(data, &container); err != nil {
		return nil, fmt.Errorf("plex parse item details: %w", err)
	}

	if len(container.Videos) == 0 {
		return nil, models.ErrNotFound
	}

	item := container.Videos[0]

	genres := make([]string, 0, len(item.Genres))
	for _, g := range item.Genres {
		genres = append(genres, g.Tag)
	}

	directors := make([]string, 0, len(item.Directors))
	for _, d := range item.Directors {
		directors = append(directors, d.Tag)
	}

	cast := make([]models.CastMember, 0, len(item.Roles))
	for _, r := range item.Roles {
		thumbURL := r.Thumb
		if thumbURL != "" && !strings.HasPrefix(thumbURL, "http") {
			thumbURL = fmt.Sprintf("/api/servers/%d/thumb%s", serverID, thumbURL)
		}
		cast = append(cast, models.CastMember{
			Name:     r.Tag,
			Role:     r.Role,
			ThumbURL: thumbURL,
		})
	}

	thumbURL := item.Thumb
	if thumbURL != "" {
		thumbURL = fmt.Sprintf("/api/servers/%d/thumb%s", serverID, thumbURL)
	}

	details := &models.ItemDetails{
		ID:            item.RatingKey,
		Title:         item.Title,
		Year:          atoi(item.Year),
		Summary:       item.Summary,
		MediaType:     plexMediaType(item.Type),
		ThumbURL:      thumbURL,
		Genres:        genres,
		Directors:     directors,
		Cast:          cast,
		Rating:        atof(item.Rating),
		ContentRating: item.ContentRating,
		DurationMs:    atoi64(item.Duration),
		Studio:        item.Studio,
		SeriesTitle:   item.GrandparentTitle,
		SeasonNumber:  atoi(item.ParentIndex),
		EpisodeNumber: atoi(item.Index),
		ServerID:      serverID,
		ServerName:    serverName,
		ServerType:    models.ServerTypePlex,
	}

	return details, nil
}
