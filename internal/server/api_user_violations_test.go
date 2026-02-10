package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
)

func setupViolation(t *testing.T, st interface {
	CreateRule(rule *models.Rule) error
	InsertViolation(v *models.RuleViolation) error
}, userName string) {
	t.Helper()
	rule := &models.Rule{
		Name:    "Test Rule",
		Type:    models.RuleTypeConcurrentStreams,
		Enabled: true,
		Config:  json.RawMessage(`{}`),
	}
	if err := st.CreateRule(rule); err != nil {
		t.Fatalf("creating rule: %v", err)
	}
	v := &models.RuleViolation{
		RuleID:          rule.ID,
		UserName:        userName,
		Severity:        models.SeverityWarning,
		Message:         "test violation",
		ConfidenceScore: 80,
		OccurredAt:      time.Now().UTC(),
	}
	if err := st.InsertViolation(v); err != nil {
		t.Fatalf("inserting violation: %v", err)
	}
}

func TestUserViolations(t *testing.T) {
	t.Run("viewer cannot access own violations when setting disabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		viewerToken := createViewerSession(t, st, "viewer-v1")
		setupViolation(t, st, "viewer-v1")

		req := httptest.NewRequest(http.MethodGet, "/api/users/viewer-v1/violations", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("viewer can access own violations when setting enabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		if err := st.SetTrustScoreVisibility(true); err != nil {
			t.Fatal(err)
		}
		viewerToken := createViewerSession(t, st, "viewer-v2")
		setupViolation(t, st, "viewer-v2")

		req := httptest.NewRequest(http.MethodGet, "/api/users/viewer-v2/violations", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp models.PaginatedResult[models.RuleViolation]
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Fatalf("expected 1 violation, got %d", len(resp.Items))
		}
	})

	t.Run("viewer cannot access other user violations even when enabled", func(t *testing.T) {
		srv, st := newTestServer(t)
		if err := st.SetTrustScoreVisibility(true); err != nil {
			t.Fatal(err)
		}
		viewerToken := createViewerSession(t, st, "viewer-v3")
		setupViolation(t, st, "other-user")

		req := httptest.NewRequest(http.MethodGet, "/api/users/other-user/violations", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("admin can access any user violations regardless of setting", func(t *testing.T) {
		srv, st := newTestServerWrapped(t)
		setupViolation(t, st, "some-user")

		req := httptest.NewRequest(http.MethodGet, "/api/users/some-user/violations", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp models.PaginatedResult[models.RuleViolation]
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Fatalf("expected 1 violation, got %d", len(resp.Items))
		}
	})
}
