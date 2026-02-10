package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

type overseerrUserCache struct {
	mu        sync.RWMutex
	emailToID map[string]int // lowercase email → Overseerr user ID
	expiresAt time.Time
}

type overseerrPlexTokenCache struct {
	mu          sync.RWMutex
	userIDMap   map[int64]int       // StreamMon user ID → Overseerr user ID
	entryExpiry map[int64]time.Time // per-user cache expiry
}

const (
	overseerrUserCacheTTL      = 15 * time.Minute
	overseerrPlexTokenCacheTTL = 15 * time.Minute
	overseerrUserResolveTimeout = 15 * time.Second
	overseerrPlexClaimTTL      = 30 * time.Second
)

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

	s.invalidateOverseerrUserCache()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteOverseerrSettings(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteOverseerrConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.invalidateOverseerrUserCache()
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

func (s *Server) handleOverseerrConfigured(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetOverseerrConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"configured": cfg.URL != "" && cfg.APIKey != "",
	})
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

var allowedDiscoverCategories = map[string]bool{
	"trending":        true,
	"movies":          true,
	"movies/upcoming": true,
	"tv":              true,
	"tv/upcoming":     true,
}

func (s *Server) handleOverseerrDiscover(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "*")
	if strings.Contains(category, "..") || !allowedDiscoverCategories[category] {
		writeError(w, http.StatusNotFound, "unknown discover category")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	client, ctx, cancel, ok := s.overseerrClientWithTimeout(w, r)
	if !ok {
		return
	}
	defer cancel()

	data, err := client.Discover(ctx, category, page)
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

	resolveCtx, cancel := context.WithTimeout(ctx, overseerrUserResolveTimeout)
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

// invalidateOverseerrUserCache clears both the email-based and Plex-token-based
// user ID caches. Both are cleared together because changing the Overseerr
// instance (URL/key) invalidates all previously resolved user IDs.
func (s *Server) invalidateOverseerrUserCache() {
	s.overseerrUsers.mu.Lock()
	s.overseerrUsers.emailToID = nil
	s.overseerrUsers.expiresAt = time.Time{}
	s.overseerrUsers.mu.Unlock()

	s.invalidatePlexTokenCache()
}

func (s *Server) invalidatePlexTokenCache() {
	s.overseerrPlexCache.mu.Lock()
	s.overseerrPlexCache.userIDMap = make(map[int64]int)
	s.overseerrPlexCache.entryExpiry = make(map[int64]time.Time)
	s.overseerrPlexCache.mu.Unlock()
}

// resolveOverseerrUserWithPlex attempts to resolve a StreamMon user's Overseerr
// user ID by authenticating with their stored Plex token. This auto-creates
// the user in Overseerr if they don't exist yet.
func (s *Server) resolveOverseerrUserWithPlex(ctx context.Context, userID int64) (int, bool) {
	s.overseerrPlexCache.mu.RLock()
	if expiry, ok := s.overseerrPlexCache.entryExpiry[userID]; ok && time.Now().UTC().Before(expiry) {
		id := s.overseerrPlexCache.userIDMap[userID]
		s.overseerrPlexCache.mu.RUnlock()
		return id, true
	}
	s.overseerrPlexCache.mu.RUnlock()

	// Write lock, re-check, claim with short expiry to prevent thundering herd
	s.overseerrPlexCache.mu.Lock()
	if expiry, ok := s.overseerrPlexCache.entryExpiry[userID]; ok && time.Now().UTC().Before(expiry) {
		id := s.overseerrPlexCache.userIDMap[userID]
		s.overseerrPlexCache.mu.Unlock()
		return id, true
	}
	claimedExpiry := time.Now().UTC().Add(overseerrPlexClaimTTL)
	s.overseerrPlexCache.entryExpiry[userID] = claimedExpiry
	s.overseerrPlexCache.mu.Unlock()

	// clearClaim removes the short-lived claim so a future call can retry,
	// but only if the expiry still matches ours (avoids clobbering a later claim).
	clearClaim := func() {
		s.overseerrPlexCache.mu.Lock()
		if exp, ok := s.overseerrPlexCache.entryExpiry[userID]; ok && exp.Equal(claimedExpiry) {
			delete(s.overseerrPlexCache.userIDMap, userID)
			delete(s.overseerrPlexCache.entryExpiry, userID)
		}
		s.overseerrPlexCache.mu.Unlock()
	}

	enabled, _ := s.store.GetStorePlexTokens()
	if !enabled {
		clearClaim()
		return 0, false
	}

	plexToken, err := s.store.GetProviderToken(userID, store.ProviderPlex)
	if err != nil || plexToken == "" {
		clearClaim()
		return 0, false
	}

	cfg, err := s.store.GetOverseerrConfig()
	if err != nil || cfg.URL == "" || cfg.APIKey == "" {
		clearClaim()
		return 0, false
	}

	s.warnHTTPOnce.Do(func() {
		if strings.HasPrefix(cfg.URL, "http://") {
			log.Printf("WARNING: Overseerr URL uses plain HTTP (%s). Plex tokens will be sent unencrypted. Use HTTPS for secure token transmission.", cfg.URL)
		}
	})

	client, err := s.newOverseerrClient()
	if err != nil {
		log.Printf("overseerr plex auth: client error: %v", err)
		clearClaim()
		return 0, false
	}

	resolveCtx, cancel := context.WithTimeout(ctx, overseerrUserResolveTimeout)
	defer cancel()

	overseerrID, err := client.AuthenticateWithPlex(resolveCtx, plexToken)
	if err != nil {
		log.Printf("overseerr plex auth for user %d: %v", userID, err)
		clearClaim()
		return 0, false
	}

	// Cache the result with full TTL and sweep expired entries
	now := time.Now().UTC()
	s.overseerrPlexCache.mu.Lock()
	s.overseerrPlexCache.userIDMap[userID] = overseerrID
	s.overseerrPlexCache.entryExpiry[userID] = now.Add(overseerrPlexTokenCacheTTL)
	for uid, exp := range s.overseerrPlexCache.entryExpiry {
		if now.After(exp) {
			delete(s.overseerrPlexCache.userIDMap, uid)
			delete(s.overseerrPlexCache.entryExpiry, uid)
		}
	}
	s.overseerrPlexCache.mu.Unlock()

	return overseerrID, true
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

	payload := overseerrCreateRequestPayload{
		overseerrCreateRequestBody: req,
	}

	if user := UserFromContext(r.Context()); user != nil {
		// Try Plex token auth first (auto-creates user in Overseerr)
		if overseerrID, ok := s.resolveOverseerrUserWithPlex(r.Context(), user.ID); ok {
			payload.UserID = &overseerrID
		} else if user.Email != "" {
			// Fall back to email matching
			if overseerrID, ok := s.resolveOverseerrUserID(r.Context(), user.Email); ok {
				payload.UserID = &overseerrID
			}
		}
	}

	sanitized, err := json.Marshal(payload)
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
