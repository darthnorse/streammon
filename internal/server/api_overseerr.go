package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
	"streammon/internal/overseerr"
)

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

var emptyRequestList = json.RawMessage(`{"pageInfo":{"pages":1,"page":1,"results":0},"results":[]}`)

type overseerrUserCache struct {
	mu        sync.RWMutex
	fetchOnce sync.Once
	fetching  chan struct{}  // capacity-1 semaphore: one refresh at a time, cancellable
	emailToID map[string]int // lowercase email → Overseerr user ID
	expiresAt time.Time
	retryAt   time.Time // suppresses refresh attempts after a failed fetch
}

// sem returns the refresh semaphore, creating it on first use. Lazy init keeps
// a zero-value cache usable: a nil channel would block every sender forever.
func (c *overseerrUserCache) sem() chan struct{} {
	c.fetchOnce.Do(func() { c.fetching = make(chan struct{}, 1) })
	return c.fetching
}

const overseerrUserRetryDelay = 30 * time.Second

const overseerrUserCacheTTL = 15 * time.Minute

type overseerrMediaCache struct {
	mu        sync.RWMutex
	statuses  map[string]int // "mediaType:tmdbId" → status
	expiresAt time.Time
	gen       uint64 // bumped on every invalidation; discards stale in-flight refresh writes
}

const overseerrMediaCacheTTL = 5 * time.Minute

func (s *Server) overseerrDeps() integrationDeps {
	return integrationDeps{
		validateURL:  overseerr.ValidateURL,
		newClient:    func(url, apiKey string) (integrationClient, error) { return overseerr.NewClient(url, apiKey) },
		getConfig:    s.store.GetOverseerrConfig,
		setConfig:    s.store.SetOverseerrConfig,
		deleteConfig: s.store.DeleteOverseerrConfig,
		onUpdate:     s.invalidateOverseerrCaches,
		onDelete:     s.invalidateOverseerrCaches,
	}
}

func (s *Server) newOverseerrClient() (*overseerr.Client, error) {
	cfg, err := s.store.GetOverseerrConfig()
	if err != nil {
		return nil, errors.New("overseerr/seerr not available")
	}
	if !cfg.IsUsable() {
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

	writeRawJSON(w, http.StatusOK, redactRequestsForNonAdmin(r, data))
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

	writeRawJSON(w, http.StatusOK, redactRequestsForNonAdmin(r, data))
}

func (s *Server) handleOverseerrMovie(w http.ResponseWriter, r *http.Request) {
	id64, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}
	id := int(id64)

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

	writeRawJSON(w, http.StatusOK, redactRequestsForNonAdmin(r, data))
}

func (s *Server) handleOverseerrTV(w http.ResponseWriter, r *http.Request) {
	id64, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid TV ID")
		return
	}
	id := int(id64)

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

	writeRawJSON(w, http.StatusOK, redactRequestsForNonAdmin(r, data))
}

// redactRequestsForNonAdmin strips every `mediaInfo.requests` field (which
// contains requester PII — email, plexUsername, avatar) from an Overseerr
// response when the caller is not an admin. It walks the full response tree
// so it catches a top-level `mediaInfo` (movie/TV detail endpoints) as well
// as per-result `mediaInfo` fields nested under `results[]` (search/discover
// lists) and `results[].knownFor[]` (person results), matching Overseerr's
// MovieResult/TvResult schema which embeds the same MediaInfo object
// everywhere. Returns the input unchanged if either the caller is an admin
// or the response shape isn't what we expect (e.g. malformed upstream JSON —
// let the original bytes pass rather than fail).
func redactRequestsForNonAdmin(r *http.Request, data []byte) []byte {
	user := UserFromContext(r.Context())
	if user != nil && user.Role == models.RoleAdmin {
		return data
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return data
	}
	stripMediaInfoRequests(parsed)
	out, err := json.Marshal(parsed)
	if err != nil {
		return data
	}
	return out
}

// stripMediaInfoRequests recursively walks v (as produced by decoding JSON
// into `any`) and deletes the "requests" key from every "mediaInfo" object
// found at any nesting depth.
func stripMediaInfoRequests(v any) {
	switch val := v.(type) {
	case map[string]any:
		if mi, ok := val["mediaInfo"].(map[string]any); ok {
			delete(mi, "requests")
		}
		for _, child := range val {
			stripMediaInfoRequests(child)
		}
	case []any:
		for _, child := range val {
			stripMediaInfoRequests(child)
		}
	}
}

