package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streammon/internal/models"
)

type ISPVelocityEvaluator struct {
	store HistoryQuerier
	geo   GeoResolver
}

func NewISPVelocityEvaluator(geo GeoResolver, store HistoryQuerier) *ISPVelocityEvaluator {
	return &ISPVelocityEvaluator{store: store, geo: geo}
}

func (e *ISPVelocityEvaluator) Type() models.RuleType {
	return models.RuleTypeISPVelocity
}

func (e *ISPVelocityEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil {
		return nil, nil
	}

	var config models.ISPVelocityConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	stream := input.Stream

	// Get ISP for the current stream
	var currentISP string
	if input.GeoData != nil && input.GeoData.ISP != "" {
		currentISP = input.GeoData.ISP
	} else if e.geo != nil && stream.IPAddress != "" {
		geo, err := e.geo.Lookup(ctx, stream.IPAddress)
		if err == nil && geo != nil {
			currentISP = geo.ISP
		}
	}

	// If we can't determine ISP, skip evaluation
	if currentISP == "" {
		return nil, nil
	}

	recentISPs, err := e.store.GetRecentISPs(stream.UserName, stream.StartedAt, config.TimeWindowHours)
	if err != nil {
		return nil, fmt.Errorf("getting recent ISPs: %w", err)
	}

	// Check if current ISP is already in the list
	found := false
	for _, isp := range recentISPs {
		if isp == currentISP {
			found = true
			break
		}
	}

	ispCount := len(recentISPs)
	if !found {
		ispCount++
	}

	if ispCount <= config.MaxISPs {
		return nil, nil
	}

	severity := models.SeverityInfo
	if ispCount >= config.MaxISPs*2 {
		severity = models.SeverityCritical
	} else if ispCount >= config.MaxISPs+2 {
		severity = models.SeverityWarning
	}

	// Calculate time window in days for clearer message
	timeWindowDays := config.TimeWindowHours / 24
	timeUnit := "hours"
	timeValue := config.TimeWindowHours
	if timeWindowDays >= 1 && config.TimeWindowHours%24 == 0 {
		timeUnit = "days"
		timeValue = timeWindowDays
		if timeWindowDays == 7 {
			timeUnit = "week"
			timeValue = 1
		}
	}

	violation := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: stream.UserName,
		Severity: severity,
		Message:  fmt.Sprintf("user has used %d different ISPs in the past %d %s (max: %d)", ispCount, timeValue, timeUnit, config.MaxISPs),
		Details: map[string]interface{}{
			"isp_count":   ispCount,
			"max_allowed": config.MaxISPs,
			"time_window": config.TimeWindowHours,
			"current_isp": currentISP,
			"recent_isps": recentISPs,
		},
		ConfidenceScore: float64(50 + (ispCount-config.MaxISPs)*15),
		OccurredAt:      time.Now().UTC(),
	}
	if violation.ConfidenceScore > 100 {
		violation.ConfidenceScore = 100
	}

	return &EvaluationResult{
		Violation: violation,
		Signals: []models.ViolationSignal{
			{Name: "isp_count", Weight: 0.7, Value: float64(ispCount)},
			{Name: "max_allowed", Weight: 0.0, Value: float64(config.MaxISPs)},
			{Name: "excess", Weight: 0.3, Value: float64(ispCount - config.MaxISPs)},
		},
	}, nil
}
