package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/models"
)

func TestListRuleExemptions_Empty(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	rule := &models.Rule{
		Name: "Test", Type: models.RuleTypeConcurrentStreams,
		Enabled: true, Config: json.RawMessage(`{}`),
	}
	st.CreateRule(rule)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var names []string
	json.Unmarshal(w.Body.Bytes(), &names)
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

func TestSetRuleExemptions(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	rule := &models.Rule{
		Name: "Test", Type: models.RuleTypeConcurrentStreams,
		Enabled: true, Config: json.RawMessage(`{}`),
	}
	st.CreateRule(rule)

	// Set exemptions
	body := `["alice","bob"]`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify via GET
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var names []string
	json.Unmarshal(w.Body.Bytes(), &names)
	if len(names) != 2 {
		t.Fatalf("expected 2 exemptions, got %d", len(names))
	}
	if names[0] != "alice" || names[1] != "bob" {
		t.Errorf("expected [alice, bob], got %v", names)
	}
}

func TestSetRuleExemptions_Replace(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	rule := &models.Rule{
		Name: "Test", Type: models.RuleTypeConcurrentStreams,
		Enabled: true, Config: json.RawMessage(`{}`),
	}
	st.CreateRule(rule)

	// Set initial
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), strings.NewReader(`["alice"]`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Replace
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), strings.NewReader(`["charlie"]`))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var names []string
	json.Unmarshal(w.Body.Bytes(), &names)
	if len(names) != 1 || names[0] != "charlie" {
		t.Errorf("expected [charlie], got %v", names)
	}
}

func TestListRuleExemptions_NotFound(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/rules/999/exemptions", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListNotificationChannels_MasksSecrets(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	discord := &models.NotificationChannel{
		Name: "Discord", ChannelType: models.ChannelTypeDiscord, Enabled: true,
		Config: json.RawMessage(`{"webhook_url":"https://discord.com/api/webhooks/123/supersecrettoken"}`),
	}
	pushover := &models.NotificationChannel{
		Name: "Pushover", ChannelType: models.ChannelTypePushover, Enabled: true,
		Config: json.RawMessage(`{"user_key":"userkey123","api_token":"pushovertokensecret"}`),
	}
	ntfy := &models.NotificationChannel{
		Name: "Ntfy", ChannelType: models.ChannelTypeNtfy, Enabled: true,
		Config: json.RawMessage(`{"server_url":"https://ntfy.sh","topic":"alerts","token":"ntfytokensecret"}`),
	}
	webhook := &models.NotificationChannel{
		Name: "Webhook", ChannelType: models.ChannelTypeWebhook, Enabled: true,
		Config: json.RawMessage(`{"url":"https://example.com/hook","method":"POST","headers":{"Authorization":"Bearer webhooksecrettoken"}}`),
	}
	for _, c := range []*models.NotificationChannel{discord, pushover, ntfy, webhook} {
		if err := st.CreateNotificationChannel(c); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, secret := range []string{"supersecrettoken", "pushovertokensecret", "ntfytokensecret", "webhooksecrettoken"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response leaked secret %q: %s", secret, body)
		}
	}

	var channels []models.NotificationChannel
	if err := json.Unmarshal(w.Body.Bytes(), &channels); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(channels) != 4 {
		t.Fatalf("expected 4 channels, got %d", len(channels))
	}

	for _, c := range channels {
		switch c.ChannelType {
		case models.ChannelTypeDiscord:
			var cfg models.DiscordConfig
			json.Unmarshal(c.Config, &cfg)
			if cfg.WebhookURL != "********" {
				t.Errorf("discord webhook_url not masked: %q", cfg.WebhookURL)
			}
		case models.ChannelTypePushover:
			var cfg models.PushoverConfig
			json.Unmarshal(c.Config, &cfg)
			if cfg.APIToken != "********" {
				t.Errorf("pushover api_token not masked: %q", cfg.APIToken)
			}
			if cfg.UserKey != "userkey123" {
				t.Errorf("pushover user_key should not be masked, got %q", cfg.UserKey)
			}
		case models.ChannelTypeNtfy:
			var cfg models.NtfyConfig
			json.Unmarshal(c.Config, &cfg)
			if cfg.Token != "********" {
				t.Errorf("ntfy token not masked: %q", cfg.Token)
			}
		case models.ChannelTypeWebhook:
			var cfg models.WebhookConfig
			json.Unmarshal(c.Config, &cfg)
			if cfg.Headers["Authorization"] != "********" {
				t.Errorf("webhook auth header not masked: %q", cfg.Headers["Authorization"])
			}
		}
	}

	// Verify the stored config in the DB is still the real secret (masking
	// must not mutate what's persisted).
	stored, err := st.GetNotificationChannel(discord.ID)
	if err != nil {
		t.Fatal(err)
	}
	var storedCfg models.DiscordConfig
	json.Unmarshal(stored.Config, &storedCfg)
	if storedCfg.WebhookURL != "https://discord.com/api/webhooks/123/supersecrettoken" {
		t.Fatalf("stored secret was mutated by masking: %q", storedCfg.WebhookURL)
	}
}

