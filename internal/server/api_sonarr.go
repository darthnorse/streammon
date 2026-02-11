package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/sonarr"
	"streammon/internal/store"
)

type sonarrSettings struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

type sonarrTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleGetSonarrSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetSonarrConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	apiKey := ""
	if cfg.APIKey != "" {
		apiKey = maskedSecret
	}

	writeJSON(w, http.StatusOK, sonarrSettings{
		URL:    cfg.URL,
		APIKey: apiKey,
	})
}

func (s *Server) handleUpdateSonarrSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req sonarrSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.APIKey == maskedSecret {
		req.APIKey = ""
	}

	if req.URL != "" {
		if err := sonarr.ValidateURL(req.URL); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if req.APIKey == "" {
		existing, err := s.store.GetSonarrConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		if req.URL != existing.URL {
			writeError(w, http.StatusBadRequest, "api_key is required when changing the URL")
			return
		}
	}

	storeCfg := store.SonarrConfig{
		URL:    req.URL,
		APIKey: req.APIKey,
	}

	if err := s.store.SetSonarrConfig(storeCfg); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteSonarrSettings(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteSonarrConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleTestSonarrConnection(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req sonarrSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	apiKey := req.APIKey
	if apiKey == "" || apiKey == maskedSecret {
		cfg, err := s.store.GetSonarrConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		apiKey = cfg.APIKey
	}

	if apiKey == "" {
		writeError(w, http.StatusBadRequest, "api_key is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	client, err := sonarr.NewClient(req.URL, apiKey)
	if err != nil {
		writeJSON(w, http.StatusOK, sonarrTestResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if err := client.TestConnection(ctx); err != nil {
		writeJSON(w, http.StatusOK, sonarrTestResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, sonarrTestResponse{Success: true})
}

func (s *Server) handleSonarrConfigured(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetSonarrConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"configured": cfg.URL != "" && cfg.APIKey != "",
	})
}

func (s *Server) newSonarrClient() (*sonarr.Client, error) {
	cfg, err := s.store.GetSonarrConfig()
	if err != nil {
		return nil, fmt.Errorf("sonarr not available")
	}
	if cfg.URL == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("sonarr not configured")
	}
	return sonarr.NewClient(cfg.URL, cfg.APIKey)
}

func (s *Server) sonarrClientWithTimeout(w http.ResponseWriter, r *http.Request) (*sonarr.Client, context.Context, context.CancelFunc, bool) {
	client, err := s.newSonarrClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return nil, nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	return client, ctx, cancel, true
}

func validDateParam(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func (s *Server) handleSonarrCalendar(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	if !validDateParam(start) || !validDateParam(end) {
		writeError(w, http.StatusBadRequest, "start and end must be YYYY-MM-DD format")
		return
	}

	client, ctx, cancel, ok := s.sonarrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.GetCalendar(ctx, start, end)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleSonarrPoster(w http.ResponseWriter, r *http.Request) {
	seriesID := chi.URLParam(r, "seriesId")
	if _, err := strconv.Atoi(seriesID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid series ID")
		return
	}

	cfg, err := s.store.GetSonarrConfig()
	if err != nil || cfg.URL == "" || cfg.APIKey == "" {
		writeError(w, http.StatusServiceUnavailable, "sonarr not configured")
		return
	}

	imgURL := fmt.Sprintf("%s/api/v3/mediacover/%s/poster-250.jpg",
		strings.TrimRight(cfg.URL, "/"), seriesID)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imgURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "bad request")
		return
	}
	req.Header.Set("X-Api-Key", cfg.APIKey)

	resp, err := s.sonarrPosterHTTP.Do(req)
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
	w.Header().Set("Cache-Control", "public, max-age=14400")
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 5<<20))
}
