package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"streammon/internal/store"
)

const maxSettingsBody = 1 << 16

type integrationSettings struct {
	URL     string `json:"url"`
	APIKey  string `json:"api_key"`
	Enabled *bool  `json:"enabled,omitempty"`
}

type integrationTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type integrationClient interface {
	TestConnection(ctx context.Context) error
}

const integrationTimeout = 15 * time.Second

type integrationDeps struct {
	validateURL  func(string) error
	newClient    func(url, apiKey string) (integrationClient, error)
	getConfig    func() (store.IntegrationConfig, error)
	setConfig    func(store.IntegrationConfig) error
	deleteConfig func() error
	onUpdate     func() // optional: called after successful update
	onDelete     func() // optional: called after successful delete
}

func (s *Server) handleGetIntegrationSettings(d integrationDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := d.getConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}

		apiKey := ""
		if cfg.APIKey != "" {
			apiKey = maskedSecret
		}

		writeJSON(w, http.StatusOK, integrationSettings{
			URL:     cfg.URL,
			APIKey:  apiKey,
			Enabled: &cfg.Enabled,
		})
	}
}

func (s *Server) handleUpdateIntegrationSettings(d integrationDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxSettingsBody)
		var req integrationSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		if req.APIKey == maskedSecret {
			req.APIKey = ""
		}

		if req.URL != "" {
			if err := d.validateURL(req.URL); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		existing, err := d.getConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}

		if req.APIKey == "" {
			if req.URL != existing.URL {
				writeError(w, http.StatusBadRequest, "api_key is required when changing the URL")
				return
			}
			req.APIKey = existing.APIKey
		}

		enabled := existing.Enabled
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		storeCfg := store.IntegrationConfig{
			URL:     req.URL,
			APIKey:  req.APIKey,
			Enabled: enabled,
		}

		if err := d.setConfig(storeCfg); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}

		if d.onUpdate != nil {
			d.onUpdate()
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func (s *Server) handleDeleteIntegrationSettings(d integrationDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := d.deleteConfig(); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		if d.onDelete != nil {
			d.onDelete()
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleTestIntegrationConnection(d integrationDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxSettingsBody)
		var req integrationSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		if req.URL == "" {
			writeError(w, http.StatusBadRequest, "url is required")
			return
		}

		apiKey := req.APIKey
		if apiKey == "" || apiKey == maskedSecret {
			cfg, err := d.getConfig()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal")
				return
			}
			apiKey = cfg.APIKey
		}

		if apiKey == "" {
			writeError(w, http.StatusBadRequest, "api_key is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), integrationTimeout)
		defer cancel()

		client, err := d.newClient(req.URL, apiKey)
		if err != nil {
			writeJSON(w, http.StatusOK, integrationTestResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		if err := client.TestConnection(ctx); err != nil {
			writeJSON(w, http.StatusOK, integrationTestResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, integrationTestResponse{Success: true})
	}
}

func (s *Server) handleIntegrationConfigured(d integrationDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := d.getConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{
			"configured": cfg.URL != "" && cfg.APIKey != "" && cfg.Enabled,
		})
	}
}