func (s *Server) handleOverseerrTVSeason(w http.ResponseWriter, r *http.Request) {
	tvID64, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid TV ID")
		return
	}
	tvID := int(tvID64)
	// Season 0 is a valid, real-world value (specials/extras), so this is
	// deliberately not routed through parseIDParam (which rejects <=0).
	seasonNum, err := strconv.Atoi(chi.URLParam(r, "seasonNumber"))
	if err != nil || seasonNum < 0 {
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

	// Overseerr's Season schema has no mediaInfo of its own today, but redact
	// defensively for consistency with the other raw-passthrough endpoints
	// and in case that ever changes upstream.
	writeRawJSON(w, http.StatusOK, redactRequestsForNonAdmin(r, data))
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
		id, ok, err := s.resolveOverseerrUserID(r.Context(), user.Email)
		if err != nil {
			writeError(w, http.StatusBadGateway, "upstream service error")
			return
		}
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

// writeUpstreamError translates an Overseerr failure into a response the user
// can act on. Impersonated requests are evaluated against the requester's own
// permissions and quota, so a 4xx is usually about their account rather than a
// broken integration — reporting those as 502 sends people chasing the wrong
// problem. The upstream body is never echoed back; only the status is trusted.
func writeUpstreamError(w http.ResponseWriter, err error) {
	var statusErr *overseerr.StatusError
	if !errors.As(err, &statusErr) || statusErr.Code < 400 || statusErr.Code >= 500 {
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	switch statusErr.Code {
	case http.StatusConflict:
		writeError(w, http.StatusConflict, "this title has already been requested")
	case http.StatusForbidden:
		writeError(w, http.StatusForbidden, "Overseerr / Seerr declined this request — check your request permissions and remaining quota")
	default:
		writeError(w, http.StatusUnprocessableEntity, "Overseerr / Seerr rejected this request")
	}
}

// errOverseerrUnavailable reports that the Overseerr user list could not be
// fetched, as distinct from a successful fetch that held no match. Callers must
// keep the two apart: the first is an upstream failure, the second is a real
// "your account is not linked" answer.
var errOverseerrUnavailable = errors.New("overseerr user list unavailable")

// cachedOverseerrUserID returns a cached answer when the map is populated and
// unexpired. The third result reports whether the cache could answer at all —
// distinct from a populated cache that simply holds no match for the email.
func (s *Server) cachedOverseerrUserID(email string) (int, bool, bool) {
	s.overseerrUsers.mu.RLock()
	defer s.overseerrUsers.mu.RUnlock()

	if s.overseerrUsers.emailToID == nil || !time.Now().UTC().Before(s.overseerrUsers.expiresAt) {
		return 0, false, false
	}
	id, ok := s.overseerrUsers.emailToID[email]
	return id, ok, true
}

// resolveOverseerrUserID maps a StreamMon email to an Overseerr user ID.
// Refreshes are serialized: concurrent callers wait for the single in-flight
// fetch rather than each reporting "no match" against a cache that has not been
// populated yet. A non-nil error means the list could not be fetched at all.
func (s *Server) resolveOverseerrUserID(ctx context.Context, email string) (int, bool, error) {
	email = strings.ToLower(email)

	if id, ok, cached := s.cachedOverseerrUserID(email); cached {
		return id, ok, nil
	}

	// Wait for the in-flight refresh, but stay responsive to our own
	// cancellation rather than blocking for the upstream timeout.
	sem := s.overseerrUsers.sem()
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return 0, false, ctx.Err()
	}

	// A concurrent refresh may have completed while we queued here.
	if id, ok, cached := s.cachedOverseerrUserID(email); cached {
		return id, ok, nil
	}

	s.overseerrUsers.mu.RLock()
	backoff := time.Now().UTC().Before(s.overseerrUsers.retryAt)
	s.overseerrUsers.mu.RUnlock()
	if backoff {
		return 0, false, errOverseerrUnavailable
	}

	if err := s.refreshOverseerrUsers(ctx); err != nil {
		return 0, false, err
	}

	id, ok, _ := s.cachedOverseerrUserID(email)
	return id, ok, nil
}

// refreshOverseerrUsers fetches the Overseerr user list and replaces the cache.
// Callers must hold the fetching semaphore.
func (s *Server) refreshOverseerrUsers(ctx context.Context) error {
	fail := func(err error) error {
		s.invalidateOverseerrCaches()
		s.overseerrUsers.mu.Lock()
		s.overseerrUsers.retryAt = time.Now().UTC().Add(overseerrUserRetryDelay)
		s.overseerrUsers.mu.Unlock()
		return err
	}

	client, err := s.newOverseerrClient()
	if err != nil {
		log.Printf("overseerr user resolve: %v", err)
		return fail(errOverseerrUnavailable)
	}

	// The cache is shared, so the fetch must outlive the request that happened
	// to trigger it: a client disconnecting mid-refresh would otherwise arm the
	// failure backoff and starve every other user of a healthy Overseerr.
	resolveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	users, err := client.ListUsers(resolveCtx)
	if err != nil {
		log.Printf("overseerr list users: %v", err)
		return fail(errOverseerrUnavailable)
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
	s.overseerrUsers.retryAt = time.Time{}
	s.overseerrUsers.mu.Unlock()

	return nil
}

func (s *Server) invalidateOverseerrMediaCache() {
	s.overseerrMedia.mu.Lock()
	s.overseerrMedia.statuses = nil
	s.overseerrMedia.expiresAt = time.Time{}
	s.overseerrMedia.gen++
	s.overseerrMedia.mu.Unlock()
}

func (s *Server) invalidateOverseerrCaches() {
	s.overseerrUsers.mu.Lock()
	s.overseerrUsers.emailToID = nil
	s.overseerrUsers.expiresAt = time.Time{}
	s.overseerrUsers.retryAt = time.Time{}
	s.overseerrUsers.mu.Unlock()

	s.invalidateOverseerrMediaCache()
}

func (s *Server) handleOverseerrMediaStatuses(w http.ResponseWriter, r *http.Request) {
	s.overseerrMedia.mu.RLock()
	if time.Now().UTC().Before(s.overseerrMedia.expiresAt) {
		cached := s.overseerrMedia.statuses
		s.overseerrMedia.mu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{"statuses": cached})
		return
	}
	s.overseerrMedia.mu.RUnlock()

	s.overseerrMedia.mu.Lock()
	if time.Now().UTC().Before(s.overseerrMedia.expiresAt) {
		cached := s.overseerrMedia.statuses
		s.overseerrMedia.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"statuses": cached})
		return
	}
	s.overseerrMedia.expiresAt = time.Now().UTC().Add(30 * time.Second)
	gen := s.overseerrMedia.gen
	s.overseerrMedia.mu.Unlock()

	client, err := s.newOverseerrClient()
	if err != nil {
		s.invalidateOverseerrCaches()
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), integrationTimeout)
	defer cancel()

	statuses, err := client.ListMedia(ctx)
	if err != nil {
		s.invalidateOverseerrCaches()
		writeError(w, http.StatusBadGateway, "upstream service error")
		return
	}

	// Only publish if no invalidation (create/approve/decline/delete) raced this
	// fetch; otherwise the result predates the mutation and would mask it for the
	// full TTL.
	s.overseerrMedia.mu.Lock()
	if s.overseerrMedia.gen == gen {
		s.overseerrMedia.statuses = statuses
		s.overseerrMedia.expiresAt = time.Now().UTC().Add(overseerrMediaCacheTTL)
	}
	s.overseerrMedia.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"statuses": statuses})
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

	// Attribute the request to the requesting user so Overseerr applies their
	// own request permission, approval rules and quota rather than the API
	// key owner's. Non-admins must resolve to an Overseerr account, otherwise
	// the request would be silently filed as the admin and auto-approved.
	user := UserFromContext(r.Context())
	isAdmin := user != nil && user.Role == models.RoleAdmin
	hasEmail := user != nil && user.Email != ""

	var (
		overseerrID int
		resolved    bool
	)
	if hasEmail {
		var resolveErr error
		overseerrID, resolved, resolveErr = s.resolveOverseerrUserID(r.Context(), user.Email)
		if resolveErr != nil {
			writeError(w, http.StatusBadGateway, "upstream service error")
			return
		}
	}

	if !isAdmin && !resolved {
		msg := "no matching Overseerr account found for your email"
		if !hasEmail {
			msg = "cannot determine your Overseerr identity; ask an admin to link your account"
		}
		writeError(w, http.StatusUnprocessableEntity, msg)
		return
	}

	var data json.RawMessage
	if resolved {
		data, err = client.CreateRequestAsUser(ctx, overseerrID, reqBody)
	} else {
		data, err = client.CreateRequest(ctx, reqBody)
	}
	if err != nil {
		log.Printf("overseerr: create request for user %d failed: %v", user.ID, err)
		writeUpstreamError(w, err)
		return
	}

	s.invalidateOverseerrMediaCache()
	writeRawJSON(w, http.StatusCreated, data)
}

func (s *Server) handleOverseerrRequestAction(w http.ResponseWriter, r *http.Request) {
	id64, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}
	id := int(id64)

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

	s.invalidateOverseerrMediaCache()
	writeRawJSON(w, http.StatusOK, data)
}

func (s *Server) handleOverseerrDeleteRequest(w http.ResponseWriter, r *http.Request) {
	id64, ok := parseIDParam(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}
	id := int(id64)

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
		overseerrID, ok, err := s.resolveOverseerrUserID(ctx, user.Email)
		if err != nil {
			writeError(w, http.StatusBadGateway, "upstream service error")
			return
		}
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

	s.invalidateOverseerrMediaCache()
	w.WriteHeader(http.StatusNoContent)
}
