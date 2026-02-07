package auth

import (
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
