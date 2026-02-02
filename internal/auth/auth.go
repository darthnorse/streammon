package auth

import (
	"context"
	"errors"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"streammon/internal/store"
)

type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func (c Config) isSet() bool {
	return c.Issuer != "" || c.ClientID != "" || c.ClientSecret != ""
}

func (c Config) validate() error {
	if c.Issuer == "" || c.ClientID == "" || c.ClientSecret == "" {
		return errors.New("OIDC_ISSUER, OIDC_CLIENT_ID, and OIDC_CLIENT_SECRET must all be set")
	}
	if c.RedirectURL == "" {
		return errors.New("OIDC_REDIRECT_URL is required")
	}
	return nil
}

const SessionDuration = 7 * 24 * time.Hour
const CookieName = "streammon_session"

type Service struct {
	enabled  bool
	store    *store.Store
	provider *gooidc.Provider
	oauth2   oauth2.Config
	verifier *gooidc.IDTokenVerifier
}

func NewService(cfg Config, st *store.Store) (*Service, error) {
	if !cfg.isSet() {
		return &Service{enabled: false, store: st}, nil
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	ctx := context.Background()
	provider, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID})

	return &Service{
		enabled:  true,
		store:    st,
		provider: provider,
		oauth2:   oauth2Cfg,
		verifier: verifier,
	}, nil
}

func (s *Service) Enabled() bool {
	return s.enabled
}

func (s *Service) Store() *store.Store {
	return s.store
}
