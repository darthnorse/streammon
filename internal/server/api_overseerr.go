package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/overseerr"
	"streammon/internal/store"
)

type overseerrSettings struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

type overseerrTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type overseerrCreateRequestBody struct {
	MediaType string          `json:"mediaType"`
	MediaID   int             `json:"mediaId"`
	Seasons   json.RawMessage `json:"seasons,omitempty"`
	Is4K      bool            `json:"is4k,omitempty"`
}

var allowedRequestFilters = map[string]bool{
	"all": true, "pending": true, "approved": true,
	"processing": true, "available": true, "declined": true,
}

var allowedRequestSorts = map[string]bool{
	"added": true, "modified": true,
}

const maxRequestTake = 100
const defaultRequestTake = 20

func (s *Server) handleGetOverseerrSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetOverseerrConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	apiKey := ""
	if cfg.APIKey != "" {
		apiKey = maskedSecret
	}

	writeJSON(w, http.StatusOK, overseerrSettings{
		URL:    cfg.URL,
		APIKey: apiKey,
	})
}

func (s *Server) handleUpdateOverseerrSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req overseerrSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.APIKey == maskedSecret {
		req.APIKey = ""
	}

	if req.URL != "" {
		if err := overseerr.ValidateURL(req.URL); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	storeCfg := store.OverseerrConfig{
		URL:    req.URL,
		APIKey: req.APIKey,
	}

	if err := s.store.SetOverseerrConfig(storeCfg); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteOverseerrSettings(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteOverseerrConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleTestOverseerrConnection(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req overseerrSettings
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
		cfg, err := s.store.GetOverseerrConfig()
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

	client, err := overseerr.NewClient(req.URL, apiKey)
	if err != nil {
		writeJSON(w, http.StatusOK, overseerrTestResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if err := client.TestConnection(ctx); err != nil {
		writeJSON(w, http.StatusOK, overseerrTestResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, overseerrTestResponse{Success: true})
}

func (s *Server) newOverseerrClient() (*overseerr.Client, error) {
	cfg, err := s.store.GetOverseerrConfig()
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if cfg.URL == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("Overseerr not configured")
	}
	return overseerr.NewClient(cfg.URL, cfg.APIKey)
}

func (s *Server) overseerrClientWithTimeout(w http.ResponseWriter, r *http.Request) (*overseerr.Client, context.Context, context.CancelFunc, bool) {
	client, err := s.newOverseerrClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return nil, nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	return client, ctx, cancel, true
}

func (s *Server) handleOverseerrSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter is required")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.Search(ctx, query, page)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrDiscoverTrending(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.DiscoverTrending(ctx, page)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrMovie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.GetMovie(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrTV(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid TV ID")
		return
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.GetTV(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrTVSeason(w http.ResponseWriter, r *http.Request) {
	tvID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid TV ID")
		return
	}
	seasonNum, err := strconv.Atoi(chi.URLParam(r, "seasonNumber"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid season number")
		return
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.GetTVSeason(ctx, tvID, seasonNum)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrListRequests(w http.ResponseWriter, r *http.Request) {
	take, _ := strconv.Atoi(r.URL.Query().Get("take"))
	skip, _ := strconv.Atoi(r.URL.Query().Get("skip"))
	filter := r.URL.Query().Get("filter")
	sort := r.URL.Query().Get("sort")

	if take <= 0 {
		take = defaultRequestTake
	} else if take > maxRequestTake {
		take = maxRequestTake
	}
	if filter != "" && !allowedRequestFilters[filter] {
		writeError(w, http.StatusBadRequest, "invalid filter value")
		return
	}
	if sort != "" && !allowedRequestSorts[sort] {
		writeError(w, http.StatusBadRequest, "invalid sort value")
		return
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.ListRequests(ctx, take, skip, filter, sort)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrRequestCount(w http.ResponseWriter, r *http.Request) {
	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.RequestCount(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrCreateRequest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var req overseerrCreateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.MediaType != "movie" && req.MediaType != "tv" {
		writeError(w, http.StatusBadRequest, "mediaType must be 'movie' or 'tv'")
		return
	}
	if req.MediaID <= 0 {
		writeError(w, http.StatusBadRequest, "mediaId is required")
		return
	}

	sanitized, err := json.Marshal(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.CreateRequest(ctx, sanitized)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusCreated, data)
}

func (s *Server) handleOverseerrRequestAction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	action := chi.URLParam(r, "action")
	if action != "approve" && action != "decline" {
		writeError(w, http.StatusBadRequest, "action must be 'approve' or 'decline'")
		return
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.UpdateRequestStatus(ctx, id, action)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrDeleteRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	if err := client.DeleteRequest(ctx, id); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
