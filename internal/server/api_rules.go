package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
	"streammon/internal/notifier"
	"streammon/internal/store"
)

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}
	if rules == nil {
		rules = []models.Rule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleGetRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	rule, err := s.store.GetRule(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var rule models.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := rule.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.CreateRule(&rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}

	s.invalidateRulesCache()
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	// Verify rule exists
	if _, err := s.store.GetRule(id); err != nil {
		writeStoreError(w, err)
		return
	}

	var rule models.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	rule.ID = id
	if err := rule.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.UpdateRule(&rule); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}

	s.invalidateRulesCache()
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	// Verify rule exists
	if _, err := s.store.GetRule(id); err != nil {
		writeStoreError(w, err)
		return
	}

	if err := s.store.DeleteRule(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	s.invalidateRulesCache()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListViolations(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r, 50, 100)

	var filters store.ViolationFilters
	filters.UserName = r.URL.Query().Get("user")

	if severity := r.URL.Query().Get("severity"); severity != "" {
		sev := models.Severity(severity)
		if !sev.Valid() {
			writeError(w, http.StatusBadRequest, "invalid severity value")
			return
		}
		filters.Severity = sev
	}

	if ruleID := r.URL.Query().Get("rule_id"); ruleID != "" {
		filters.RuleID, _ = strconv.ParseInt(ruleID, 10, 64)
	}

	if ruleType := r.URL.Query().Get("rule_type"); ruleType != "" {
		rt := models.RuleType(ruleType)
		if !rt.Valid() {
			writeError(w, http.StatusBadRequest, "invalid rule_type value")
			return
		}
		filters.RuleType = rt
	}

	if minConf := r.URL.Query().Get("min_confidence"); minConf != "" {
		conf, err := strconv.ParseFloat(minConf, 64)
		if err != nil || conf < 0 || conf > 100 {
			writeError(w, http.StatusBadRequest, "min_confidence must be between 0 and 100")
			return
		}
		filters.MinConfidence = conf
	}

	if since := r.URL.Query().Get("since"); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since timestamp format")
			return
		}
		filters.Since = t
	}

	result, err := s.store.ListViolations(page, perPage, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list violations")
		return
	}
	if result.Items == nil {
		result.Items = []models.RuleViolation{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListNotificationChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.store.ListNotificationChannels()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}
	if channels == nil {
		channels = []models.NotificationChannel{}
	}
	writeJSON(w, http.StatusOK, channels)
}

func (s *Server) handleGetNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	channel, err := s.store.GetNotificationChannel(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, channel)
}

func (s *Server) handleCreateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	var channel models.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := channel.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.CreateNotificationChannel(&channel); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create channel")
		return
	}

	writeJSON(w, http.StatusCreated, channel)
}

func (s *Server) handleUpdateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	var channel models.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	channel.ID = id
	if err := channel.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.UpdateNotificationChannel(&channel); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update channel")
		return
	}

	writeJSON(w, http.StatusOK, channel)
}

