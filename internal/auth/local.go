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

func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

type LocalProvider struct {
	store   *store.Store
	manager *Manager
}

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
	return true
}

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
		writeJSONError(w, "invalid credentials", http.StatusUnauthorized) // Generic to prevent enumeration
		return
	}

	user, passwordHash, err := p.store.GetUserByUsername(req.Username)
	userFound := err == nil && passwordHash != ""

	// Always verify against a hash (real or dummy) to prevent timing attacks
	hashToVerify := passwordHash
	if !userFound {
		hashToVerify = DummyHash
	}

	valid, _ := VerifyPassword(req.Password, hashToVerify)
	if !userFound || !valid {
		writeJSONError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if user.Role != models.RoleAdmin {
		guestAccess, _ := p.store.GetGuestAccess()
		if !guestAccess {
			writeJSONError(w, "guest access is disabled", http.StatusForbidden)
			return
		}
	}

	if err := p.manager.CreateSessionAndRespond(w, r, user, http.StatusOK); err != nil {
		log.Printf("session creation error: %v", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func (p *LocalProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, "not supported", http.StatusNotFound)
}

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

func (p *LocalProvider) CreateFirstAdmin(username, password, email string) (*models.User, error) {
	return p.CreateUser(username, password, email, models.RoleAdmin)
}

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

	if err := p.manager.CreateSessionAndRespond(w, r, user, http.StatusCreated); err != nil {
		log.Printf("session creation error: %v", err)
		writeJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}
}
