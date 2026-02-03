package server

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

var thumbProxyClient = &http.Client{Timeout: 10 * time.Second}

func (s *Server) handleThumbProxy(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}

	thumbPath := chi.URLParam(r, "*")
	if thumbPath == "" {
		writeError(w, http.StatusBadRequest, "missing thumb path")
		return
	}

	if strings.Contains(thumbPath, "..") || strings.Contains(thumbPath, "?") || strings.Contains(thumbPath, "#") {
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
		imgURL = fmt.Sprintf("%s/%s?X-Plex-Token=%s", baseURL, thumbPath, srv.APIKey)
	case models.ServerTypeEmby, models.ServerTypeJellyfin:
		imgURL = fmt.Sprintf("%s/Items/%s/Images/Primary?maxHeight=300", baseURL, thumbPath)
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

	resp, err := thumbProxyClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		writeError(w, resp.StatusCode, "upstream error")
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
