package server

import (
	"net/http"
	"time"

	"streammon/internal/auth"
)

type apiKeyStatusResponse struct {
	Configured bool       `json:"configured"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
}

type apiKeyRotateResponse struct {
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) handleGetAPIKeyStatus(w http.ResponseWriter, r *http.Request) {
	hash, err := s.store.GetAPIKeyHash()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp := apiKeyStatusResponse{Configured: hash != ""}
	if resp.Configured {
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

// handleRotateAPIKey generates a new key, stores its hash, and returns the
// plaintext to the caller exactly once. The route is mounted under
// RequireInteractiveSession so a leaked API key cannot self-rotate.
func (s *Server) handleRotateAPIKey(w http.ResponseWriter, r *http.Request) {
	plain, err := auth.GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	createdAt := time.Now().UTC()
	if err := s.store.SetAPIKey(auth.HashAPIKey(plain), createdAt); err != nil {
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
