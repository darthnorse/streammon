package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRuleTypeValid(t *testing.T) {
	tests := []struct {
		rt    RuleType
		valid bool
	}{
		{RuleTypeImpossibleTravel, true},
		{RuleTypeConcurrentStreams, true},
		{RuleTypeSimultaneousLocs, true},
		{RuleTypeDeviceVelocity, true},
		{RuleTypeGeoRestriction, true},
		{RuleTypeNewDevice, true},
		{RuleTypeNewLocation, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.rt.Valid(); got != tt.valid {
			t.Errorf("RuleType(%q).Valid() = %v, want %v", tt.rt, got, tt.valid)
		}
	}
}

func TestRuleTypeIsRealTime(t *testing.T) {
	tests := []struct {
		rt       RuleType
		realtime bool
	}{
		{RuleTypeConcurrentStreams, true},
		{RuleTypeSimultaneousLocs, true},
		{RuleTypeGeoRestriction, true},
		{RuleTypeNewDevice, true},
		{RuleTypeNewLocation, true},
		{RuleTypeImpossibleTravel, true},
		{RuleTypeDeviceVelocity, true},
		// Invalid/unknown rule types return false (for future batch-only rules)
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.rt.IsRealTime(); got != tt.realtime {
			t.Errorf("RuleType(%q).IsRealTime() = %v, want %v", tt.rt, got, tt.realtime)
		}
	}
}

func TestSeverityValid(t *testing.T) {
	tests := []struct {
		s     Severity
		valid bool
	}{
		{SeverityInfo, true},
		{SeverityWarning, true},
		{SeverityCritical, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.s.Valid(); got != tt.valid {
			t.Errorf("Severity(%q).Valid() = %v, want %v", tt.s, got, tt.valid)
		}
	}
}

func TestChannelTypeValid(t *testing.T) {
	tests := []struct {
		ct    ChannelType
		valid bool
	}{
		{ChannelTypeDiscord, true},
		{ChannelTypeWebhook, true},
		{ChannelTypePushover, true},
		{ChannelTypeNtfy, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.ct.Valid(); got != tt.valid {
			t.Errorf("ChannelType(%q).Valid() = %v, want %v", tt.ct, got, tt.valid)
		}
	}
}

func TestRuleValidate(t *testing.T) {
	tests := []struct {
		name    string
		rule    Rule
		wantErr bool
	}{
		{
			name: "valid rule",
			rule: Rule{
				Name:   "Test Rule",
				Type:   RuleTypeConcurrentStreams,
				Config: json.RawMessage(`{"max_streams": 2}`),
			},
			wantErr: false,
		},
		{
			name: "missing name",
			rule: Rule{
				Type:   RuleTypeConcurrentStreams,
				Config: json.RawMessage(`{}`),
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			rule: Rule{
				Name:   "Test",
				Type:   "invalid",
				Config: json.RawMessage(`{}`),
			},
			wantErr: true,
		},
		{
			name: "empty config gets default",
			rule: Rule{
				Name: "Test",
				Type: RuleTypeConcurrentStreams,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Rule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleViolationValidate(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name      string
		violation RuleViolation
		wantErr   bool
	}{
		{
			name: "valid violation",
			violation: RuleViolation{
				RuleID:          1,
				UserName:        "testuser",
				Severity:        SeverityWarning,
				Message:         "Test violation",
				ConfidenceScore: 85.5,
				OccurredAt:      now,
			},
			wantErr: false,
		},
		{
			name: "missing rule_id",
			violation: RuleViolation{
				UserName:   "testuser",
				Severity:   SeverityWarning,
				Message:    "Test",
				OccurredAt: now,
			},
			wantErr: true,
		},
		{
			name: "missing user_name",
			violation: RuleViolation{
				RuleID:     1,
				Severity:   SeverityWarning,
				Message:    "Test",
				OccurredAt: now,
			},
			wantErr: true,
		},
		{
			name: "invalid severity",
			violation: RuleViolation{
				RuleID:     1,
				UserName:   "testuser",
				Severity:   "invalid",
				Message:    "Test",
				OccurredAt: now,
			},
			wantErr: true,
		},
		{
			name: "missing message",
			violation: RuleViolation{
				RuleID:     1,
				UserName:   "testuser",
				Severity:   SeverityWarning,
				OccurredAt: now,
			},
			wantErr: true,
		},
		{
			name: "confidence score too high",
			violation: RuleViolation{
				RuleID:          1,
				UserName:        "testuser",
				Severity:        SeverityWarning,
				Message:         "Test",
				ConfidenceScore: 150,
				OccurredAt:      now,
			},
			wantErr: true,
		},
		{
			name: "confidence score negative",
			violation: RuleViolation{
				RuleID:          1,
				UserName:        "testuser",
				Severity:        SeverityWarning,
				Message:         "Test",
				ConfidenceScore: -10,
				OccurredAt:      now,
			},
			wantErr: true,
		},
		{
			name: "zero occurred_at gets default",
			violation: RuleViolation{
				RuleID:          1,
				UserName:        "testuser",
				Severity:        SeverityWarning,
				Message:         "Test",
				ConfidenceScore: 50,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.violation.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RuleViolation.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHouseholdLocationValidate(t *testing.T) {
	tests := []struct {
		name    string
		loc     HouseholdLocation
		wantErr bool
	}{
		{
			name: "valid with IP",
			loc: HouseholdLocation{
				UserName:  "testuser",
				IPAddress: "192.168.1.1",
			},
			wantErr: false,
		},
		{
			name: "valid with city/country",
			loc: HouseholdLocation{
				UserName: "testuser",
				City:     "New York",
				Country:  "US",
			},
			wantErr: false,
		},
		{
			name: "missing user_name",
			loc: HouseholdLocation{
				IPAddress: "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "no location data",
			loc: HouseholdLocation{
				UserName: "testuser",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.loc.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("HouseholdLocation.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotificationChannelValidate(t *testing.T) {
	tests := []struct {
		name    string
		channel NotificationChannel
		wantErr bool
	}{
		{
			name: "valid discord",
			channel: NotificationChannel{
				Name:        "My Discord",
				ChannelType: ChannelTypeDiscord,
				Config:      json.RawMessage(`{"webhook_url": "https://discord.com/api/webhooks/123"}`),
			},
			wantErr: false,
		},
		{
			name: "missing name",
			channel: NotificationChannel{
				ChannelType: ChannelTypeDiscord,
				Config:      json.RawMessage(`{"webhook_url": "https://discord.com/api/webhooks/123"}`),
			},
			wantErr: true,
		},
		{
			name: "invalid channel type",
			channel: NotificationChannel{
				Name:        "Test",
				ChannelType: "invalid",
				Config:      json.RawMessage(`{}`),
			},
			wantErr: true,
		},
		{
			name: "missing config",
			channel: NotificationChannel{
				Name:        "Test",
				ChannelType: ChannelTypeDiscord,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.channel.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationChannel.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	t.Run("ImpossibleTravelConfig defaults", func(t *testing.T) {
		c := &ImpossibleTravelConfig{}
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.MaxSpeedKmH != 800 {
			t.Errorf("MaxSpeedKmH = %v, want 800", c.MaxSpeedKmH)
		}
		if c.MinDistanceKm != 100 {
			t.Errorf("MinDistanceKm = %v, want 100", c.MinDistanceKm)
		}
		if c.TimeWindowHours != 24 {
			t.Errorf("TimeWindowHours = %v, want 24", c.TimeWindowHours)
		}
	})

	t.Run("ConcurrentStreamsConfig defaults", func(t *testing.T) {
		c := &ConcurrentStreamsConfig{}
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.MaxStreams != 2 {
			t.Errorf("MaxStreams = %v, want 2", c.MaxStreams)
		}
	})

	t.Run("DiscordConfig validation", func(t *testing.T) {
		c := &DiscordConfig{}
		if err := c.Validate(); err == nil {
			t.Error("expected error for empty webhook_url")
		}
		c.WebhookURL = "https://discord.com/api/webhooks/123"
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("PushoverConfig validation", func(t *testing.T) {
		c := &PushoverConfig{}
		if err := c.Validate(); err == nil {
			t.Error("expected error for empty config")
		}
		c.UserKey = "user123"
		if err := c.Validate(); err == nil {
			t.Error("expected error for missing api_token")
		}
		c.APIToken = "token123"
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("NtfyConfig defaults", func(t *testing.T) {
		c := &NtfyConfig{Topic: "test"}
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.ServerURL != "https://ntfy.sh" {
			t.Errorf("ServerURL = %v, want https://ntfy.sh", c.ServerURL)
		}
	})

	t.Run("WebhookConfig validation", func(t *testing.T) {
		c := &WebhookConfig{}
		if err := c.Validate(); err == nil {
			t.Error("expected error for empty url")
		}
		c.URL = "not-a-valid-url"
		if err := c.Validate(); err == nil {
			t.Error("expected error for invalid url format")
		}
		c.URL = "ftp://example.com/webhook"
		if err := c.Validate(); err == nil {
			t.Error("expected error for non-http scheme")
		}
		c.URL = "https://example.com/webhook"
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.Method != "POST" {
			t.Errorf("Method = %v, want POST", c.Method)
		}
	})

	t.Run("SimultaneousLocsConfig defaults", func(t *testing.T) {
		c := &SimultaneousLocsConfig{}
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.MinDistanceKm != 50 {
			t.Errorf("MinDistanceKm = %v, want 50", c.MinDistanceKm)
		}
	})

	t.Run("DeviceVelocityConfig defaults", func(t *testing.T) {
		c := &DeviceVelocityConfig{}
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.MaxDevicesPerHour != 3 {
			t.Errorf("MaxDevicesPerHour = %v, want 3", c.MaxDevicesPerHour)
		}
		if c.TimeWindowHours != 1 {
			t.Errorf("TimeWindowHours = %v, want 1", c.TimeWindowHours)
		}
	})

	t.Run("GeoRestrictionConfig validation", func(t *testing.T) {
		c := &GeoRestrictionConfig{}
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Empty allowed/blocked countries is valid (means no restrictions)
	})

	t.Run("NewDeviceConfig validation", func(t *testing.T) {
		c := &NewDeviceConfig{}
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// NotifyOnNew defaults to false (zero value)
		if c.NotifyOnNew {
			t.Errorf("NotifyOnNew = %v, want false", c.NotifyOnNew)
		}
	})

	t.Run("NewLocationConfig defaults", func(t *testing.T) {
		c := &NewLocationConfig{}
		if err := c.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if c.MinDistanceKm != 50 {
			t.Errorf("MinDistanceKm = %v, want 50", c.MinDistanceKm)
		}
		if c.SeverityThresholdKm != 500 {
			t.Errorf("SeverityThresholdKm = %v, want 500", c.SeverityThresholdKm)
		}
	})
}

func TestCalculateConfidence(t *testing.T) {
	tests := []struct {
		name    string
		signals []ViolationSignal
		want    float64
	}{
		{
			name:    "empty signals",
			signals: nil,
			want:    0,
		},
		{
			name: "single signal",
			signals: []ViolationSignal{
				{Name: "test", Weight: 1.0, Value: 80.0},
			},
			want: 80,
		},
		{
			name: "weighted average",
			signals: []ViolationSignal{
				{Name: "a", Weight: 0.3, Value: 100.0},
				{Name: "b", Weight: 0.7, Value: 50.0},
			},
			want: 65,
		},
		{
			name: "bool values",
			signals: []ViolationSignal{
				{Name: "a", Weight: 1.0, Value: true},
			},
			want: 100,
		},
		{
			name: "mixed values",
			signals: []ViolationSignal{
				{Name: "a", Weight: 0.5, Value: 80.0},
				{Name: "b", Weight: 0.5, Value: true},
			},
			want: 90,
		},
		{
			name: "capped at 100",
			signals: []ViolationSignal{
				{Name: "a", Weight: 1.0, Value: 150.0},
			},
			want: 100,
		},
		{
			name: "bool false contributes 0",
			signals: []ViolationSignal{
				{Name: "a", Weight: 0.5, Value: true},
				{Name: "b", Weight: 0.5, Value: false},
			},
			want: 50, // true=100, false=0, average=50
		},
		{
			name: "int values",
			signals: []ViolationSignal{
				{Name: "a", Weight: 1.0, Value: 75},
			},
			want: 75,
		},
		{
			name: "int64 values",
			signals: []ViolationSignal{
				{Name: "a", Weight: 1.0, Value: int64(60)},
			},
			want: 60,
		},
		{
			name: "unknown types skipped",
			signals: []ViolationSignal{
				{Name: "a", Weight: 1.0, Value: 80.0},
				{Name: "b", Weight: 1.0, Value: "ignored"},
			},
			want: 80, // string signal skipped, only float64 counted
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateConfidence(tt.signals)
			if got != tt.want {
				t.Errorf("CalculateConfidence() = %v, want %v", got, tt.want)
			}
		})
	}
}
