package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"streammon/internal/models"
)

type ConcurrentStreamsEvaluator struct{}

func NewConcurrentStreamsEvaluator() *ConcurrentStreamsEvaluator {
	return &ConcurrentStreamsEvaluator{}
}

func (e *ConcurrentStreamsEvaluator) Type() models.RuleType {
	return models.RuleTypeConcurrentStreams
}

func (e *ConcurrentStreamsEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil {
		return nil, nil
	}

	var config models.ConcurrentStreamsConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	userName := input.Stream.UserName
	userStreams := filterStreamsByUser(input.AllStreams, userName)
	if config.CountPausedAsOne {
		userStreams = collapsePausedStreams(userStreams)
	}
	streamCount := len(userStreams)

	if streamCount <= config.MaxStreams {
		return nil, nil
	}

	if config.ExemptHousehold && allFromHousehold(userStreams, input.Households) {
		return nil, nil
	}

	// Tiebreak by SessionID for deterministic ordering with identical timestamps.
	// When paused streams are collapsed to a single representative, that
	// representative must never be preferred over a genuinely active stream
	// as the auto-terminate target — otherwise a paused session could be
	// terminated ahead of an active one. Scoped to CountPausedAsOne so the
	// default (disabled) path's ordering is unchanged.
	sort.Slice(userStreams, func(i, j int) bool {
		if config.CountPausedAsOne {
			iPaused := userStreams[i].State == models.SessionStatePaused
			jPaused := userStreams[j].State == models.SessionStatePaused
			if iPaused != jPaused {
				return !iPaused
			}
		}
		if userStreams[i].StartedAt.Equal(userStreams[j].StartedAt) {
			return userStreams[i].SessionID > userStreams[j].SessionID
		}
		return userStreams[i].StartedAt.After(userStreams[j].StartedAt)
	})
	newest := userStreams[0]

	signals := []models.ViolationSignal{
		{Name: "stream_count", Weight: 0.6, Value: float64(streamCount)},
		{Name: "max_allowed", Weight: 0.0, Value: float64(config.MaxStreams)},
		{Name: "excess", Weight: 0.4, Value: float64(streamCount-config.MaxStreams) * 25},
	}

	confidence := models.CalculateConfidence(signals)
	if confidence < 50 {
		confidence = 50 + float64(streamCount-config.MaxStreams)*10
	}
	if confidence > 100 {
		confidence = 100
	}

	locations := extractLocations(userStreams)
	devices := extractDevices(userStreams)

	violation := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: userName,
		Severity: determineSeverity(streamCount, config.MaxStreams),
		Message:  fmt.Sprintf("%d concurrent streams detected (max: %d)", streamCount, config.MaxStreams),
		Details: map[string]interface{}{
			"stream_count": streamCount,
			"max_allowed":  config.MaxStreams,
			"locations":    locations,
			"devices":      devices,
		},
		ConfidenceScore: confidence,
		OccurredAt:      time.Now().UTC(),
	}

	return &EvaluationResult{
		Violation: violation,
		Signals:   signals,
		TerminateTarget: &TerminateTarget{
			ServerID:        newest.ServerID,
			SessionID:       newest.SessionID,
			PlexSessionUUID: newest.PlexSessionUUID,
		},
	}, nil
}

// collapsePausedStreams collapses all paused streams down to a single
// representative (the most recently started one), so that multiple paused
// streams from one user count as one toward the concurrent-streams limit.
// Non-paused streams are unaffected.
func collapsePausedStreams(streams []models.ActiveStream) []models.ActiveStream {
	var result []models.ActiveStream
	var representative *models.ActiveStream

	for i := range streams {
		s := streams[i]
		if s.State != models.SessionStatePaused {
			result = append(result, s)
			continue
		}
		if representative == nil || s.StartedAt.After(representative.StartedAt) {
			representative = &s
		}
	}

	if representative != nil {
		result = append(result, *representative)
	}

	return result
}

func allFromHousehold(streams []models.ActiveStream, households []models.HouseholdLocation) bool {
	if len(households) == 0 {
		return false
	}

	householdIPs := trustedHouseholdIPs(households)
	for _, s := range streams {
		if s.IPAddress == "" || !householdIPs[s.IPAddress] {
			return false
		}
	}
	return true
}

func determineSeverity(actual, max int) models.Severity {
	excess := actual - max
	if excess >= 3 {
		return models.SeverityCritical
	}
	if excess >= 2 {
		return models.SeverityWarning
	}
	return models.SeverityInfo
}

func extractLocations(streams []models.ActiveStream) []string {
	seen := make(map[string]bool)
	var locations []string
	for _, s := range streams {
		if s.IPAddress != "" && !seen[s.IPAddress] {
			seen[s.IPAddress] = true
			locations = append(locations, s.IPAddress)
		}
	}
	return locations
}

func extractDevices(streams []models.ActiveStream) []string {
	seen := make(map[string]bool)
	var devices []string
	for _, s := range streams {
		key := fmt.Sprintf("%s (%s)", s.Player, s.Platform)
		if !seen[key] {
			seen[key] = true
			devices = append(devices, key)
		}
	}
	return devices
}
