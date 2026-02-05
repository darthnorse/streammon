package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"streammon/internal/models"
)

type Notifier struct {
	client *http.Client
}

func New() *Notifier {
	return &Notifier{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (n *Notifier) Notify(ctx context.Context, violation *models.RuleViolation, channels []models.NotificationChannel) error {
	if len(channels) == 0 {
		return nil
	}

	// Send to all channels in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	for _, ch := range channels {
		wg.Add(1)
		go func(ch models.NotificationChannel) {
			defer wg.Done()

			var err error
			switch ch.ChannelType {
			case models.ChannelTypeDiscord:
				err = n.sendDiscord(ctx, ch, violation)
			case models.ChannelTypeWebhook:
				err = n.sendWebhook(ctx, ch, violation)
			case models.ChannelTypePushover:
				err = n.sendPushover(ctx, ch, violation)
			case models.ChannelTypeNtfy:
				err = n.sendNtfy(ctx, ch, violation)
			default:
				err = fmt.Errorf("unknown channel type: %s", ch.ChannelType)
			}
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: %v", ch.Name, err))
				mu.Unlock()
			}
		}(ch)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (n *Notifier) sendDiscord(ctx context.Context, ch models.NotificationChannel, v *models.RuleViolation) error {
	var config models.DiscordConfig
	if err := json.Unmarshal(ch.Config, &config); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return err
	}

	color := 0x808080
	switch v.Severity {
	case models.SeverityCritical:
		color = 0xFF0000
	case models.SeverityWarning:
		color = 0xFFA500
	case models.SeverityInfo:
		color = 0x0000FF
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("Rule Violation: %s", v.RuleName),
				"description": v.Message,
				"color":       color,
				"fields": []map[string]interface{}{
					{"name": "User", "value": v.UserName, "inline": true},
					{"name": "Severity", "value": string(v.Severity), "inline": true},
					{"name": "Confidence", "value": fmt.Sprintf("%.0f%%", v.ConfidenceScore), "inline": true},
				},
				"timestamp": v.OccurredAt.Format(time.RFC3339),
				"footer": map[string]string{
					"text": "StreamMon Rules Engine",
				},
			},
		},
	}

	return n.postJSON(ctx, config.WebhookURL, payload)
}

func (n *Notifier) sendWebhook(ctx context.Context, ch models.NotificationChannel, v *models.RuleViolation) error {
	var config models.WebhookConfig
	if err := json.Unmarshal(ch.Config, &config); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"event":            "rule_violation",
		"rule_id":          v.RuleID,
		"rule_name":        v.RuleName,
		"user_name":        v.UserName,
		"severity":         v.Severity,
		"message":          v.Message,
		"confidence_score": v.ConfidenceScore,
		"details":          v.Details,
		"occurred_at":      v.OccurredAt.Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, val := range config.Headers {
		req.Header.Set(k, val)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) sendPushover(ctx context.Context, ch models.NotificationChannel, v *models.RuleViolation) error {
	var config models.PushoverConfig
	if err := json.Unmarshal(ch.Config, &config); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return err
	}

	priority := "0"
	switch v.Severity {
	case models.SeverityCritical:
		priority = "1"
	case models.SeverityInfo:
		priority = "-1"
	}

	form := url.Values{}
	form.Set("token", config.APIToken)
	form.Set("user", config.UserKey)
	form.Set("title", fmt.Sprintf("StreamMon: %s", v.RuleName))
	form.Set("message", fmt.Sprintf("%s\n\nUser: %s\nConfidence: %.0f%%", v.Message, v.UserName, v.ConfidenceScore))
	form.Set("priority", priority)
	form.Set("timestamp", fmt.Sprintf("%d", v.OccurredAt.Unix()))

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.pushover.net/1/messages.json",
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("pushover returned status %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) sendNtfy(ctx context.Context, ch models.NotificationChannel, v *models.RuleViolation) error {
	var config models.NtfyConfig
	if err := json.Unmarshal(ch.Config, &config); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return err
	}

	ntfyURL := strings.TrimRight(config.ServerURL, "/") + "/" + config.Topic

	priority := "default"
	switch v.Severity {
	case models.SeverityCritical:
		priority = "urgent"
	case models.SeverityWarning:
		priority = "high"
	case models.SeverityInfo:
		priority = "low"
	}

	message := fmt.Sprintf("%s\n\nUser: %s\nConfidence: %.0f%%", v.Message, v.UserName, v.ConfidenceScore)

	req, err := http.NewRequestWithContext(ctx, "POST", ntfyURL, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Title", fmt.Sprintf("StreamMon: %s", v.RuleName))
	req.Header.Set("Priority", priority)
	req.Header.Set("Tags", string(v.Severity))

	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) postJSON(ctx context.Context, url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) TestChannel(ctx context.Context, ch *models.NotificationChannel) error {
	testViolation := &models.RuleViolation{
		RuleID:          0,
		RuleName:        "Test Rule",
		UserName:        "test_user",
		Severity:        models.SeverityInfo,
		Message:         "This is a test notification from StreamMon",
		ConfidenceScore: 100,
		OccurredAt:      time.Now().UTC(),
	}

	return n.Notify(ctx, testViolation, []models.NotificationChannel{*ch})
}
