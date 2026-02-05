package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestNotifier_SendDiscord(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := New()
	ctx := context.Background()

	channel := models.NotificationChannel{
		Name:        "Test Discord",
		ChannelType: models.ChannelTypeDiscord,
		Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
		Enabled:     true,
	}

	violation := &models.RuleViolation{
		RuleID:          1,
		RuleName:        "Test Rule",
		UserName:        "testuser",
		Severity:        models.SeverityWarning,
		Message:         "Test violation message",
		ConfidenceScore: 85.5,
		OccurredAt:      time.Now().UTC(),
	}

	err := n.Notify(ctx, violation, []models.NotificationChannel{channel})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	embeds, ok := receivedBody["embeds"].([]interface{})
	if !ok || len(embeds) == 0 {
		t.Fatal("expected embeds in Discord payload")
	}
	embed := embeds[0].(map[string]interface{})
	if embed["title"] != "Rule Violation: Test Rule" {
		t.Errorf("title = %q, want 'Rule Violation: Test Rule'", embed["title"])
	}
}

func TestNotifier_SendWebhook(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := New()
	ctx := context.Background()

	channel := models.NotificationChannel{
		Name:        "Test Webhook",
		ChannelType: models.ChannelTypeWebhook,
		Config:      json.RawMessage(`{"url":"` + server.URL + `","method":"POST","headers":{"X-Custom":"test123"}}`),
		Enabled:     true,
	}

	violation := &models.RuleViolation{
		RuleID:          1,
		RuleName:        "Test Rule",
		UserName:        "testuser",
		Severity:        models.SeverityCritical,
		Message:         "Test violation",
		ConfidenceScore: 95,
		Details:         map[string]interface{}{"key": "value"},
		OccurredAt:      time.Now().UTC(),
	}

	err := n.Notify(ctx, violation, []models.NotificationChannel{channel})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if receivedBody["event"] != "rule_violation" {
		t.Errorf("event = %v, want rule_violation", receivedBody["event"])
	}
	if receivedBody["user_name"] != "testuser" {
		t.Errorf("user_name = %v, want testuser", receivedBody["user_name"])
	}
	if receivedHeaders.Get("X-Custom") != "test123" {
		t.Errorf("X-Custom header = %q, want test123", receivedHeaders.Get("X-Custom"))
	}
}

func TestNotifier_SendNtfy(t *testing.T) {
	var receivedBody string
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := New()
	ctx := context.Background()

	channel := models.NotificationChannel{
		Name:        "Test Ntfy",
		ChannelType: models.ChannelTypeNtfy,
		Config:      json.RawMessage(`{"server_url":"` + server.URL + `","topic":"test-topic","token":"secret123"}`),
		Enabled:     true,
	}

	violation := &models.RuleViolation{
		RuleID:          1,
		RuleName:        "Test Rule",
		UserName:        "testuser",
		Severity:        models.SeverityCritical,
		Message:         "Critical violation",
		ConfidenceScore: 100,
		OccurredAt:      time.Now().UTC(),
	}

	err := n.Notify(ctx, violation, []models.NotificationChannel{channel})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if receivedHeaders.Get("Title") != "StreamMon: Test Rule" {
		t.Errorf("Title header = %q, want 'StreamMon: Test Rule'", receivedHeaders.Get("Title"))
	}
	if receivedHeaders.Get("Priority") != "urgent" {
		t.Errorf("Priority = %q, want urgent", receivedHeaders.Get("Priority"))
	}
	if receivedHeaders.Get("Authorization") != "Bearer secret123" {
		t.Errorf("Authorization = %q, want 'Bearer secret123'", receivedHeaders.Get("Authorization"))
	}
	if receivedBody == "" {
		t.Error("expected non-empty body")
	}
}

