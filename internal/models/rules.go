package models

import (
	"encoding/json"
	"errors"
	"net/url"
	"time"
)

type RuleType string

const (
	RuleTypeImpossibleTravel  RuleType = "impossible_travel"
	RuleTypeConcurrentStreams RuleType = "concurrent_streams"
	RuleTypeSimultaneousLocs  RuleType = "simultaneous_locations"
	RuleTypeDeviceVelocity    RuleType = "device_velocity"
	RuleTypeGeoRestriction    RuleType = "geo_restriction"
	RuleTypeNewDevice         RuleType = "new_device"
	RuleTypeNewLocation       RuleType = "new_location"
)

func (rt RuleType) Valid() bool {
	switch rt {
	case RuleTypeImpossibleTravel, RuleTypeConcurrentStreams,
		RuleTypeSimultaneousLocs, RuleTypeDeviceVelocity,
		RuleTypeGeoRestriction, RuleTypeNewDevice, RuleTypeNewLocation:
		return true
	}
	return false
}

func (rt RuleType) IsRealTime() bool {
	switch rt {
	case RuleTypeConcurrentStreams, RuleTypeSimultaneousLocs,
		RuleTypeGeoRestriction, RuleTypeNewDevice, RuleTypeNewLocation,
		RuleTypeImpossibleTravel, RuleTypeDeviceVelocity:
		return true
	}
	return false
}

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

func (s Severity) Valid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	}
	return false
}

type ChannelType string

const (
	ChannelTypeDiscord  ChannelType = "discord"
	ChannelTypeWebhook  ChannelType = "webhook"
	ChannelTypePushover ChannelType = "pushover"
	ChannelTypeNtfy     ChannelType = "ntfy"
)

func (ct ChannelType) Valid() bool {
	switch ct {
	case ChannelTypeDiscord, ChannelTypeWebhook, ChannelTypePushover, ChannelTypeNtfy:
		return true
	}
	return false
}

type Rule struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Type      RuleType        `json:"type"`
	Enabled   bool            `json:"enabled"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (r *Rule) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if !r.Type.Valid() {
		return errors.New("invalid rule type")
	}
	if len(r.Config) == 0 {
		r.Config = json.RawMessage("{}")
	}
	return nil
}

type ImpossibleTravelConfig struct {
	MaxSpeedKmH     float64 `json:"max_speed_km_h"`
	MinDistanceKm   float64 `json:"min_distance_km"`
	TimeWindowHours int     `json:"time_window_hours"`
}

func (c *ImpossibleTravelConfig) Validate() error {
	if c.MaxSpeedKmH <= 0 {
		c.MaxSpeedKmH = 800
	}
	if c.MinDistanceKm <= 0 {
		c.MinDistanceKm = 100
	}
	if c.TimeWindowHours <= 0 {
		c.TimeWindowHours = 24
	}
	return nil
}

type DeviceVelocityConfig struct {
	MaxDevicesPerHour int `json:"max_devices_per_hour"`
	TimeWindowHours   int `json:"time_window_hours"`
}

func (c *DeviceVelocityConfig) Validate() error {
	if c.MaxDevicesPerHour <= 0 {
		c.MaxDevicesPerHour = 3
	}
	if c.TimeWindowHours <= 0 {
		c.TimeWindowHours = 1
	}
	return nil
}

type ConcurrentStreamsConfig struct {
	MaxStreams       int  `json:"max_streams"`
	ExemptHousehold  bool `json:"exempt_household"`
	CountPausedAsOne bool `json:"count_paused_as_one"`
}

func (c *ConcurrentStreamsConfig) Validate() error {
	if c.MaxStreams <= 0 {
		c.MaxStreams = 2
	}
	return nil
}

type SimultaneousLocsConfig struct {
	MinDistanceKm   float64 `json:"min_distance_km"`
	ExemptHousehold bool    `json:"exempt_household"`
}

func (c *SimultaneousLocsConfig) Validate() error {
	if c.MinDistanceKm <= 0 {
		c.MinDistanceKm = 50
	}
	return nil
}

type GeoRestrictionConfig struct {
	AllowedCountries []string `json:"allowed_countries"`
	BlockedCountries []string `json:"blocked_countries"`
}

func (c *GeoRestrictionConfig) Validate() error {
	return nil
}

type NewDeviceConfig struct {
	NotifyOnNew bool `json:"notify_on_new"`
}

func (c *NewDeviceConfig) Validate() error {
	return nil
}

type NewLocationConfig struct {
	NotifyOnNew         bool    `json:"notify_on_new"`
	MinDistanceKm       float64 `json:"min_distance_km"`
	SeverityThresholdKm float64 `json:"severity_threshold_km"`
	ExemptHousehold     bool    `json:"exempt_household"`
}

func (c *NewLocationConfig) Validate() error {
	if c.MinDistanceKm <= 0 {
		c.MinDistanceKm = 50
	}
	if c.SeverityThresholdKm <= 0 {
		c.SeverityThresholdKm = 500
	}
	return nil
}

type RuleViolation struct {
	ID              int64                  `json:"id"`
	RuleID          int64                  `json:"rule_id"`
	RuleName        string                 `json:"rule_name,omitempty"`
	RuleType        RuleType               `json:"rule_type,omitempty"`
	UserName        string                 `json:"user_name"`
	Severity        Severity               `json:"severity"`
	Message         string                 `json:"message"`
	Details         map[string]interface{} `json:"details,omitempty"`
	ConfidenceScore float64                `json:"confidence_score"`
	OccurredAt      time.Time              `json:"occurred_at"`
	CreatedAt       time.Time              `json:"created_at"`
}

func (v *RuleViolation) Validate() error {
	if v.RuleID == 0 {
		return errors.New("rule_id is required")
	}
	if v.UserName == "" {
		return errors.New("user_name is required")
	}
	if !v.Severity.Valid() {
		return errors.New("invalid severity")
	}
	if v.Message == "" {
		return errors.New("message is required")
	}
	if v.OccurredAt.IsZero() {
		v.OccurredAt = time.Now().UTC()
	}
	if v.ConfidenceScore < 0 || v.ConfidenceScore > 100 {
		return errors.New("confidence_score must be between 0 and 100")
	}
	return nil
}

type HouseholdLocation struct {
	ID           int64     `json:"id"`
	UserName     string    `json:"user_name"`
	IPAddress    string    `json:"ip_address,omitempty"`
	City         string    `json:"city,omitempty"`
	Country      string    `json:"country,omitempty"`
	Latitude     float64   `json:"latitude,omitempty"`
	Longitude    float64   `json:"longitude,omitempty"`
	AutoLearned  bool      `json:"auto_learned"`
	Trusted      bool      `json:"trusted"`
	SessionCount int       `json:"session_count"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *HouseholdLocation) Validate() error {
	if h.UserName == "" {
		return errors.New("user_name is required")
	}
	if h.IPAddress == "" && h.City == "" && h.Country == "" {
		return errors.New("ip_address or city/country is required")
	}
	if h.FirstSeen.IsZero() {
		h.FirstSeen = time.Now().UTC()
	}
	if h.LastSeen.IsZero() {
		h.LastSeen = h.FirstSeen
	}
	return nil
}

