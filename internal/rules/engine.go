package rules

import (
	"context"
	"log"
	"sync"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

type Engine struct {
	store       *store.Store
	geoResolver GeoResolver
	evaluators  map[models.RuleType]Evaluator
	notifier    Notifier

	mu          sync.RWMutex
	cachedRules []models.Rule
	lastRefresh time.Time

	ruleCacheTTL      time.Duration
	violationCooldown time.Duration

	// Trust score decrements by severity
	trustDecrementCritical int
	trustDecrementWarning  int
	trustDecrementInfo     int

	// Track in-flight notification goroutines for graceful shutdown
	notifyWg sync.WaitGroup
}

type Notifier interface {
	Notify(ctx context.Context, violation *models.RuleViolation, channels []models.NotificationChannel) error
}

type EngineConfig struct {
	RuleCacheTTL      time.Duration
	ViolationCooldown time.Duration

	// Trust score decrements by severity (default: 20/10/5)
	TrustDecrementCritical int
	TrustDecrementWarning  int
	TrustDecrementInfo     int
}

func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		RuleCacheTTL:           5 * time.Minute,
		ViolationCooldown:      15 * time.Minute,
		TrustDecrementCritical: 20,
		TrustDecrementWarning:  10,
		TrustDecrementInfo:     5,
	}
}

func NewEngine(s *store.Store, geo GeoResolver, config EngineConfig) *Engine {
	// Apply defaults for zero values
	if config.TrustDecrementCritical == 0 {
		config.TrustDecrementCritical = 20
	}
	if config.TrustDecrementWarning == 0 {
		config.TrustDecrementWarning = 10
	}
	if config.TrustDecrementInfo == 0 {
		config.TrustDecrementInfo = 5
	}

	e := &Engine{
		store:                  s,
		geoResolver:           geo,
		evaluators:            make(map[models.RuleType]Evaluator),
		ruleCacheTTL:          config.RuleCacheTTL,
		violationCooldown:     config.ViolationCooldown,
		trustDecrementCritical: config.TrustDecrementCritical,
		trustDecrementWarning:  config.TrustDecrementWarning,
		trustDecrementInfo:     config.TrustDecrementInfo,
	}

	e.RegisterEvaluator(NewConcurrentStreamsEvaluator())
	e.RegisterEvaluator(NewGeoRestrictionEvaluator())
	e.RegisterEvaluator(NewSimultaneousLocsEvaluator(geo))
	e.RegisterEvaluator(NewImpossibleTravelEvaluator(geo, s))
	e.RegisterEvaluator(NewDeviceVelocityEvaluator(s))
	e.RegisterEvaluator(NewNewDeviceEvaluator(s))
	e.RegisterEvaluator(NewNewLocationEvaluator(geo, s))
	e.RegisterEvaluator(NewISPVelocityEvaluator(geo, s))

	return e
}

func (e *Engine) RegisterEvaluator(evaluator Evaluator) {
	e.evaluators[evaluator.Type()] = evaluator
}

func (e *Engine) SetNotifier(n Notifier) {
	e.notifier = n
}

func (e *Engine) EvaluateSession(ctx context.Context, stream *models.ActiveStream, allStreams []models.ActiveStream) {
	if stream == nil {
		return
	}

	rules, err := e.getEnabledRules()
	if err != nil {
		log.Printf("rules engine: failed to get rules: %v", err)
		return
	}

	input := &EvaluationInput{
		Stream:     stream,
		AllStreams: allStreams,
	}

	if e.geoResolver != nil && stream.IPAddress != "" {
		geo, err := e.geoResolver.Lookup(ctx, stream.IPAddress)
		if err == nil {
			input.GeoData = geo
		}
	}

	households, err := e.store.ListTrustedHouseholdLocations(stream.UserName)
	if err == nil {
		input.Households = households
	}

	for _, rule := range rules {
		if !rule.Type.IsRealTime() {
			continue
		}

		evaluator, ok := e.evaluators[rule.Type]
		if !ok {
			continue
		}

		e.evaluateRule(ctx, &rule, evaluator, input)
	}
}

