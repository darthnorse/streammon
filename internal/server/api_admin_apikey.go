package server

import (
	"net/http"
	"time"

	"streammon/internal/auth"
)

type apiKeyStatusResponse struct {
	Configured bool       `json:"configured"`
	Key        string     `json:"key,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
}

type apiKeyRotateResponse struct {
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

// handleGetAPIKeyStatus returns the configured key (plaintext) and its
// creation time. Mounted under RequireInteractiveSession so a leaked X-API-Key
// caller cannot extract the key value.
func (s *Server) handleGetAPIKeyStatus(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.GetAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp := apiKeyStatusResponse{Configured: value != ""}
	if resp.Configured {
		resp.Key = value
		createdAt, err := s.store.GetAPIKeyCreatedAt()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		if !createdAt.IsZero() {
			resp.CreatedAt = &createdAt
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleRotateAPIKey generates a new key, stores it, and returns the plaintext.
// Subsequent GETs will also return it; the rotate response is the immediate
// confirmation. Mounted under RequireInteractiveSession.
func (s *Server) handleRotateAPIKey(w http.ResponseWriter, r *http.Request) {
	plain, err := auth.GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	createdAt := time.Now().UTC()
	if err := s.store.SetAPIKey(plain, createdAt); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusCreated, apiKeyRotateResponse{Key: plain, CreatedAt: createdAt})
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ClearAPIKey(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