type UserTrustScore struct {
	UserName        string     `json:"user_name"`
	Score           int        `json:"score"`
	ViolationCount  int        `json:"violation_count"`
	LastViolationAt *time.Time `json:"last_violation_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type NotificationChannel struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	ChannelType ChannelType     `json:"channel_type"`
	Config      json.RawMessage `json:"config"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (n *NotificationChannel) Validate() error {
	if n.Name == "" {
		return errors.New("name is required")
	}
	if !n.ChannelType.Valid() {
		return errors.New("invalid channel type")
	}
	if len(n.Config) == 0 {
		return errors.New("config is required")
	}
	return nil
}

type DiscordConfig struct {
	WebhookURL string `json:"webhook_url"`
}

func (c *DiscordConfig) Validate() error {
	if c.WebhookURL == "" {
		return errors.New("webhook_url is required")
	}
	return nil
}

type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (c *WebhookConfig) Validate() error {
	if c.URL == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(c.URL)
	if err != nil {
		return errors.New("invalid url format")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must use http or https scheme")
	}
	if c.Method == "" {
		c.Method = "POST"
	}
	return nil
}

type PushoverConfig struct {
	UserKey  string `json:"user_key"`
	APIToken string `json:"api_token"`
}

func (c *PushoverConfig) Validate() error {
	if c.UserKey == "" {
		return errors.New("user_key is required")
	}
	if c.APIToken == "" {
		return errors.New("api_token is required")
	}
	return nil
}

type NtfyConfig struct {
	ServerURL string `json:"server_url"`
	Topic     string `json:"topic"`
	Token     string `json:"token,omitempty"`
}

func (c *NtfyConfig) Validate() error {
	if c.ServerURL == "" {
		c.ServerURL = "https://ntfy.sh"
	}
	if c.Topic == "" {
		return errors.New("topic is required")
	}
	return nil
}

type MaintenanceTaskStatus string

const (
	MaintenanceStatusPending   MaintenanceTaskStatus = "pending"
	MaintenanceStatusConfirmed MaintenanceTaskStatus = "confirmed"
	MaintenanceStatusExecuted  MaintenanceTaskStatus = "executed"
	MaintenanceStatusFailed    MaintenanceTaskStatus = "failed"
	MaintenanceStatusSkipped   MaintenanceTaskStatus = "skipped"
)

type MaintenanceTask struct {
	ID          int64                 `json:"id"`
	RuleID      int64                 `json:"rule_id"`
	ServerID    int64                 `json:"server_id"`
	ActionType  string                `json:"action_type"`
	ItemID      string                `json:"item_id"`
	Title       string                `json:"title"`
	Details     json.RawMessage       `json:"details,omitempty"`
	Status      MaintenanceTaskStatus `json:"status"`
	ConfirmedBy string                `json:"confirmed_by,omitempty"`
	ConfirmedAt *time.Time            `json:"confirmed_at,omitempty"`
	ExecutedAt  *time.Time            `json:"executed_at,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
}

type EvaluationContext struct {
	Stream           *ActiveStream
	AllStreams       []ActiveStream
	RecentHistory    []WatchHistoryEntry
	Households       []HouseholdLocation
	GeoData          *GeoResult
	PreviousGeoData  *GeoResult
	TimeSinceLastSeen time.Duration
}

type ViolationSignal struct {
	Name   string      `json:"name"`
	Weight float64     `json:"weight"`
	Value  interface{} `json:"value"`
}

type DeviceInfo struct {
	Player   string `json:"player"`
	Platform string `json:"platform"`
}

func CalculateConfidence(signals []ViolationSignal) float64 {
	if len(signals) == 0 {
		return 0
	}
	totalWeight := 0.0
	weightedSum := 0.0
	for _, s := range signals {
		var value float64
		switch v := s.Value.(type) {
		case float64:
			value = v
		case float32:
			value = float64(v)
		case int:
			value = float64(v)
		case int64:
			value = float64(v)
		case bool:
			if v {
				value = 100
			}
			// false contributes 0
		default:
			// Skip unknown types - don't add to totalWeight
			continue
		}
		totalWeight += s.Weight
		weightedSum += value * s.Weight
	}
	if totalWeight == 0 {
		return 0
	}
	score := weightedSum / totalWeight
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