func (s *Server) handleDeleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	if err := s.store.DeleteNotificationChannel(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestNotificationChannel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	channel, err := s.store.GetNotificationChannel(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	n := notifier.New()
	if err := n.TestChannel(r.Context(), channel); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// requireGuestVisibility checks that the current user is allowed to view
// a particular section. Admins always pass. Non-admins must be viewing
// their own data and the corresponding guest setting must be enabled.
// Returns true if the request should continue; false means an error was written.
func (s *Server) requireGuestVisibility(w http.ResponseWriter, r *http.Request, userName, settingKey string) bool {
	user := UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if user.Role == models.RoleAdmin {
		return true
	}
	if user.Name != userName {
		writeError(w, http.StatusForbidden, "forbidden")
		return false
	}
	visible, err := s.store.GetGuestSetting(settingKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return false
	}
	if !visible {
		writeError(w, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

func (s *Server) handleGetUserTrustScore(w http.ResponseWriter, r *http.Request) {
	userName := chi.URLParam(r, "name")
	if userName == "" {
		writeError(w, http.StatusBadRequest, "user name required")
		return
	}
	if !s.requireGuestVisibility(w, r, userName, "visible_trust_score") {
		return
	}

	ts, err := s.store.GetUserTrustScore(userName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get trust score")
		return
	}
	writeJSON(w, http.StatusOK, ts)
}

func (s *Server) handleGetUserViolations(w http.ResponseWriter, r *http.Request) {
	userName := chi.URLParam(r, "name")
	if userName == "" {
		writeError(w, http.StatusBadRequest, "user name required")
		return
	}
	if !s.requireGuestVisibility(w, r, userName, "visible_violations") {
		return
	}

	page, perPage := parsePagination(r, 50, 100)

	var filters store.ViolationFilters
	filters.UserName = userName

	result, err := s.store.ListViolations(page, perPage, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list violations")
		return
	}
	if result.Items == nil {
		result.Items = []models.RuleViolation{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListHouseholdLocations(w http.ResponseWriter, r *http.Request) {
	userName := chi.URLParam(r, "name")
	if userName == "" {
		writeError(w, http.StatusBadRequest, "user name required")
		return
	}
	if !s.requireGuestVisibility(w, r, userName, "visible_household") {
		return
	}

	locations, err := s.store.ListHouseholdLocations(userName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list household locations")
		return
	}
	if locations == nil {
		locations = []models.HouseholdLocation{}
	}
	writeJSON(w, http.StatusOK, locations)
}

func (s *Server) handleCreateHouseholdLocation(w http.ResponseWriter, r *http.Request) {
	userName := chi.URLParam(r, "name")
	if userName == "" {
		writeError(w, http.StatusBadRequest, "user name required")
		return
	}

	var loc models.HouseholdLocation
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	loc.UserName = userName
	if err := loc.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.UpsertHouseholdLocation(&loc); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save household location")
		return
	}

	writeJSON(w, http.StatusCreated, loc)
}

func (s *Server) handleUpdateHouseholdTrusted(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid location id")
		return
	}

	var body struct {
		Trusted bool `json:"trusted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.store.UpdateHouseholdTrusted(id, body.Trusted); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update household")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteHouseholdLocation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid location id")
		return
	}

	if err := s.store.DeleteHouseholdLocation(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete household location")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCalculateHouseholdLocations(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MinSessions int `json:"min_sessions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.MinSessions = 10
	}
	if body.MinSessions <= 0 {
		body.MinSessions = 10
	}

	created, err := s.store.CalculateAllHouseholdLocations(r.Context(), body.MinSessions)
	if err != nil {
		log.Printf("CalculateAllHouseholdLocations error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to calculate household locations")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"created":      created,
		"min_sessions": body.MinSessions,
	})
}

func (s *Server) handleLinkRuleToChannel(w http.ResponseWriter, r *http.Request) {
	ruleID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	var body struct {
		ChannelID int64 `json:"channel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.store.LinkRuleToChannel(ruleID, body.ChannelID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link channel")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUnlinkRuleFromChannel(w http.ResponseWriter, r *http.Request) {
	ruleID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	channelID, err := strconv.ParseInt(chi.URLParam(r, "channelId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	if err := s.store.UnlinkRuleFromChannel(ruleID, channelID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink channel")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetRuleChannels(w http.ResponseWriter, r *http.Request) {
	ruleID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}

	channels, err := s.store.GetChannelsForRule(ruleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get channels")
		return
	}
	if channels == nil {
		channels = []models.NotificationChannel{}
	}
	writeJSON(w, http.StatusOK, channels)
}

// invalidateRulesCache notifies the rules engine to clear its cache.
func (s *Server) invalidateRulesCache() {
	if s.rulesEngine != nil {
		s.rulesEngine.InvalidateCache()
	}
}