func TestNotifier_MultipleChannels(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := New()
	ctx := context.Background()

	channels := []models.NotificationChannel{
		{Name: "Discord", ChannelType: models.ChannelTypeDiscord, Config: json.RawMessage(`{"webhook_url":"` + server.URL + `"}`)},
		{Name: "Webhook", ChannelType: models.ChannelTypeWebhook, Config: json.RawMessage(`{"url":"` + server.URL + `"}`)},
	}

	violation := &models.RuleViolation{
		RuleID:          1,
		RuleName:        "Test",
		UserName:        "test",
		Severity:        models.SeverityInfo,
		Message:         "Test",
		ConfidenceScore: 50,
		OccurredAt:      time.Now().UTC(),
	}

	err := n.Notify(ctx, violation, channels)
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestNotifier_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := New()
	ctx := context.Background()

	channel := models.NotificationChannel{
		Name:        "Failing",
		ChannelType: models.ChannelTypeDiscord,
		Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
	}

	violation := &models.RuleViolation{
		RuleID:          1,
		RuleName:        "Test",
		UserName:        "test",
		Severity:        models.SeverityInfo,
		Message:         "Test",
		ConfidenceScore: 50,
		OccurredAt:      time.Now().UTC(),
	}

	err := n.Notify(ctx, violation, []models.NotificationChannel{channel})
	if err == nil {
		t.Error("expected error for failing webhook")
	}
}

func TestNotifier_PartialFailure(t *testing.T) {
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer goodServer.Close()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badServer.Close()

	n := New()
	ctx := context.Background()

	channels := []models.NotificationChannel{
		{Name: "Good", ChannelType: models.ChannelTypeDiscord, Config: json.RawMessage(`{"webhook_url":"` + goodServer.URL + `"}`)},
		{Name: "Bad", ChannelType: models.ChannelTypeDiscord, Config: json.RawMessage(`{"webhook_url":"` + badServer.URL + `"}`)},
	}

	violation := &models.RuleViolation{
		RuleID:          1,
		RuleName:        "Test",
		UserName:        "test",
		Severity:        models.SeverityInfo,
		Message:         "Test",
		ConfidenceScore: 50,
		OccurredAt:      time.Now().UTC(),
	}

	err := n.Notify(ctx, violation, channels)
	if err == nil {
		t.Error("expected error for partial failure")
	}
}

func TestNotifier_TestChannel(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := New()
	ctx := context.Background()

	channel := &models.NotificationChannel{
		Name:        "Test Channel",
		ChannelType: models.ChannelTypeDiscord,
		Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
	}

	err := n.TestChannel(ctx, channel)
	if err != nil {
		t.Fatalf("TestChannel: %v", err)
	}

	embeds, ok := receivedBody["embeds"].([]interface{})
	if !ok || len(embeds) == 0 {
		t.Fatal("expected embeds")
	}
	embed := embeds[0].(map[string]interface{})
	if embed["title"] != "Rule Violation: Test Rule" {
		t.Error("expected test rule name")
	}
}

func TestNotifier_InvalidConfig(t *testing.T) {
	n := New()
	ctx := context.Background()

	channel := models.NotificationChannel{
		Name:        "Bad Config",
		ChannelType: models.ChannelTypeDiscord,
		Config:      json.RawMessage(`{"invalid`),
	}

	violation := &models.RuleViolation{
		RuleID:          1,
		RuleName:        "Test",
		UserName:        "test",
		Severity:        models.SeverityInfo,
		Message:         "Test",
		ConfidenceScore: 50,
		OccurredAt:      time.Now().UTC(),
	}

	err := n.Notify(ctx, violation, []models.NotificationChannel{channel})
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestNotifier_SeverityColors(t *testing.T) {
	tests := []struct {
		severity models.Severity
		wantColor int
	}{
		{models.SeverityCritical, 0xFF0000},
		{models.SeverityWarning, 0xFFA500},
		{models.SeverityInfo, 0x0000FF},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			var receivedBody map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &receivedBody)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			n := New()
			ctx := context.Background()

			channel := models.NotificationChannel{
				ChannelType: models.ChannelTypeDiscord,
				Config:      json.RawMessage(`{"webhook_url":"` + server.URL + `"}`),
			}

			violation := &models.RuleViolation{
				RuleID:          1,
				RuleName:        "Test",
				UserName:        "test",
				Severity:        tt.severity,
				Message:         "Test",
				ConfidenceScore: 50,
				OccurredAt:      time.Now().UTC(),
			}

			n.Notify(ctx, violation, []models.NotificationChannel{channel})

			embeds := receivedBody["embeds"].([]interface{})
			embed := embeds[0].(map[string]interface{})
			gotColor := int(embed["color"].(float64))
			if gotColor != tt.wantColor {
				t.Errorf("color = %x, want %x", gotColor, tt.wantColor)
			}
		})
	}
}
