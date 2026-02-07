package auth

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"streammon/internal/store"
)

// Manager coordinates multiple authentication providers
type Manager struct {
	mu        sync.RWMutex
	store     *store.Store
	providers map[ProviderType]Provider
}

// NewManager creates a new auth manager
func NewManager(st *store.Store) *Manager {
	return &Manager{
		store:     st,
		providers: make(map[ProviderType]Provider),
	}
}

// Store returns the underlying store
func (m *Manager) Store() *store.Store {
	return m.store
}

// RegisterProvider adds a provider to the manager
func (m *Manager) RegisterProvider(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
}

// GetProvider returns a provider by type
func (m *Manager) GetProvider(pt ProviderType) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[pt]
	return p, ok
}

// GetEnabledProviders returns list of enabled provider names
func (m *Manager) GetEnabledProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var enabled []string
	for _, p := range m.providers {
		if p.Enabled() {
			enabled = append(enabled, string(p.Name()))
		}
	}
	return enabled
}

// IsSetupRequired checks if initial setup is needed (no users exist)
func (m *Manager) IsSetupRequired() (bool, error) {
	return m.store.IsSetupRequired()
}

// Reload refreshes provider configurations (called when settings change)
func (m *Manager) Reload() error {
	// Currently a no-op, but provides hook for future provider reloads
	return nil
}

// HandleLogin routes to the appropriate provider
func (m *Manager) HandleLogin(w http.ResponseWriter, r *http.Request, pt ProviderType) {
	p, ok := m.GetProvider(pt)
	if !ok || !p.Enabled() {
		http.Error(w, `{"error":"provider not available"}`, http.StatusNotFound)
		return
	}
	p.HandleLogin(w, r)
}

// HandleCallback routes to the appropriate provider's callback handler
func (m *Manager) HandleCallback(w http.ResponseWriter, r *http.Request, pt ProviderType) {
	p, ok := m.GetProvider(pt)
	if !ok || !p.Enabled() {
		http.Error(w, `{"error":"provider not available"}`, http.StatusNotFound)
		return
	}
	p.HandleCallback(w, r)
}

// CreateSession creates a new session for the user and sets the cookie
func (m *Manager) CreateSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, err := m.store.CreateSession(userID, time.Now().UTC().Add(SessionDuration))
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

// HandleGetProviders returns the list of enabled providers
func (m *Manager) HandleGetProviders(w http.ResponseWriter, r *http.Request) {
	type providerInfo struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}

	m.mu.RLock()
	providers := make([]providerInfo, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, providerInfo{
			Name:    string(p.Name()),
			Enabled: p.Enabled(),
		})
	}
	m.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

// HandleGetStatus returns auth status (setup required, current user, etc.)
func (m *Manager) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	setupRequired, _ := m.IsSetupRequired()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"setup_required":    setupRequired,
		"enabled_providers": m.GetEnabledProviders(),
	})
}

// HandleLogout invalidates the current session
func (m *Manager) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil {
		m.store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
