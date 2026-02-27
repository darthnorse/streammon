package auth

import (
	"slices"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"empty config is invalid when isSet", Config{Issuer: "https://example.com"}, true},
		{"missing client_id", Config{Issuer: "https://example.com", ClientSecret: "secret"}, true},
		{"missing redirect_url", Config{Issuer: "https://example.com", ClientID: "id", ClientSecret: "secret"}, true},
		{"http issuer rejected", Config{Issuer: "http://example.com", ClientID: "id", ClientSecret: "secret", RedirectURL: "https://example.com/callback"}, true},
		{"valid config", Config{Issuer: "https://example.com", ClientID: "id", ClientSecret: "secret", RedirectURL: "https://example.com/callback"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainsGroup(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		target string
		want   bool
	}{
		{"match", []string{"users", "admins"}, "admins", true},
		{"case insensitive", []string{"Users", "Admins"}, "admins", true},
		{"no match", []string{"users", "viewers"}, "admins", false},
		{"empty groups", nil, "admins", false},
		{"empty target", []string{"users"}, "", false},
		{"substring no match", []string{"admin"}, "admins", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsGroup(tt.groups, tt.target); got != tt.want {
				t.Errorf("containsGroup(%v, %q) = %v, want %v", tt.groups, tt.target, got, tt.want)
			}
		})
	}
}

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"default when empty", "", []string{"openid", "profile", "email", "groups"}},
		{"custom scopes", "openid,profile,email", []string{"openid", "profile", "email"}},
		{"trims whitespace", " openid , profile , email ", []string{"openid", "profile", "email"}},
		{"skips empty segments", "openid,,profile,", []string{"openid", "profile"}},
		{"auto-prepends openid", "profile,email", []string{"openid", "profile", "email"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseScopes(tt.raw)
			if !slices.Equal(got, tt.want) {
				t.Errorf("parseScopes(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestConfig_isSet(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"empty", Config{}, false},
		{"only issuer", Config{Issuer: "https://example.com"}, true},
		{"only client_id", Config{ClientID: "id"}, true},
		{"only client_secret", Config{ClientSecret: "secret"}, true},
		{"only redirect_url", Config{RedirectURL: "https://example.com/callback"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.isSet(); got != tt.want {
				t.Errorf("isSet() = %v, want %v", got, tt.want)
			}
		})
	}
}