func TestGetNotificationChannel_MasksSecret(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	channel := &models.NotificationChannel{
		Name: "Discord", ChannelType: models.ChannelTypeDiscord, Enabled: true,
		Config: json.RawMessage(`{"webhook_url":"https://discord.com/api/webhooks/123/supersecrettoken"}`),
	}
	if err := st.CreateNotificationChannel(channel); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/notifications/%d", channel.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "supersecrettoken") {
		t.Fatalf("response leaked secret: %s", w.Body.String())
	}
}

func TestUpdateNotificationChannel_MaskedValuePreservesSecret(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	channel := &models.NotificationChannel{
		Name: "Pushover", ChannelType: models.ChannelTypePushover, Enabled: true,
		Config: json.RawMessage(`{"user_key":"userkey123","api_token":"originalsecret"}`),
	}
	if err := st.CreateNotificationChannel(channel); err != nil {
		t.Fatal(err)
	}

	// Client changes the name/user_key but leaves api_token as the masked
	// placeholder returned by a prior GET.
	body := `{"name":"Pushover Renamed","channel_type":"pushover","enabled":true,"config":{"user_key":"userkey123","api_token":"********"}}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifications/%d", channel.ID), strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "originalsecret") {
		t.Fatalf("response leaked stored secret: %s", w.Body.String())
	}

	stored, err := st.GetNotificationChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	var cfg models.PushoverConfig
	json.Unmarshal(stored.Config, &cfg)
	if cfg.APIToken != "originalsecret" {
		t.Fatalf("expected preserved api_token, got %q", cfg.APIToken)
	}
	if stored.Name != "Pushover Renamed" {
		t.Fatalf("expected updated name, got %q", stored.Name)
	}
}

func TestUpdateNotificationChannel_NewSecretOverwritesStored(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	channel := &models.NotificationChannel{
		Name: "Pushover", ChannelType: models.ChannelTypePushover, Enabled: true,
		Config: json.RawMessage(`{"user_key":"userkey123","api_token":"originalsecret"}`),
	}
	if err := st.CreateNotificationChannel(channel); err != nil {
		t.Fatal(err)
	}

	body := `{"name":"Pushover","channel_type":"pushover","enabled":true,"config":{"user_key":"userkey123","api_token":"brandnewsecret"}}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/notifications/%d", channel.ID), strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	stored, err := st.GetNotificationChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	var cfg models.PushoverConfig
	json.Unmarshal(stored.Config, &cfg)
	if cfg.APIToken != "brandnewsecret" {
		t.Fatalf("expected new api_token to be saved, got %q", cfg.APIToken)
	}
}

func TestUpdateNotificationChannel_NotFound(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	body := `{"name":"Ghost","channel_type":"discord","enabled":true,"config":{"webhook_url":"https://discord.com/api/webhooks/1/a"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/notifications/999", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetRuleExemptions_InvalidJSON(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	rule := &models.Rule{
		Name: "Test", Type: models.RuleTypeConcurrentStreams,
		Enabled: true, Config: json.RawMessage(`{}`),
	}
	st.CreateRule(rule)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
