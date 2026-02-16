package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
	"streammon/internal/overseerr"
	"streammon/internal/store"
)

type overseerrCreateRequestBody struct {
	MediaType string          `json:"mediaType"`
	MediaID   int             `json:"mediaId"`
	Seasons   json.RawMessage `json:"seasons,omitempty"`
	Is4K      bool            `json:"is4k,omitempty"`
}

type overseerrCreateRequestPayload struct {
	overseerrCreateRequestBody
	UserID *int `json:"userId,omitempty"`
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

var emptyRequestList = json.RawMessage(`{"pageInfo":{"pages":1,"page":1,"results":0},"results":[]}`)

type overseerrUserCache struct {
	mu        sync.RWMutex
	emailToID map[string]int // lowercase email → Overseerr user ID
	expiresAt time.Time
}

const overseerrUserCacheTTL = 15 * time.Minute

func (s *Server) overseerrDeps() integrationDeps {
	return integrationDeps{
		validateURL:  overseerr.ValidateURL,
		newClient:    func(url, apiKey string) (integrationClient, error) { return overseerr.NewClient(url, apiKey) },
		getConfig:    s.store.GetOverseerrConfig,
		setConfig:    s.store.SetOverseerrConfig,
		deleteConfig: s.store.DeleteOverseerrConfig,
		onUpdate:     s.invalidateOverseerrUserCache,
		onDelete:     s.invalidateOverseerrUserCache,
	}
}

func (s *Server) newOverseerrClient() (*overseerr.Client, error) {
	cfg, err := s.store.GetOverseerrConfig()
	if err != nil {
		return nil, errors.New("overseerr/seerr not available")
	}
	if cfg.URL == "" || cfg.APIKey == "" || !cfg.Enabled {
		return nil, errors.New("overseerr/seerr not configured")
	}
	return overseerr.NewClient(cfg.URL, cfg.APIKey)
}

func (s *Server) overseerrClientWithTimeout(w http.ResponseWriter, r *http.Request) (*overseerr.Client, context.Context, context.CancelFunc, bool) {
	client, err := s.newOverseerrClient()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return nil, nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), integrationTimeout)
	return client, ctx, cancel, true
}

func (s *Server) handleOverseerrSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter is required")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 0 {
		page = 0
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.Search(ctx, query, page)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

var allowedDiscoverCategories = map[string]bool{
	"trending":        true,
	"movies":          true,
	"movies/upcoming": true,
	"tv":              true,
	"tv/upcoming":     true,
}

