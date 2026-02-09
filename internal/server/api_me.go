package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"

	"streammon/internal/auth"
	"streammon/internal/models"
	"streammon/internal/store"
)

type meResponse struct {
	*models.User
	HasPassword bool `json:"has_password"`
}

func (s *Server) writeMeJSON(w http.ResponseWriter, user *models.User) {
	hash, err := s.store.GetPasswordHashByUserID(user.ID)
	if err != nil {
		log.Printf("getting password hash for user %d: %v", user.ID, err)
	}
	writeJSON(w, http.StatusOK, meResponse{User: user, HasPassword: hash != ""})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	s.writeMeJSON(w, user)
}

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email != "" && !emailRegex.MatchString(req.Email) {
		writeError(w, http.StatusBadRequest, "invalid email format")
		return
	}

	if err := s.store.UpdateUserEmail(user.ID, req.Email); err != nil {
		if errors.Is(err, store.ErrEmailInUse) {
			writeError(w, http.StatusConflict, "email already in use")
			return
		}
		log.Printf("updating email for user %d: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	// Re-fetch user from DB to avoid mutating the context pointer
	updated, err := s.store.GetUserByID(user.ID)
	if err != nil {
		log.Printf("re-fetching user %d: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	s.writeMeJSON(w, updated)
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	passwordHash, err := s.store.GetPasswordHashByUserID(user.ID)
	if err != nil {
		log.Printf("getting password hash for user %d: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if passwordHash == "" {
		writeError(w, http.StatusBadRequest, "no password set for this account")
		return
	}

	valid, err := auth.VerifyPassword(req.CurrentPassword, passwordHash)
	if err != nil {
		log.Printf("verifying password for user %d: %v", user.ID, err)
	}
	if !valid {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	localProvider, ok := s.authManager.GetProvider(auth.ProviderLocal)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	lp, ok := localProvider.(*auth.LocalProvider)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := lp.UpdatePassword(user.ID, req.NewPassword); err != nil {
		if errors.Is(err, auth.ErrPasswordTooShort) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("updating password for user %d: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if cookie, err := r.Cookie(auth.CookieName); err == nil {
		if err := s.store.DeleteUserSessionsExcept(user.ID, cookie.Value); err != nil {
			log.Printf("invalidating sessions for user %d: %v", user.ID, err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
