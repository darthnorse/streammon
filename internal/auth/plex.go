package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

const (
	plexAPIBase    = "https://plex.tv/api/v2"
	plexAPITimeout = 10 * time.Second
)

// PlexProvider handles Plex.tv OAuth authentication
type PlexProvider struct {
	store   *store.Store
	manager *Manager
	client  *http.Client
}

// NewPlexProvider creates a new Plex authentication provider
func NewPlexProvider(st *store.Store, mgr *Manager) *PlexProvider {
	return &PlexProvider{
		store:   st,
		manager: mgr,
		client: &http.Client{
			Timeout: plexAPITimeout,
		},
	}
}

func (p *PlexProvider) Name() ProviderType {
	return ProviderPlex
}

func (p *PlexProvider) Enabled() bool {
	return true // Plex auth is always available (uses plex.tv)
}

// plexUser represents user info from Plex.tv API
type plexUser struct {
	ID       int    `json:"id"`
	UUID     string `json:"uuid"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Title    string `json:"title"`
	Thumb    string `json:"thumb"`
}

// plexResource represents a Plex server resource
type plexResource struct {
	Name             string `json:"name"`
	ClientIdentifier string `json:"clientIdentifier"`
	Provides         string `json:"provides"`
	Owned            bool   `json:"owned"`
	Home             bool   `json:"home"`
}

// HandleLogin processes Plex auth token from frontend PIN flow
func (p *PlexProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthToken string `json:"auth_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.AuthToken == "" {
		writeJSONError(w, "auth_token is required", http.StatusBadRequest)
		return
	}

	// Verify token with Plex.tv
	plexUser, err := p.verifyToken(r.Context(), req.AuthToken)
	if err != nil {
		log.Printf("Plex token verification error: %v", err)
		writeJSONError(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Check server access if guest mode disabled
	guestAccess, _ := p.store.GetPlexGuestAccess()
	if !guestAccess {
		hasAccess, err := p.verifyServerAccess(r.Context(), req.AuthToken)
		if err != nil {
			log.Printf("Plex server access check error: %v", err)
			writeJSONError(w, "failed to verify server access", http.StatusInternalServerError)
			return
		}
		if !hasAccess {
			writeJSONError(w, "no access to configured Plex servers", http.StatusForbidden)
			return
		}
	}

	// Get or create user with account linking
	providerID := fmt.Sprintf("%d", plexUser.ID)
	user, err := p.store.GetOrLinkUserByEmail(
		plexUser.Email,
		plexUser.Title, // Display name
		string(ProviderPlex),
		providerID,
		plexUser.Thumb,
	)
	if err != nil {
		log.Printf("user creation error: %v", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create session
	if err := p.manager.CreateSession(w, r, user.ID); err != nil {
		log.Printf("session creation error: %v", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// HandleCallback is not used for Plex (PIN flow handled client-side)
func (p *PlexProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, "not supported", http.StatusNotFound)
}

// verifyToken validates the auth token with Plex.tv and returns user info
func (p *PlexProvider) verifyToken(ctx context.Context, token string) (*plexUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", plexAPIBase+"/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Plex-Token", token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plex API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex API returned status %d", resp.StatusCode)
	}

	var user plexUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding plex user: %w", err)
	}

	return &user, nil
}

// verifyServerAccess checks if user has access to any configured Plex server
func (p *PlexProvider) verifyServerAccess(ctx context.Context, token string) (bool, error) {
	// Get configured Plex servers
	servers, err := p.store.ListServers()
	if err != nil {
		return false, fmt.Errorf("listing servers: %w", err)
	}

	// Build set of configured Plex server names (normalized for comparison)
	configuredNames := make(map[string]bool)
	for _, s := range servers {
		if s.Type == models.ServerTypePlex && s.Enabled {
			configuredNames[strings.ToLower(s.Name)] = true
		}
	}

	// If no Plex servers configured, allow access (no restriction)
	if len(configuredNames) == 0 {
		return true, nil
	}

	// Fetch user's resources from Plex.tv
	req, err := http.NewRequestWithContext(ctx, "GET", plexAPIBase+"/resources?includeHttps=1", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Plex-Token", token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("plex API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("plex API returned status %d", resp.StatusCode)
	}

	var resources []plexResource
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return false, fmt.Errorf("decoding plex resources: %w", err)
	}

	// Check if user has access to any of our configured servers
	for _, r := range resources {
		if r.Provides != "server" {
			continue
		}
		// Match by server name (case-insensitive)
		if configuredNames[strings.ToLower(r.Name)] {
			log.Printf("Plex user has access to configured server: %s", r.Name)
			return true, nil
		}
	}

	return false, nil
}

// HandleSetup processes Plex auth for setup wizard
func (p *PlexProvider) HandleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthToken string `json:"auth_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.AuthToken == "" {
		writeJSONError(w, "auth_token is required", http.StatusBadRequest)
		return
	}

	// Verify token with Plex.tv
	plexUser, err := p.verifyToken(r.Context(), req.AuthToken)
	if err != nil {
		log.Printf("Plex token verification error: %v", err)
		writeJSONError(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Atomically create the first admin user (handles race condition)
	providerID := fmt.Sprintf("%d", plexUser.ID)
	user, err := p.store.CreateFirstAdmin(plexUser.Title, plexUser.Email, "", string(ProviderPlex), providerID, plexUser.Thumb)
	if err != nil {
		if errors.Is(err, store.ErrSetupComplete) {
			writeJSONError(w, "setup already complete", http.StatusConflict)
			return
		}
		log.Printf("create first admin error: %v", err)
		writeJSONError(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	// Create session
	if err := p.manager.CreateSession(w, r, user.ID); err != nil {
		log.Printf("session creation error: %v", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
