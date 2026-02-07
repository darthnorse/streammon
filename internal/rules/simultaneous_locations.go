package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streammon/internal/models"
	"streammon/internal/units"
)

type SimultaneousLocsEvaluator struct {
	geoResolver GeoResolver
}

func NewSimultaneousLocsEvaluator(resolver GeoResolver) *SimultaneousLocsEvaluator {
	return &SimultaneousLocsEvaluator{geoResolver: resolver}
}

func (e *SimultaneousLocsEvaluator) Type() models.RuleType {
	return models.RuleTypeSimultaneousLocs
}

func (e *SimultaneousLocsEvaluator) Evaluate(ctx context.Context, rule *models.Rule, input *EvaluationInput) (*EvaluationResult, error) {
	if input.Stream == nil {
		return nil, nil
	}

	var config models.SimultaneousLocsConfig
	if err := json.Unmarshal(rule.Config, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	userName := input.Stream.UserName
	userStreams := filterStreamsByUser(input.AllStreams, userName)

	if len(userStreams) < 2 {
		return nil, nil
	}

	locations := make(map[string][]locationInfo)
	for _, s := range userStreams {
		geo := e.resolveGeo(ctx, s.IPAddress, input)
		if geo == nil {
			continue
		}

		key := fmt.Sprintf("%s,%s", geo.City, geo.Country)
		locations[key] = append(locations[key], locationInfo{
			ip:      s.IPAddress,
			lat:     geo.Lat,
			lng:     geo.Lng,
			city:    geo.City,
			country: geo.Country,
		})
	}

	if len(locations) < 2 {
		return nil, nil
	}

	var locs []locationInfo
	for _, infos := range locations {
		if len(infos) > 0 {
			locs = append(locs, infos[0])
		}
	}

	maxDistance := 0.0
	var loc1, loc2 locationInfo
	for i := 0; i < len(locs); i++ {
		for j := i + 1; j < len(locs); j++ {
			dist := HaversineDistance(locs[i].lat, locs[i].lng, locs[j].lat, locs[j].lng)
			if dist > maxDistance {
				maxDistance = dist
				loc1 = locs[i]
				loc2 = locs[j]
			}
		}
	}

	if maxDistance < config.MinDistanceKm {
		return nil, nil
	}

	if config.ExemptHousehold && allLocationsInHousehold(locs, input.Households) {
		return nil, nil
	}

	signals := []models.ViolationSignal{
		{Name: "distance_km", Weight: 0.6, Value: maxDistance},
		{Name: "location_count", Weight: 0.4, Value: float64(len(locations)) * 25},
	}

	confidence := models.CalculateConfidence(signals)
	if confidence < 70 {
		confidence = 70
	}

	distStr := units.FormatDistance(maxDistance, input.UnitSystem)

	v := &models.RuleViolation{
		RuleID:   rule.ID,
		UserName: userName,
		Severity: determineSeverityByDistance(maxDistance),
		Message: fmt.Sprintf("streaming from %d different locations simultaneously (%s apart)",
			len(locations), distStr),
		Details: map[string]interface{}{
			"location_count": len(locations),
			"max_distance":   maxDistance,
			"location_1": map[string]interface{}{
				"city":    loc1.city,
				"country": loc1.country,
				"ip":      loc1.ip,
			},
			"location_2": map[string]interface{}{
				"city":    loc2.city,
				"country": loc2.country,
				"ip":      loc2.ip,
			},
		},
		ConfidenceScore: confidence,
		OccurredAt:      time.Now().UTC(),
	}

	return &EvaluationResult{
		Violation: v,
		Signals:   signals,
	}, nil
}

type locationInfo struct {
	ip      string
	lat     float64
	lng     float64
	city    string
	country string
}

func (e *SimultaneousLocsEvaluator) resolveGeo(ctx context.Context, ip string, input *EvaluationInput) *models.GeoResult {
	if input.GeoData != nil && input.GeoData.IP == ip {
		return input.GeoData
	}

	if e.geoResolver == nil {
		return nil
	}

	geo, err := e.geoResolver.Lookup(ctx, ip)
	if err != nil {
		return nil
	}
	return geo
}

func allLocationsInHousehold(locs []locationInfo, households []models.HouseholdLocation) bool {
	if len(households) == 0 {
		return false
	}

	householdIPs := trustedHouseholdIPs(households)
	for _, loc := range locs {
		if !householdIPs[loc.ip] {
			return false
		}
	}
	return true
}

func determineSeverityByDistance(distanceKm float64) models.Severity {
	if distanceKm >= 500 {
		return models.SeverityCritical
	}
	if distanceKm >= 200 {
		return models.SeverityWarning
	}
	return models.SeverityInfo
}
