package auth

import (
	"net/http"
)

// ProviderType identifies the authentication provider
type ProviderType string

const (
	ProviderLocal ProviderType = "local"
	ProviderPlex  ProviderType = "plex"
	ProviderOIDC  ProviderType = "oidc"
)

// Provider defines the interface for authentication providers
type Provider interface {
	// Name returns the provider identifier
	Name() ProviderType

	// Enabled returns true if the provider is configured and ready
	Enabled() bool

	// HandleLogin initiates or processes the login flow
	// For local: processes username/password
	// For Plex: processes auth token from PIN flow
	// For OIDC: redirects to identity provider
	HandleLogin(w http.ResponseWriter, r *http.Request)

	// HandleCallback processes OAuth callback (no-op for local auth)
	HandleCallback(w http.ResponseWriter, r *http.Request)
}
