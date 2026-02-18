package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

var (
	// Emby/Jellyfin user IDs: 32-char hex GUIDs, with or without dashes
	validUserIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{4}-?[a-fA-F0-9]{12}$`)
	// Emby/Jellyfin item IDs: 32-char hex or alphanumeric (some versions use different formats)
	validItemIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]{1,64}$`)
	// Plex rating keys: numeric IDs
	validPlexIDPattern = regexp.MustCompile(`^[0-9]+$`)
	// Plex thumbnail paths: library/{segments}/{id}/thumb with optional cache-buster
	validPlexThumbPath = regexp.MustCompile(`^library/(?:[a-z]+/)*[0-9]+/thumb(?:/[0-9]+)?$`)
)

func escapePathSegments(path string) string {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}

func (s *Server) handleThumbProxy(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	thumbPath := strings.TrimLeft(chi.URLParam(r, "*"), "/")
	if thumbPath == "" {
		writeError(w, http.StatusBadRequest, "missing thumb path")
		return
	}

	if !isValidPathSegment(thumbPath) {
		writeError(w, http.StatusBadRequest, "invalid thumb path")
		return
	}

	srv, err := s.store.GetServer(serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	baseURL := strings.TrimRight(srv.URL, "/")
	var imgURL string

	switch srv.Type {
	case models.ServerTypePlex:
		if strings.Contains(thumbPath, "/") {
			if !validPlexThumbPath.MatchString(thumbPath) {
				writeError(w, http.StatusBadRequest, "invalid plex thumb path")
				return
			}
			imgURL = fmt.Sprintf("%s/%s?X-Plex-Token=%s", baseURL, escapePathSegments(thumbPath), srv.APIKey)
		} else {
			if !validPlexIDPattern.MatchString(thumbPath) {
				writeError(w, http.StatusBadRequest, "invalid plex id format")
				return
			}
			imgURL = fmt.Sprintf("%s/library/metadata/%s/thumb?X-Plex-Token=%s", baseURL, thumbPath, srv.APIKey)
		}
	case models.ServerTypeEmby, models.ServerTypeJellyfin:
		if strings.HasPrefix(thumbPath, "user/") {
			userID := strings.TrimPrefix(thumbPath, "user/")
			if !validUserIDPattern.MatchString(userID) {
				writeError(w, http.StatusBadRequest, "invalid user id format")
				return
			}
			imgURL = fmt.Sprintf("%s/Users/%s/Images/Primary?maxHeight=300", baseURL, userID)
		} else {
			if !validItemIDPattern.MatchString(thumbPath) {
				writeError(w, http.StatusBadRequest, "invalid item id format")
				return
			}
			imgURL = fmt.Sprintf("%s/Items/%s/Images/Primary?maxHeight=300", baseURL, url.PathEscape(thumbPath))
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported server type")
		return
	}

	ctx := r.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "bad request")
		return
	}

	if srv.Type == models.ServerTypeEmby || srv.Type == models.ServerTypeJellyfin {
		req.Header.Set("X-Emby-Token", srv.APIKey)
	}

	resp, err := s.thumbProxyHTTP.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		writeError(w, http.StatusBadGateway, "upstream error")
		return
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		ct = "image/jpeg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 5<<20))
}
