package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"streammon/internal/httputil"
	"streammon/internal/models"
	"streammon/internal/store"
)

// MediaServerProvider handles authentication against Emby and Jellyfin servers.
// A single implementation is shared because both use the identical AuthenticateByName API.
type MediaServerProvider struct {
	store      *store.Store
	manager    *Manager
	client     *http.Client
	serverType models.ServerType
}

func NewMediaServerProvider(st *store.Store, mgr *Manager, serverType models.ServerType) *MediaServerProvider {
	return &MediaServerProvider{
		store:      st,
		manager:    mgr,
		client:     httputil.NewClient(),
		serverType: serverType,
	}
}

func (p *MediaServerProvider) Name() ProviderType {
	switch p.serverType {
	case models.ServerTypeEmby:
		return ProviderEmby
	case models.ServerTypeJellyfin:
		return ProviderJellyfin
	default:
		log.Printf("WARNING: unexpected server type %q in MediaServerProvider.Name()", p.serverType)
		return ProviderType(p.serverType)
	}
}

func (p *MediaServerProvider) Enabled() bool {
	servers, err := p.getEnabledServers()
	if err != nil {
		log.Printf("error checking %s servers: %v", p.serverType, err)
		return false
	}
	return len(servers) > 0
}

func (p *MediaServerProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, "not supported", http.StatusNotFound)
}

type mediaServerLoginRequest struct {
	ServerID int64  `json:"server_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	User        authUser `json:"User"`
	AccessToken string   `json:"AccessToken"`
}

type authUser struct {
	ID     string     `json:"Id"`
	Name   string     `json:"Name"`
	Policy authPolicy `json:"Policy"`
}

type authPolicy struct {
	IsAdministrator bool `json:"IsAdministrator"`
}

// validateAndAuthenticate decodes the request, validates the server, and authenticates
// against the media server. Returns the validated request, server, and auth response.
func (p *MediaServerProvider) validateAndAuthenticate(w http.ResponseWriter, r *http.Request) (*mediaServerLoginRequest, *models.Server, *authResponse, bool) {
	var req mediaServerLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return nil, nil, nil, false
	}

	if req.Username == "" || req.Password == "" || req.ServerID == 0 {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return nil, nil, nil, false
	}

	srv, err := p.store.GetServer(req.ServerID)
	if err != nil || srv.Type != p.serverType || !srv.Enabled || srv.DeletedAt != nil {
		writeJSONError(w, "server not available", http.StatusBadRequest)
		return nil, nil, nil, false
	}

	authResp, err := p.authenticate(r, srv, req.Username, req.Password)
	if err != nil {
		log.Printf("%s auth error for %s: %v", p.serverType, req.Username, err)
		writeJSONError(w, "invalid credentials", http.StatusUnauthorized)
		return nil, nil, nil, false
	}

	if authResp.User.ID == "" || authResp.User.Name == "" {
		log.Printf("%s auth returned empty user ID or name", p.serverType)
		writeJSONError(w, "invalid server response", http.StatusBadGateway)
		return nil, nil, nil, false
	}

	return &req, srv, authResp, true
}

func (p *MediaServerProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
	_, srv, authResp, ok := p.validateAndAuthenticate(w, r)
	if !ok {
		return
	}

	providerID := fmt.Sprintf("%d:%s", srv.ID, authResp.User.ID)
	existingUser, _ := p.store.GetUserByProvider(string(p.Name()), providerID)

	guestAccess, _ := p.store.GetGuestAccess()
	if !guestAccess {
		isAdmin := existingUser != nil && existingUser.Role == models.RoleAdmin
		if !isAdmin && !authResp.User.Policy.IsAdministrator {
			writeJSONError(w, "guest access is disabled", http.StatusForbidden)
			return
		}
	}

	var user *models.User
	if existingUser != nil {
		user = existingUser
	} else {
		var err error
		user, err = p.store.GetOrLinkUserByEmail(
			"",
			authResp.User.Name,
			string(p.Name()),
			providerID,
			"",
		)
		if err != nil {
			log.Printf("user creation error: %v", err)
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

func (p *MediaServerProvider) HandleSetup(w http.ResponseWriter, r *http.Request) {
	_, srv, authResp, ok := p.validateAndAuthenticate(w, r)
	if !ok {
		return
	}

	if !authResp.User.Policy.IsAdministrator {
		writeJSONError(w, "only server administrators can create the first admin account", http.StatusForbidden)
		return
	}

	providerID := fmt.Sprintf("%d:%s", srv.ID, authResp.User.ID)
	user, err := p.store.CreateFirstAdmin(authResp.User.Name, "", "", string(p.Name()), providerID, "")
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

type serverInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (p *MediaServerProvider) HandleGetServers(w http.ResponseWriter, r *http.Request) {
	servers, err := p.getEnabledServers()
	if err != nil {
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	infos := make([]serverInfo, len(servers))
	for i, s := range servers {
		infos[i] = serverInfo{ID: s.ID, Name: s.Name}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(infos)
}

func (p *MediaServerProvider) getEnabledServers() ([]models.Server, error) {
	all, err := p.store.ListServers()
	if err != nil {
		return nil, err
	}
	var filtered []models.Server
	for _, s := range all {
		if s.Type == p.serverType && s.Enabled {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (p *MediaServerProvider) authenticate(r *http.Request, srv *models.Server, username, password string) (*authResponse, error) {
	serverURL := strings.TrimRight(srv.URL, "/")

	if !strings.HasPrefix(serverURL, "https://") {
		log.Printf("WARNING: authenticating against %s server %q over insecure HTTP", p.serverType, srv.Name)
	}

	body, err := json.Marshal(map[string]string{
		"Username": username,
		"Pw":       password,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, serverURL+"/Users/AuthenticateByName", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Authorization",
		fmt.Sprintf(`MediaBrowser Client="StreamMon", Device="StreamMon", DeviceId="streammon-%d", Version="1.0"`, srv.ID))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", p.serverType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", p.serverType, resp.StatusCode)
	}

	var authResp authResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("parsing %s auth response: %w", p.serverType, err)
	}

	return &authResp, nil
}
