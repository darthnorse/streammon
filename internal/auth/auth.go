package auth

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"streammon/internal/store"
)

const DefaultScopes = "openid,profile,email,groups"

type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AdminGroup   string
	Scopes       string
}

func ConfigFromStore(sc store.OIDCConfig) Config {
	return Config{
		Issuer:       sc.Issuer,
		ClientID:     sc.ClientID,
		ClientSecret: sc.ClientSecret,
		RedirectURL:  sc.RedirectURL,
		AdminGroup:   sc.AdminGroup,
		Scopes:       sc.Scopes,
	}
}

func (c Config) isSet() bool {
	return c.Issuer != "" || c.ClientID != "" || c.ClientSecret != ""
}

func (c Config) Validate() error {
	if c.Issuer == "" || c.ClientID == "" || c.ClientSecret == "" {
		return errors.New("issuer, client ID, and client secret are all required")
	}
	if err := validateIssuerURL(c.Issuer); err != nil {
		return err
	}
	if c.RedirectURL == "" {
		return errors.New("redirect URL is required")
	}
	return nil
}

const SessionDuration = 7 * 24 * time.Hour
const CookieName = "streammon_session"
const stateCookieName = "oidc_state"

type oidcProvider struct {
	provider *gooidc.Provider
	oauth2   oauth2.Config
	verifier *gooidc.IDTokenVerifier
}

func buildProvider(ctx context.Context, cfg Config) (*oidcProvider, error) {
	if err := validateIssuerURL(cfg.Issuer); err != nil {
		return nil, err
	}
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
			Scopes:       parseScopes(cfg.Scopes),
		},
		verifier: provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func parseScopes(raw string) []string {
	if raw == "" {
		raw = DefaultScopes
	}
	var scopes []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			scopes = append(scopes, s)
		}
	}
	if !slices.Contains(scopes, "openid") {
		scopes = append([]string{"openid"}, scopes...)
	}
	return scopes
}

func containsGroup(groups []string, target string) bool {
	for _, g := range groups {
		if strings.EqualFold(g, target) {
			return true
		}
	}
	return false
}

func TestIssuer(ctx context.Context, issuer string) error {
	if err := validateIssuerURL(issuer); err != nil {
		return err
	}
	_, err := gooidc.NewProvider(ctx, issuer)
	return err
}

func validateIssuerURL(issuer string) error {
	u, err := url.Parse(issuer)
	if err != nil || u.Host == "" {
		return errors.New("issuer must be a valid URL")
	}
	if u.Scheme != "https" {
		return errors.New("issuer must use https://")
	}
	return nil
}
