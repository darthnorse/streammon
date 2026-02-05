package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"streammon/internal/models"
)

type GeoRestrictionEvaluator struct{}

func NewGeoRestrictionEvaluator() *GeoRestrictionEvaluator {
	return &GeoRestrictionEvaluator{}
}

func (e *GeoRestrictionEvaluator) Type() models.RuleType {
	return models.RuleTypeGeoRestriction
}

func (e *GeoRestrictionEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil || input.GeoData == nil {
		return nil, nil
	}

	var config models.GeoRestrictionConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	country := strings.ToUpper(input.GeoData.Country)
	if country == "" {
		return nil, nil
	}

	var violation bool
	var reason string

	if len(config.AllowedCountries) > 0 {
		allowed := false
		for _, c := range config.AllowedCountries {
			if strings.EqualFold(c, country) {
				allowed = true
				break
			}
		}
		if !allowed {
			violation = true
			reason = fmt.Sprintf("streaming from non-allowed country: %s", country)
		}
	}

	if !violation && len(config.BlockedCountries) > 0 {
		for _, c := range config.BlockedCountries {
			if strings.EqualFold(c, country) {
				violation = true
				reason = fmt.Sprintf("streaming from blocked country: %s", country)
				break
			}
		}
	}

	if !violation {
		return nil, nil
	}

	signals := []models.ViolationSignal{
		{Name: "geo_match", Weight: 1.0, Value: true},
	}

	v := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: input.Stream.UserName,
		Severity: models.SeverityCritical,
		Message:  reason,
		Details: map[string]interface{}{
			"country":           country,
			"city":              input.GeoData.City,
			"ip_address":        input.Stream.IPAddress,
			"allowed_countries": config.AllowedCountries,
			"blocked_countries": config.BlockedCountries,
		},
		ConfidenceScore: 100,
		OccurredAt:      time.Now().UTC(),
	}

	return &EvaluationResult{
		Violation: v,
		Signals:   signals,
	}, nil
}
