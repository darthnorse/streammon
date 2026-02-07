package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"unicode/utf8"

	"streammon/internal/models"
	"streammon/internal/store"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateUsername checks username requirements
func validateUsername(username string) error {
	length := utf8.RuneCountInString(username)
	if length < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if length > 32 {
		return fmt.Errorf("username must be at most 32 characters")
	}
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("username can only contain letters, numbers, underscores, and hyphens")
	}
	return nil
}

// writeJSONError writes a safe JSON error response
func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// LocalProvider handles username/password authentication
type LocalProvider struct {
	store   *store.Store
	manager *Manager
}

// NewLocalProvider creates a new local authentication provider
func NewLocalProvider(st *store.Store, mgr *Manager) *LocalProvider {
	return &LocalProvider{
		store:   st,
		manager: mgr,
	}
}

func (p *LocalProvider) Name() ProviderType {
	return ProviderLocal
}

func (p *LocalProvider) Enabled() bool {
	return true // Local auth is always available
}

// HandleLogin processes username/password login
func (p *LocalProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		// Generic error to prevent username enumeration
		writeJSONError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	user, passwordHash, err := p.store.GetUserByUsername(req.Username)
	userFound := err == nil && passwordHash != ""

	// Always perform password verification to prevent timing attacks.
	// If user not found or has no password, verify against a dummy hash.
	hashToVerify := passwordHash
	if !userFound {
		hashToVerify = DummyHash
	}

	valid, _ := VerifyPassword(req.Password, hashToVerify)
	if !userFound || !valid {
		writeJSONError(w, "invalid credentials", http.StatusUnauthorized)
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

// HandleCallback is a no-op for local auth (no OAuth flow)
func (p *LocalProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, "not supported", http.StatusNotFound)
}

// CreateUser creates a new local user (admin only)
func (p *LocalProvider) CreateUser(username, password, email string, role models.Role) (*models.User, error) {
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	return p.store.CreateLocalUser(username, email, hash, role)
}

// CreateFirstAdmin creates the first admin user during setup
func (p *LocalProvider) CreateFirstAdmin(username, password, email string) (*models.User, error) {
	return p.CreateUser(username, password, email, models.RoleAdmin)
}

// UpdatePassword updates a user's password
func (p *LocalProvider) UpdatePassword(userID int64, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	return p.store.UpdatePassword(userID, hash)
}

// HandleSetup processes the setup wizard for local auth
func (p *LocalProvider) HandleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := validateUsername(req.Username); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ValidatePassword(req.Password); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		log.Printf("password hash error: %v", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Atomically create the first admin user (handles race condition)
	user, err := p.store.CreateFirstAdmin(req.Username, req.Email, hash, "local", req.Email, "")
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
