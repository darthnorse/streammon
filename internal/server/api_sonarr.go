package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
	"streammon/internal/sonarr"
	"streammon/internal/store"
)

func (s *Server) sonarrDeps() integrationDeps {
	return integrationDeps{
		validateURL:  sonarr.ValidateURL,
		newClient:    func(url, apiKey string) (integrationClient, error) { return sonarr.NewClient(url, apiKey) },
		getConfig:    s.store.GetSonarrConfig,
		setConfig:    s.store.SetSonarrConfig,
		deleteConfig: s.store.DeleteSonarrConfig,
	}
}

func (s *Server) validSonarrConfig() (store.IntegrationConfig, error) {
	cfg, err := s.store.GetSonarrConfig()
	if err != nil {
		return cfg, errors.New("sonarr not available")
	}
	if cfg.URL == "" || cfg.APIKey == "" || !cfg.Enabled {
		return cfg, errors.New("sonarr not configured")
	}
	return cfg, nil
}

func (s *Server) newSonarrClient() (*sonarr.Client, error) {
	cfg, err := s.validSonarrConfig()
	if err != nil {
		return nil, err
	}
	return sonarr.NewClient(cfg.URL, cfg.APIKey)
}

func (s *Server) sonarrClientWithTimeout(w http.ResponseWriter, r *http.Request) (*sonarr.Client, context.Context, context.CancelFunc, bool) {
	client, err := s.newSonarrClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return nil, nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), integrationTimeout)
	return client, ctx, cancel, true
}

func validDateParam(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func (s *Server) calendarAccessBlocked(w http.ResponseWriter, r *http.Request) bool {
	if user := UserFromContext(r.Context()); user != nil && user.Role != models.RoleAdmin {
		allowed, err := s.store.GetGuestSetting("show_calendar")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return true
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "calendar access disabled")
			return true
		}
	}
	return false
}

func (s *Server) handleSonarrSeries(w http.ResponseWriter, r *http.Request) {
	if s.calendarAccessBlocked(w, r) {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid series ID")
		return
	}
	client, ctx, cancel, ok := s.sonarrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()
	data, err := client.GetSeries(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}
	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleSonarrCalendar(w http.ResponseWriter, r *http.Request) {
	if s.calendarAccessBlocked(w, r) {
		return
	}

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

func (s *Server) handleSonarrSeriesStatuses(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TVDBIDs []string `json:"tvdb_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.TVDBIDs) > 500 {
		writeError(w, http.StatusBadRequest, "too many IDs (max 500)")
		return
	}
	if len(req.TVDBIDs) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{})
		return
	}

	for _, id := range req.TVDBIDs {
		if _, err := strconv.Atoi(id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid tvdb_id: "+id)
			return
		}
	}

	client, ctx, cancel, ok := s.sonarrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	series, err := client.ListSeriesStatuses(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	tvdbMap := make(map[string]string, len(series))
	for _, sr := range series {
		tvdbMap[strconv.Itoa(sr.TVDBID)] = sr.Status
	}

	result := make(map[string]string, len(req.TVDBIDs))
	for _, id := range req.TVDBIDs {
		if status, ok := tvdbMap[id]; ok {
			result[id] = status
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSonarrPoster(w http.ResponseWriter, r *http.Request) {
	if s.calendarAccessBlocked(w, r) {
		return
	}
	seriesID := chi.URLParam(r, "seriesId")
	if _, err := strconv.Atoi(seriesID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid series ID")
		return
	}

	cfg, err := s.validSonarrConfig()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	imgURL := fmt.Sprintf("%s/api/v3/mediacover/%s/poster-250.jpg",
		strings.TrimRight(cfg.URL, "/"), seriesID)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imgURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
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