func (s *Server) handleOverseerrDiscover(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "*")
	if !allowedDiscoverCategories[category] {
		writeError(w, http.StatusNotFound, "unknown discover category")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 0 {
		page = 0
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.Discover(ctx, category, page)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
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
		writeError(w, http.StatusBadGateway, "upstream service error")
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
		writeError(w, http.StatusBadGateway, "upstream service error")
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
		writeError(w, http.StatusBadGateway, "upstream service error")
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
	if skip < 0 {
		skip = 0
	}
	if filter != "" && !allowedRequestFilters[filter] {
		writeError(w, http.StatusBadRequest, "invalid filter value")
		return
	}
	if sort != "" && !allowedRequestSorts[sort] {
		writeError(w, http.StatusBadRequest, "invalid sort value")
		return
	}

	// For non-admin users, filter to only their own requests.
	var requestedBy int
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if user.Role != models.RoleAdmin {
		if user.Email == "" {
			writeRawJSON(w, http.StatusOK, emptyRequestList)
			return
		}
		id, ok := s.resolveOverseerrUserID(r.Context(), user.Email)
		if !ok {
			writeRawJSON(w, http.StatusOK, emptyRequestList)
			return
		}
		requestedBy = id
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.ListRequests(ctx, take, skip, requestedBy, filter, sort)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
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
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) resolveOverseerrUserID(ctx context.Context, email string) (int, bool) {
	email = strings.ToLower(email)

	s.overseerrUsers.mu.RLock()
	if time.Now().UTC().Before(s.overseerrUsers.expiresAt) {
		id, ok := s.overseerrUsers.emailToID[email]
		s.overseerrUsers.mu.RUnlock()
		return id, ok
	}
	s.overseerrUsers.mu.RUnlock()

	// Acquire write lock, re-check, and claim the refresh with a short expiry
	// so concurrent goroutines use stale data instead of all fetching simultaneously
	s.overseerrUsers.mu.Lock()
	if time.Now().UTC().Before(s.overseerrUsers.expiresAt) {
		id, ok := s.overseerrUsers.emailToID[email]
		s.overseerrUsers.mu.Unlock()
		return id, ok
	}
	s.overseerrUsers.expiresAt = time.Now().UTC().Add(30 * time.Second)
	s.overseerrUsers.mu.Unlock()

	client, err := s.newOverseerrClient()
	if err != nil {
		log.Printf("overseerr user resolve: %v", err)
		s.invalidateOverseerrUserCache()
		return 0, false
	}

	resolveCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	users, err := client.ListUsers(resolveCtx)
	if err != nil {
		log.Printf("overseerr list users: %v", err)
		s.invalidateOverseerrUserCache()
		return 0, false
	}

	emailToID := make(map[string]int, len(users))
	for _, u := range users {
		if u.Email != "" {
			emailToID[strings.ToLower(u.Email)] = u.ID
		}
	}

	s.overseerrUsers.mu.Lock()
	s.overseerrUsers.emailToID = emailToID
	s.overseerrUsers.expiresAt = time.Now().UTC().Add(overseerrUserCacheTTL)
	s.overseerrUsers.mu.Unlock()

	id, ok := emailToID[email]
	return id, ok
}

func (s *Server) isOverseerrURLSafeForTokens() bool {
	cfg, err := s.store.GetOverseerrConfig()
	if err != nil || cfg.URL == "" {
		return false
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	// Non-IP hostnames (e.g. Docker service names like "overseerr") are
	// considered safe — they resolve within the private network.
	return !strings.Contains(host, ".")
}

func (s *Server) invalidateOverseerrUserCache() {
	s.overseerrUsers.mu.Lock()
	s.overseerrUsers.emailToID = nil
	s.overseerrUsers.expiresAt = time.Time{}
	s.overseerrUsers.mu.Unlock()
}

func (s *Server) handleOverseerrCreateRequest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSettingsBody)
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

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	reqBody, err := json.Marshal(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	// Try to create the request as the actual user via their Plex token.
	// This ensures Overseerr applies the user's auto-approval settings
	// rather than auto-approving everything as admin.
	user := UserFromContext(r.Context())
	if user != nil {
		enabled, _ := s.store.GetStorePlexTokens()
		if enabled && s.isOverseerrURLSafeForTokens() {
			plexToken, tokenErr := s.store.GetProviderToken(user.ID, store.ProviderPlex)
			if tokenErr == nil && plexToken != "" {
				data, createErr := client.CreateRequestAsUser(ctx, plexToken, reqBody)
				if createErr != nil {
					log.Printf("overseerr: create request as user %d failed: %v", user.ID, createErr)
					writeError(w, http.StatusBadGateway, "upstream service error")
					return
				}
				writeRawJSON(w, http.StatusCreated, data)
				return
			}
		}
	}

	// Fallback: use admin API key with userId attribution.
	// Auto-approval settings may not be respected in this path.
	payload := overseerrCreateRequestPayload{
		overseerrCreateRequestBody: req,
	}
	if user != nil && user.Email != "" {
		if overseerrID, ok := s.resolveOverseerrUserID(r.Context(), user.Email); ok {
			payload.UserID = &overseerrID
		}
	}

	sanitized, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	data, err := client.CreateRequest(ctx, sanitized)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
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
		writeError(w, http.StatusBadGateway, "upstream service error")
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

	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	// TOCTOU: a concurrent re-assignment between GetRequest and DeleteRequest
	// is theoretically possible but extremely unlikely in practice.
	if user.Role != models.RoleAdmin {
		if user.Email == "" {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		overseerrID, ok := s.resolveOverseerrUserID(ctx, user.Email)
		if !ok || overseerrID == 0 {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		raw, err := client.GetRequest(ctx, id)
		if err != nil {
			writeError(w, http.StatusBadGateway, "upstream service error")
			return
		}

		var reqInfo struct {
			RequestedBy struct {
				ID int `json:"id"`
			} `json:"requestedBy"`
		}
		if err := json.Unmarshal(raw, &reqInfo); err != nil {
			writeError(w, http.StatusBadGateway, "upstream service error")
			return
		}
		if reqInfo.RequestedBy.ID == 0 || reqInfo.RequestedBy.ID != overseerrID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	if err := client.DeleteRequest(ctx, id); err != nil {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
