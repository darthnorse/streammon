package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streammon/internal/models"
)

func TestListRuleExemptions_Empty(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	rule := &models.Rule{
		Name: "Test", Type: models.RuleTypeConcurrentStreams,
		Enabled: true, Config: json.RawMessage(`{}`),
	}
	st.CreateRule(rule)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var names []string
	json.Unmarshal(w.Body.Bytes(), &names)
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

func TestSetRuleExemptions(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	rule := &models.Rule{
		Name: "Test", Type: models.RuleTypeConcurrentStreams,
		Enabled: true, Config: json.RawMessage(`{}`),
	}
	st.CreateRule(rule)

	// Set exemptions
	body := `["alice","bob"]`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify via GET
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var names []string
	json.Unmarshal(w.Body.Bytes(), &names)
	if len(names) != 2 {
		t.Fatalf("expected 2 exemptions, got %d", len(names))
	}
	if names[0] != "alice" || names[1] != "bob" {
		t.Errorf("expected [alice, bob], got %v", names)
	}
}

func TestSetRuleExemptions_Replace(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	rule := &models.Rule{
		Name: "Test", Type: models.RuleTypeConcurrentStreams,
		Enabled: true, Config: json.RawMessage(`{}`),
	}
	st.CreateRule(rule)

	// Set initial
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), strings.NewReader(`["alice"]`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Replace
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), strings.NewReader(`["charlie"]`))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var names []string
	json.Unmarshal(w.Body.Bytes(), &names)
	if len(names) != 1 || names[0] != "charlie" {
		t.Errorf("expected [charlie], got %v", names)
	}
}

func TestListRuleExemptions_NotFound(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/rules/999/exemptions", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetRuleExemptions_InvalidJSON(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	rule := &models.Rule{
		Name: "Test", Type: models.RuleTypeConcurrentStreams,
		Enabled: true, Config: json.RawMessage(`{}`),
	}
	st.CreateRule(rule)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rules/%d/exemptions", rule.ID), strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
