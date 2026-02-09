package auth

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

type Manager struct {
	mu        sync.RWMutex
	store     *store.Store
	providers map[ProviderType]Provider
}

func NewManager(st *store.Store) *Manager {
	return &Manager{
		store:     st,
		providers: make(map[ProviderType]Provider),
	}
}

func (m *Manager) Store() *store.Store {
	return m.store
}

func (m *Manager) RegisterProvider(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
}

func (m *Manager) GetProvider(pt ProviderType) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[pt]
	return p, ok
}

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

func (m *Manager) IsSetupRequired() (bool, error) {
	return m.store.IsSetupRequired()
}

// Reload is a no-op hook for future provider config reloads
func (m *Manager) Reload() error {
	return nil
}


func (m *Manager) CreateSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, err := m.store.CreateSession(userID, time.Now().UTC().Add(SessionDuration))
	if err != nil {
		return err
	}

	http.SetCookie(w, makeCookie(CookieName, token, "/", int(SessionDuration.Seconds()), r))
	return nil
}

// CreateSessionAndRespond creates session and writes user JSON.
// statusCode: http.StatusOK for login, http.StatusCreated for setup.
func (m *Manager) CreateSessionAndRespond(w http.ResponseWriter, r *http.Request, user *models.User, statusCode int) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	if err := m.CreateSession(w, r, user.ID); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	if statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
	}
	_, err = w.Write(data)
	return err
}

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

func (m *Manager) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	setupRequired, _ := m.IsSetupRequired()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"setup_required":    setupRequired,
		"enabled_providers": m.GetEnabledProviders(),
	})
}

func (m *Manager) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil {
		m.store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, clearCookie(CookieName, "/", r))
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
