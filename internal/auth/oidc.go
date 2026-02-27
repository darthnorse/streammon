package auth

import (
	"context"
	"log"
	"net/http"
	"sync"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"streammon/internal/models"
	"streammon/internal/store"
)

type OIDCProvider struct {
	mu         sync.RWMutex
	enabled    bool
	store      *store.Store
	manager    *Manager
	provider   *gooidc.Provider
	oauth2     oauth2.Config
	verifier   *gooidc.IDTokenVerifier
	adminGroup string
}

func NewOIDCProvider(cfg Config, st *store.Store, mgr *Manager) (*OIDCProvider, error) {
	p := &OIDCProvider{
		store:   st,
		manager: mgr,
	}

	if cfg.isSet() {
		if err := p.Reload(context.Background(), cfg); err != nil {
			return nil, err
		}
	}

	return p, nil
}

func (p *OIDCProvider) Name() ProviderType {
	return ProviderOIDC
}

func (p *OIDCProvider) Enabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

func (p *OIDCProvider) Reload(ctx context.Context, cfg Config) error {
	if !cfg.isSet() {
		p.mu.Lock()
		p.enabled = false
		p.provider = nil
		p.oauth2 = oauth2.Config{}
		p.verifier = nil
		p.adminGroup = ""
		p.mu.Unlock()
		return nil
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	op, err := buildProvider(ctx, cfg)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.enabled = true
	p.provider = op.provider
	p.oauth2 = op.oauth2
	p.verifier = op.verifier
	p.adminGroup = cfg.AdminGroup
	p.mu.Unlock()

	return nil
}

func (p *OIDCProvider) getConfig() (bool, oauth2.Config, *gooidc.IDTokenVerifier, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled, p.oauth2, p.verifier, p.adminGroup
}

func (p *OIDCProvider) HandleLogin(w http.ResponseWriter, r *http.Request) {
	enabled, oauth2Cfg, _, _ := p.getConfig()
	if !enabled {
		http.NotFound(w, r)
		return
	}

	state, err := generateState()
	if err != nil {
		log.Printf("failed to generate OIDC state: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, makeCookie(stateCookieName, state, "/", 300, r))
	http.Redirect(w, r, oauth2Cfg.AuthCodeURL(state), http.StatusFound)
}

func (p *OIDCProvider) HandleCallback(w http.ResponseWriter, r *http.Request) {
	enabled, oauth2Cfg, verifier, adminGroup := p.getConfig()
	if !enabled {
		http.NotFound(w, r)
		return
	}

	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	token, err := oauth2Cfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		log.Printf("login failed: oidc (token exchange error: %v)", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "missing id_token", http.StatusUnauthorized)
		return
	}

	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		log.Printf("login failed: oidc (token verify error: %v)", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	var claims struct {
		Email         string   `json:"email"`
		EmailVerified bool     `json:"email_verified"`
		Name          string   `json:"name"`
		Sub           string   `json:"sub"`
		Groups        []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "invalid claims", http.StatusUnauthorized)
		return
	}

	name := firstNonEmpty(claims.Name, claims.Email, claims.Sub)

	// Only link by email if verified by the IdP (prevents account hijacking)
	emailForLinking := ""
	if claims.EmailVerified {
		emailForLinking = claims.Email
	}

	groupAdmin := adminGroup != "" && containsGroup(claims.Groups, adminGroup)

	// Check guest access before creating/linking accounts to avoid side effects
	guestAccess, _ := p.store.GetGuestAccess()
	if !guestAccess && !groupAdmin && !p.isExistingAdmin(claims.Sub, emailForLinking) {
		log.Printf("login denied: oidc sub=%q (guest access disabled)", claims.Sub)
		http.SetCookie(w, clearCookie(stateCookieName, "/", r))
		http.Redirect(w, r, "/login?error=guest_access_disabled", http.StatusFound)
		return
	}

	user, err := p.store.GetOrLinkUserByEmail(
		emailForLinking,
		name,
		string(ProviderOIDC),
		claims.Sub,
		"",
	)
	if err != nil {
		log.Printf("user creation error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Sync role from OIDC group membership on every login.
	// nil means the groups claim was absent from the ID token — skip sync to avoid
	// mass demotion when the IdP doesn't include groups. An empty slice means
	// the claim was present but the user has no groups — demote as expected.
	if adminGroup != "" {
		if claims.Groups == nil {
			log.Printf("oidc role sync: groups claim not present in ID token for user %q; skipping role sync", name)
		} else {
			desiredRole := models.RoleViewer
			if groupAdmin {
				desiredRole = models.RoleAdmin
			}
			if user.Role != desiredRole {
				if err := p.store.UpdateUserRoleByIDSafe(user.ID, desiredRole); err != nil {
					log.Printf("oidc role sync: could not update user %q to %s: %v", user.Name, desiredRole, err)
				} else {
					log.Printf("oidc role sync: user %q role changed %s -> %s", user.Name, user.Role, desiredRole)
					user.Role = desiredRole
				}
			}
		}
	}

	if err := p.manager.CreateSession(w, r, user.ID); err != nil {
		log.Printf("session creation error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	log.Printf("login success: oidc user=%q role=%s", user.Name, user.Role)

	http.SetCookie(w, clearCookie(stateCookieName, "/", r))
	http.Redirect(w, r, "/", http.StatusFound)
}

func (p *OIDCProvider) isExistingAdmin(sub, email string) bool {
	if existing, err := p.store.GetUserByProvider(string(ProviderOIDC), sub); err == nil {
		return existing.Role == models.RoleAdmin
	}
	if email != "" {
		if emailUser, err := p.store.GetUserByEmail(email); err == nil {
			return emailUser.Role == models.RoleAdmin
		}
	}
	return false
}
