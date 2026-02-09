package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

const (
	plexAPIBase    = "https://plex.tv/api/v2"
	plexAPITimeout = 10 * time.Second
)

type PlexProvider struct {
	store   *store.Store
	manager *Manager
	client  *http.Client
}

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
	return true
}

type plexUser struct {
	ID       int    `json:"id"`
	UUID     string `json:"uuid"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Title    string `json:"title"`
	Thumb    string `json:"thumb"`
}

type plexResource struct {
	Name             string `json:"name"`
	ClientIdentifier string `json:"clientIdentifier"`
	Provides         string `json:"provides"`
	Owned            bool   `json:"owned"`
	Home             bool   `json:"home"`
}

func (p *PlexProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthToken string `json:"auth_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.AuthToken == "" {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	plexUser, err := p.verifyToken(r.Context(), req.AuthToken)
	if err != nil {
		log.Printf("Plex token verification error: %v", err)
		writeJSONError(w, "invalid token", http.StatusUnauthorized)
		return
	}

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

	providerID := fmt.Sprintf("%d", plexUser.ID)
	existingUser, _ := p.store.GetUserByProvider(string(ProviderPlex), providerID)

	// Also check by email for admins who haven't linked Plex yet
	var emailUser *models.User
	if existingUser == nil && plexUser.Email != "" {
		emailUser, _ = p.store.GetUserByEmail(plexUser.Email)
	}

	guestAccess, _ := p.store.GetGuestAccess()
	if !guestAccess {
		isAdmin := (existingUser != nil && existingUser.Role == models.RoleAdmin) ||
			(emailUser != nil && emailUser.Role == models.RoleAdmin)
		if !isAdmin {
			writeJSONError(w, "guest access is disabled", http.StatusForbidden)
			return
		}
	}

	var user *models.User
	if existingUser != nil {
		user = existingUser
		if plexUser.Thumb != "" && plexUser.Thumb != user.ThumbURL {
			_ = p.store.UpdateUserAvatar(user.Name, plexUser.Thumb)
			user.ThumbURL = plexUser.Thumb
		}
	} else {
		// Try to link by username first, then by title (display name)
		// This handles cases where streaming users might be stored under either name
		namesToTry := []string{plexUser.Username, plexUser.Title}
		displayName := plexUser.Title
		if displayName == "" {
			displayName = plexUser.Username
		}

		var linkErr error
		user, linkErr = p.store.GetOrLinkUser(
			plexUser.Email,
			namesToTry,
			displayName,
			string(ProviderPlex),
			providerID,
			plexUser.Thumb,
		)
		if linkErr != nil {
			log.Printf("user creation error: %v", linkErr)
			writeJSONError(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	if err := p.manager.CreateSessionAndRespond(w, r, user, http.StatusOK); err != nil {
		log.Printf("session creation error: %v", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func (p *PlexProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, "not supported", http.StatusNotFound)
}

func (p *PlexProvider) doPlexRequest(ctx context.Context, token, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", plexAPIBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Plex-Token", token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("plex API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plex API returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding plex response: %w", err)
	}
	return nil
}

func (p *PlexProvider) verifyToken(ctx context.Context, token string) (*plexUser, error) {
	var user plexUser
	if err := p.doPlexRequest(ctx, token, "/user", &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (p *PlexProvider) verifyServerAccess(ctx context.Context, token string) (bool, error) {
	servers, err := p.store.ListServers()
	if err != nil {
		return false, fmt.Errorf("listing servers: %w", err)
	}

	// Build lookup map for machine_id only (name matching removed for security)
	configuredMachineIDs := make(map[string]bool)
	var legacyServerNames []string
	for _, s := range servers {
		if s.Type == models.ServerTypePlex && s.Enabled {
			if s.MachineID != "" {
				configuredMachineIDs[s.MachineID] = true
			} else {
				// Track legacy servers for warning - they won't be matched
				legacyServerNames = append(legacyServerNames, s.Name)
			}
		}
	}

	if len(legacyServerNames) > 0 {
		log.Printf("WARNING: %d Plex server(s) have no machine_id and cannot be used for auth: %v. Edit each server and click 'Test Connection' to fix.", len(legacyServerNames), legacyServerNames)
	}

	if len(configuredMachineIDs) == 0 {
		return false, nil // No Plex servers with machine_id configured
	}

	var resources []plexResource
	if err := p.doPlexRequest(ctx, token, "/resources?includeHttps=1", &resources); err != nil {
		return false, err
	}

	for _, r := range resources {
		if r.Provides != "server" {
			continue
		}
		// Only match on machine_id (secure, non-spoofable)
		if configuredMachineIDs[r.ClientIdentifier] {
			log.Printf("Plex user has access to configured server: %s (machine_id: %s)", r.Name, r.ClientIdentifier)
			return true, nil
		}
	}

	return false, nil
}

func (p *PlexProvider) HandleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthToken string `json:"auth_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.AuthToken == "" {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	plexUser, err := p.verifyToken(r.Context(), req.AuthToken)
	if err != nil {
		log.Printf("Plex token verification error: %v", err)
		writeJSONError(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Atomic insert prevents race condition during setup
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

	if err := p.manager.CreateSessionAndRespond(w, r, user, http.StatusCreated); err != nil {
		log.Printf("session creation error: %v", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}
}