func (e *Engine) evaluateRule(ctx context.Context, rule *models.Rule, evaluator Evaluator, input *EvaluationInput) {
	result, err := evaluator.Evaluate(ctx, rule, input)
	if err != nil {
		log.Printf("rules engine: error evaluating rule %d (%s): %v", rule.ID, rule.Name, err)
		return
	}

	if result == nil || result.Violation == nil {
		return
	}

	// Set session key from the current stream for session-based deduplication
	// This prevents duplicate alerts for the same stream session even if it runs > 15 minutes
	sessionKey := ""
	if input.Stream != nil && input.Stream.SessionID != "" {
		sessionKey = input.Stream.SessionID
		result.Violation.SessionKey = sessionKey
	}

	exists, err := e.store.ViolationExistsRecent(rule.ID, result.Violation.UserName, sessionKey, e.violationCooldown)
	if err != nil {
		log.Printf("rules engine: error checking recent violation: %v", err)
		return
	}
	if exists {
		return
	}

	trustDecrement := e.getTrustDecrement(result.Violation.Severity)

	if err := e.store.InsertViolationWithTx(ctx, result.Violation, trustDecrement); err != nil {
		log.Printf("rules engine: error inserting violation: %v", err)
		return
	}

	log.Printf("rules engine: violation detected - rule=%s user=%s severity=%s confidence=%.1f",
		rule.Name, result.Violation.UserName, result.Violation.Severity, result.Violation.ConfidenceScore)

	if e.notifier != nil {
		e.notifyWg.Add(1)
		go e.sendNotifications(rule.ID, result.Violation)
	}
}

func (e *Engine) sendNotifications(ruleID int64, violation *models.RuleViolation) {
	defer e.notifyWg.Done()

	channels, err := e.store.GetChannelsForRule(ruleID)
	if err != nil {
		log.Printf("rules engine: error getting channels for rule %d: %v", ruleID, err)
		return
	}

	if len(channels) == 0 {
		return
	}

	// Use background context so notifications complete even during shutdown
	notifyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.notifier.Notify(notifyCtx, violation, channels); err != nil {
		log.Printf("rules engine: error sending notifications: %v", err)
	}
}

// WaitForNotifications waits for all in-flight notification goroutines to complete.
// Call this during graceful shutdown.
func (e *Engine) WaitForNotifications() {
	e.notifyWg.Wait()
}

func (e *Engine) getTrustDecrement(severity models.Severity) int {
	switch severity {
	case models.SeverityCritical:
		return e.trustDecrementCritical
	case models.SeverityWarning:
		return e.trustDecrementWarning
	case models.SeverityInfo:
		return e.trustDecrementInfo
	default:
		return 0
	}
}

func (e *Engine) getEnabledRules() ([]models.Rule, error) {
	e.mu.RLock()
	if time.Since(e.lastRefresh) < e.ruleCacheTTL && len(e.cachedRules) > 0 {
		rules := e.cachedRules
		e.mu.RUnlock()
		return rules, nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	if time.Since(e.lastRefresh) < e.ruleCacheTTL && len(e.cachedRules) > 0 {
		return e.cachedRules, nil
	}

	rules, err := e.store.ListEnabledRules()
	if err != nil {
		return nil, err
	}

	e.cachedRules = rules
	e.lastRefresh = time.Now().UTC()

	return rules, nil
}

func (e *Engine) RefreshRules() error {
	rules, err := e.store.ListEnabledRules()
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.cachedRules = rules
	e.lastRefresh = time.Now().UTC()
	e.mu.Unlock()

	return nil
}

// InvalidateCache clears the rules cache, forcing the next evaluation to fetch fresh rules.
func (e *Engine) InvalidateCache() {
	e.mu.Lock()
	e.cachedRules = nil
	e.lastRefresh = time.Time{}
	e.mu.Unlock()
}

func (e *Engine) GetEvaluators() map[models.RuleType]Evaluator {
	return e.evaluators
}
