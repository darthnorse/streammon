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

	// Guest access disabled = only existing admins can login
	guestAccess, _ := p.store.GetPlexGuestAccess()
	if !guestAccess {
		isAdmin := (existingUser != nil && existingUser.Role == models.RoleAdmin) ||
			(emailUser != nil && emailUser.Role == models.RoleAdmin)
		if !isAdmin {
			writeJSONError(w, "plex login not enabled for non-admin users", http.StatusForbidden)
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
		var linkErr error
		user, linkErr = p.store.GetOrLinkUserByEmail(
			plexUser.Email,
			plexUser.Title,
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

	// Build lookup maps: machine_id (secure) and name (legacy fallback)
	configuredMachineIDs := make(map[string]bool)
	configuredNames := make(map[string]bool)
	for _, s := range servers {
		if s.Type == models.ServerTypePlex && s.Enabled {
			if s.MachineID != "" {
				configuredMachineIDs[s.MachineID] = true
			} else {
				// Legacy servers without machine_id use name matching
				configuredNames[strings.ToLower(s.Name)] = true
			}
		}
	}

	if len(configuredMachineIDs) == 0 && len(configuredNames) == 0 {
		return false, nil // No Plex servers configured
	}

	var resources []plexResource
	if err := p.doPlexRequest(ctx, token, "/resources?includeHttps=1", &resources); err != nil {
		return false, err
	}

	for _, r := range resources {
		if r.Provides != "server" {
			continue
		}
		// Prefer machine_id match (secure, non-spoofable)
		if configuredMachineIDs[r.ClientIdentifier] {
			log.Printf("Plex user has access to configured server: %s (machine_id: %s)", r.Name, r.ClientIdentifier)
			return true, nil
		}
		// Fallback to name match for legacy servers (less secure)
		if configuredNames[strings.ToLower(r.Name)] {
			log.Printf("Plex user has access to configured server: %s (name match only - consider updating server config)", r.Name)
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
