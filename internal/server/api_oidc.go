package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"streammon/internal/auth"
	"streammon/internal/store"
)

const maskedSecret = "********"

type oidcSettingsResponse struct {
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	Enabled      bool   `json:"enabled"`
}

type oidcSettingsRequest struct {
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
}

func (s *Server) handleGetOIDCSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetOIDCConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	secret := ""
	if cfg.ClientSecret != "" {
		secret = maskedSecret
	}

	enabled := false
	if s.authManager != nil {
		if p, ok := s.authManager.GetProvider(auth.ProviderOIDC); ok {
			enabled = p.Enabled()
		}
	}

	writeJSON(w, http.StatusOK, oidcSettingsResponse{
		Issuer:       cfg.Issuer,
		ClientID:     cfg.ClientID,
		ClientSecret: secret,
		RedirectURL:  cfg.RedirectURL,
		Enabled:      enabled,
	})
}

func (s *Server) handleUpdateOIDCSettings(w http.ResponseWriter, r *http.Request) {
	var req oidcSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.ClientSecret == maskedSecret {
		req.ClientSecret = ""
	}

	storeCfg := store.OIDCConfig{
		Issuer:       req.Issuer,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURL:  req.RedirectURL,
	}

	cfg := auth.ConfigFromStore(storeCfg)
	if cfg.Issuer != "" || cfg.ClientID != "" || cfg.ClientSecret != "" || cfg.RedirectURL != "" {
		if cfg.ClientSecret == "" {
			dbCfg, err := s.store.GetOIDCConfig()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal")
				return
			}
			cfg.ClientSecret = dbCfg.ClientSecret
		}
		if err := cfg.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := s.store.SetOIDCConfig(storeCfg); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if err := s.reloadAuth(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"warning": "saved but OIDC reload failed — verify the issuer is reachable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type oidcTestRequest struct {
	Issuer string `json:"issuer"`
}

type oidcTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleDeleteOIDCSettings(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteOIDCConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := s.reloadAuth(r.Context()); err != nil {
		log.Printf("reloading auth after OIDC delete: %v", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) reloadAuth(ctx context.Context) error {
	if s.authManager == nil {
		return nil
	}
	p, ok := s.authManager.GetProvider(auth.ProviderOIDC)
	if !ok {
		return nil
	}
	oidcProvider, ok := p.(*auth.OIDCProvider)
	if !ok {
		return nil
	}
	dbCfg, err := s.store.GetOIDCConfig()
	if err != nil {
		return err
	}
	reloadCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return oidcProvider.Reload(reloadCtx, auth.ConfigFromStore(dbCfg))
}

func (s *Server) handleTestOIDCConnection(w http.ResponseWriter, r *http.Request) {
	var req oidcTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Issuer == "" {
		writeError(w, http.StatusBadRequest, "issuer is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := auth.TestIssuer(ctx, req.Issuer); err != nil {
		writeJSON(w, http.StatusOK, oidcTestResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to discover OIDC provider at %s", req.Issuer),
		})
		return
	}

	writeJSON(w, http.StatusOK, oidcTestResponse{Success: true})
}
