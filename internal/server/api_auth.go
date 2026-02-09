package server

import (
	"net/http"

	"streammon/internal/auth"
)

// providerHandler looks up a provider by type and returns it.
// Returns nil, false if authManager is nil or the provider isn't registered.
func (s *Server) providerHandler(pt auth.ProviderType) (auth.Provider, bool) {
	if s.authManager == nil {
		return nil, false
	}
	return s.authManager.GetProvider(pt)
}

// mediaServerHandler is a typed version of providerHandler for MediaServerProvider.
func (s *Server) mediaServerHandler(pt auth.ProviderType) (*auth.MediaServerProvider, bool) {
	p, ok := s.providerHandler(pt)
	if !ok {
		return nil, false
	}
	msp, ok := p.(*auth.MediaServerProvider)
	return msp, ok
}

// handleSetupLocal handles first admin setup via local credentials
func (s *Server) handleSetupLocal(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providerHandler(auth.ProviderLocal)
	if !ok {
		writeError(w, http.StatusInternalServerError, "local auth not available")
		return
	}
	lp, ok := p.(*auth.LocalProvider)
	if !ok {
		writeError(w, http.StatusInternalServerError, "local auth not available")
		return
	}
	lp.HandleSetup(w, r)
}

// handleSetupPlex handles first admin setup via Plex.tv
func (s *Server) handleSetupPlex(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providerHandler(auth.ProviderPlex)
	if !ok {
		writeError(w, http.StatusInternalServerError, "plex auth not available")
		return
	}
	pp, ok := p.(*auth.PlexProvider)
	if !ok {
		writeError(w, http.StatusInternalServerError, "plex auth not available")
		return
	}
	pp.HandleSetup(w, r)
}

// handleSetupEmby handles first admin setup via Emby credentials
func (s *Server) handleSetupEmby(w http.ResponseWriter, r *http.Request) {
	msp, ok := s.mediaServerHandler(auth.ProviderEmby)
	if !ok {
		writeError(w, http.StatusInternalServerError, "emby auth not available")
		return
	}
	msp.HandleSetup(w, r)
}

// handleSetupJellyfin handles first admin setup via Jellyfin credentials
func (s *Server) handleSetupJellyfin(w http.ResponseWriter, r *http.Request) {
	msp, ok := s.mediaServerHandler(auth.ProviderJellyfin)
	if !ok {
		writeError(w, http.StatusInternalServerError, "jellyfin auth not available")
		return
	}
	msp.HandleSetup(w, r)
}

// handleLocalLogin handles local username/password login
func (s *Server) handleLocalLogin(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providerHandler(auth.ProviderLocal)
	if !ok || !p.Enabled() {
		writeError(w, http.StatusNotFound, "local auth not available")
		return
	}
	p.HandleLogin(w, r)
}

// handlePlexLogin handles Plex.tv token-based login
func (s *Server) handlePlexLogin(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providerHandler(auth.ProviderPlex)
	if !ok || !p.Enabled() {
		writeError(w, http.StatusNotFound, "plex auth not available")
		return
	}
	p.HandleLogin(w, r)
}

// handleOIDCLogin initiates OIDC login flow
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providerHandler(auth.ProviderOIDC)
	if !ok || !p.Enabled() {
		writeError(w, http.StatusNotFound, "OIDC not configured")
		return
	}
	p.HandleLogin(w, r)
}

// handleOIDCCallback processes OIDC callback
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providerHandler(auth.ProviderOIDC)
	if !ok || !p.Enabled() {
		writeError(w, http.StatusNotFound, "OIDC not configured")
		return
	}
	p.HandleCallback(w, r)
}

// handleEmbyLogin handles Emby credential-based login
func (s *Server) handleEmbyLogin(w http.ResponseWriter, r *http.Request) {
	msp, ok := s.mediaServerHandler(auth.ProviderEmby)
	if !ok || !msp.Enabled() {
		writeError(w, http.StatusNotFound, "emby auth not available")
		return
	}
	msp.HandleLogin(w, r)
}

// handleJellyfinLogin handles Jellyfin credential-based login
func (s *Server) handleJellyfinLogin(w http.ResponseWriter, r *http.Request) {
	msp, ok := s.mediaServerHandler(auth.ProviderJellyfin)
	if !ok || !msp.Enabled() {
		writeError(w, http.StatusNotFound, "jellyfin auth not available")
		return
	}
	msp.HandleLogin(w, r)
}

// handleEmbyServers returns available Emby servers for login
func (s *Server) handleEmbyServers(w http.ResponseWriter, r *http.Request) {
	msp, ok := s.mediaServerHandler(auth.ProviderEmby)
	if !ok {
		writeError(w, http.StatusNotFound, "emby auth not available")
		return
	}
	msp.HandleGetServers(w, r)
}

// handleJellyfinServers returns available Jellyfin servers for login
func (s *Server) handleJellyfinServers(w http.ResponseWriter, r *http.Request) {
	msp, ok := s.mediaServerHandler(auth.ProviderJellyfin)
	if !ok {
		writeError(w, http.StatusNotFound, "jellyfin auth not available")
		return
	}
	msp.HandleGetServers(w, r)
}
