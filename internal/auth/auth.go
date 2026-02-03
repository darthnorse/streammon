package auth

import (
	"context"
	"errors"
	"sync"
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

func ConfigFromStore(sc store.OIDCConfig) Config {
	return Config{
		Issuer:       sc.Issuer,
		ClientID:     sc.ClientID,
		ClientSecret: sc.ClientSecret,
		RedirectURL:  sc.RedirectURL,
	}
}

// isSet returns true if any field is provided, so partial configs trigger validation errors.
func (c Config) isSet() bool {
	return c.Issuer != "" || c.ClientID != "" || c.ClientSecret != ""
}

func (c Config) Validate() error {
	if c.Issuer == "" || c.ClientID == "" || c.ClientSecret == "" {
		return errors.New("issuer, client ID, and client secret are all required")
	}
	if c.RedirectURL == "" {
		return errors.New("redirect URL is required")
	}
	return nil
}

const SessionDuration = 7 * 24 * time.Hour
const CookieName = "streammon_session"
const stateCookieName = "oidc_state"

type Service struct {
	mu       sync.RWMutex
	enabled  bool
	store    *store.Store
	provider *gooidc.Provider
	oauth2   oauth2.Config
	verifier *gooidc.IDTokenVerifier
}

type oidcProvider struct {
	provider *gooidc.Provider
	oauth2   oauth2.Config
	verifier *gooidc.IDTokenVerifier
}

func buildProvider(ctx context.Context, cfg Config) (*oidcProvider, error) {
	provider, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	return &oidcProvider{
		provider: provider,
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
		},
		verifier: provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func NewService(cfg Config, st *store.Store) (*Service, error) {
	if !cfg.isSet() {
		return &Service{enabled: false, store: st}, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	p, err := buildProvider(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	return &Service{
		enabled:  true,
		store:    st,
		provider: p.provider,
		oauth2:   p.oauth2,
		verifier: p.verifier,
	}, nil
}

func (s *Service) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *Service) Store() *store.Store {
	return s.store
}

func TestIssuer(ctx context.Context, issuer string) error {
	_, err := gooidc.NewProvider(ctx, issuer)
	return err
}

func (s *Service) Reload(ctx context.Context, cfg Config) error {
	if !cfg.isSet() {
		s.mu.Lock()
		s.enabled = false
		s.provider = nil
		s.oauth2 = oauth2.Config{}
		s.verifier = nil
		s.mu.Unlock()
		return nil
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	p, err := buildProvider(ctx, cfg)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.enabled = true
	s.provider = p.provider
	s.oauth2 = p.oauth2
	s.verifier = p.verifier
	s.mu.Unlock()

	return nil
}
