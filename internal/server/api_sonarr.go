package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/sonarr"
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

func (s *Server) newSonarrClient() (*sonarr.Client, error) {
	cfg, err := s.store.GetSonarrConfig()
	if err != nil {
		return nil, errors.New("sonarr not available")
	}
	if cfg.URL == "" || cfg.APIKey == "" || !cfg.Enabled {
		return nil, errors.New("sonarr not configured")
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
	if err != nil || cfg.URL == "" || cfg.APIKey == "" || !cfg.Enabled {
		writeError(w, http.StatusServiceUnavailable, "sonarr not configured")
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
