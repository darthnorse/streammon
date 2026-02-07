package server

import (
	"net/http"

	"streammon/internal/auth"
)

// handleSetupLocal handles first admin setup via local credentials
func (s *Server) handleSetupLocal(w http.ResponseWriter, r *http.Request) {
	if s.authManager == nil {
		http.Error(w, `{"error":"auth not configured"}`, http.StatusInternalServerError)
		return
	}

	local, ok := s.authManager.GetProvider(auth.ProviderLocal)
	if !ok {
		http.Error(w, `{"error":"local auth not available"}`, http.StatusInternalServerError)
		return
	}

	localProvider, ok := local.(*auth.LocalProvider)
	if !ok {
		http.Error(w, `{"error":"local auth not available"}`, http.StatusInternalServerError)
		return
	}

	localProvider.HandleSetup(w, r)
}

// handleSetupPlex handles first admin setup via Plex.tv
func (s *Server) handleSetupPlex(w http.ResponseWriter, r *http.Request) {
	if s.authManager == nil {
		http.Error(w, `{"error":"auth not configured"}`, http.StatusInternalServerError)
		return
	}

	plex, ok := s.authManager.GetProvider(auth.ProviderPlex)
	if !ok {
		http.Error(w, `{"error":"plex auth not available"}`, http.StatusInternalServerError)
		return
	}

	plexProvider, ok := plex.(*auth.PlexProvider)
	if !ok {
		http.Error(w, `{"error":"plex auth not available"}`, http.StatusInternalServerError)
		return
	}

	plexProvider.HandleSetup(w, r)
}

// handleLocalLogin handles local username/password login
func (s *Server) handleLocalLogin(w http.ResponseWriter, r *http.Request) {
	if s.authManager == nil {
		http.Error(w, `{"error":"auth not configured"}`, http.StatusInternalServerError)
		return
	}

	local, ok := s.authManager.GetProvider(auth.ProviderLocal)
	if !ok || !local.Enabled() {
		http.Error(w, `{"error":"local auth not available"}`, http.StatusNotFound)
		return
	}

	local.HandleLogin(w, r)
}

// handlePlexLogin handles Plex.tv token-based login
func (s *Server) handlePlexLogin(w http.ResponseWriter, r *http.Request) {
	if s.authManager == nil {
		http.Error(w, `{"error":"auth not configured"}`, http.StatusInternalServerError)
		return
	}

	plex, ok := s.authManager.GetProvider(auth.ProviderPlex)
	if !ok || !plex.Enabled() {
		http.Error(w, `{"error":"plex auth not available"}`, http.StatusNotFound)
		return
	}

	plex.HandleLogin(w, r)
}

// handleOIDCLogin initiates OIDC login flow
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if s.authManager == nil {
		http.Error(w, `{"error":"auth not configured"}`, http.StatusInternalServerError)
		return
	}

	oidc, ok := s.authManager.GetProvider(auth.ProviderOIDC)
	if !ok || !oidc.Enabled() {
		http.Error(w, `{"error":"OIDC not configured"}`, http.StatusNotFound)
		return
	}

	oidc.HandleLogin(w, r)
}

// handleOIDCCallback processes OIDC callback
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.authManager == nil {
		http.Error(w, `{"error":"auth not configured"}`, http.StatusInternalServerError)
		return
	}

	oidc, ok := s.authManager.GetProvider(auth.ProviderOIDC)
	if !ok || !oidc.Enabled() {
		http.Error(w, `{"error":"OIDC not configured"}`, http.StatusNotFound)
		return
	}

	oidc.HandleCallback(w, r)
}
