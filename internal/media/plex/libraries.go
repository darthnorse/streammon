package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"streammon/internal/models"
)

type librarySections struct {
	XMLName    xml.Name         `xml:"MediaContainer"`
	Directories []librarySection `xml:"Directory"`
}

type librarySection struct {
	Key   string `xml:"key,attr"`
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr"`
}

type libraryCount struct {
	XMLName  xml.Name `xml:"MediaContainer"`
	Size     int      `xml:"size,attr"`
	TotalSize int     `xml:"totalSize,attr"`
}

func (s *Server) GetLibraries(ctx context.Context) ([]models.Library, error) {
	sections, err := s.getLibrarySections(ctx)
	if err != nil {
		return nil, err
	}

	libraries := make([]models.Library, 0, len(sections.Directories))
	for _, dir := range sections.Directories {
		lib := models.Library{
			ID:         dir.Key,
			ServerID:   s.serverID,
			ServerName: s.serverName,
			ServerType: models.ServerTypePlex,
			Name:       dir.Title,
			Type:       plexLibraryType(dir.Type),
		}

		counts, err := s.getLibraryCounts(ctx, dir.Key, dir.Type)
		if err != nil {
			return nil, fmt.Errorf("getting counts for library %s: %w", dir.Title, err)
		}
		lib.ItemCount = counts.items
		lib.ChildCount = counts.children
		lib.GrandchildCount = counts.grandchildren

		libraries = append(libraries, lib)
	}

	return libraries, nil
}

func (s *Server) getLibrarySections(ctx context.Context) (*librarySections, error) {
	req, err := s.newRequest(ctx, s.url+"/library/sections")
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plex library sections: %w", err)
	}
	defer drainBody(resp)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("plex library sections: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var sections librarySections
	if err := xml.Unmarshal(body, &sections); err != nil {
		return nil, fmt.Errorf("parsing library sections: %w", err)
	}

	return &sections, nil
}

type libraryCounts struct {
	items        int
	children     int
	grandchildren int
}

func (s *Server) getLibraryCounts(ctx context.Context, key, libType string) (*libraryCounts, error) {
	counts := &libraryCounts{}

	switch libType {
	case "movie":
		count, err := s.countLibraryItems(ctx, key, "1")
		if err != nil {
			return nil, err
		}
		counts.items = count

	case "show":
		showCount, err := s.countLibraryItems(ctx, key, "2")
		if err != nil {
			return nil, err
		}
		counts.items = showCount

		seasonCount, err := s.countLibraryItems(ctx, key, "3")
		if err != nil {
			return nil, err
		}
		counts.children = seasonCount

		episodeCount, err := s.countLibraryItems(ctx, key, "4")
		if err != nil {
			return nil, err
		}
		counts.grandchildren = episodeCount

	case "artist":
		artistCount, err := s.countLibraryItems(ctx, key, "8")
		if err != nil {
			return nil, err
		}
		counts.items = artistCount

		albumCount, err := s.countLibraryItems(ctx, key, "9")
		if err != nil {
			return nil, err
		}
		counts.children = albumCount

		trackCount, err := s.countLibraryItems(ctx, key, "10")
		if err != nil {
			return nil, err
		}
		counts.grandchildren = trackCount

	default:
		count, err := s.countLibraryItems(ctx, key, "")
		if err != nil {
			return nil, err
		}
		counts.items = count
	}

	return counts, nil
}

func (s *Server) countLibraryItems(ctx context.Context, sectionKey, typeFilter string) (int, error) {
	u, err := url.Parse(s.url + "/library/sections/" + sectionKey + "/all")
	if err != nil {
		return 0, err
	}

	q := u.Query()
	q.Set("X-Plex-Container-Size", "0")
	if typeFilter != "" {
		q.Set("type", typeFilter)
	}
	u.RawQuery = q.Encode()

	req, err := s.newRequest(ctx, u.String())
	if err != nil {
		return 0, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("plex count items: %w", err)
	}
	defer drainBody(resp)

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("plex count items: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}

	var lc libraryCount
	if err := xml.Unmarshal(body, &lc); err != nil {
		return 0, fmt.Errorf("parsing library count: %w", err)
	}

	if lc.TotalSize > 0 {
		return lc.TotalSize, nil
	}
	return lc.Size, nil
}

func (s *Server) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req)
	return req, nil
}

func plexLibraryType(t string) models.LibraryType {
	switch t {
	case "movie":
		return models.LibraryTypeMovie
	case "show":
		return models.LibraryTypeShow
	case "artist":
		return models.LibraryTypeMusic
	default:
		return models.LibraryTypeOther
	}
}
