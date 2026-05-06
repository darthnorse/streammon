package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/auth"
)

func TestGetAPIKeyStatus_NotConfigured(t *testing.T) {
	ts, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/api-key", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp apiKeyStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Configured {
		t.Error("expected configured=false on fresh server")
	}
	if resp.CreatedAt != nil {
		t.Errorf("expected nil created_at, got %v", resp.CreatedAt)
	}
}

func TestRotateAPIKey_PersistsHashAndReturnsPlaintext(t *testing.T) {
	ts, st := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/api-key/rotate", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp apiKeyRotateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Key == "" {
		t.Fatal("expected plaintext key in response")
	}

	hash, _ := st.GetAPIKeyHash()
	if hash != auth.HashAPIKey(resp.Key) {
		t.Error("stored hash does not match plaintext")
	}
	if hash == resp.Key {
		t.Error("stored value must be the hash, not the plaintext")
	}
	createdAt, _ := st.GetAPIKeyCreatedAt()
	if time.Since(createdAt) > 5*time.Second {
		t.Errorf("created_at too old: %v", createdAt)
	}
}

func TestRotateAPIKey_StatusReportsConfigured(t *testing.T) {
	ts, _ := newTestServerWrapped(t)

	rotateReq := httptest.NewRequest(http.MethodPost, "/api/admin/api-key/rotate", nil)
	ts.ServeHTTP(httptest.NewRecorder(), rotateReq)

	statusReq := httptest.NewRequest(http.MethodGet, "/api/admin/api-key", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, statusReq)

	var resp apiKeyStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Configured {
		t.Error("expected configured=true after rotate")
	}
	if resp.CreatedAt == nil {
		t.Error("expected created_at on configured key")
	}
}

func TestRotateAPIKey_RejectsAPIKeyAuth(t *testing.T) {
	srv, st := newTestServer(t)

	plain, _ := auth.GenerateAPIKey()
	if err := st.SetAPIKey(auth.HashAPIKey(plain), time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/api-key/rotate", nil)
	req.Header.Set("X-API-Key", plain)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 (cookie-only), got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRevokeAPIKey_ClearsStoredHash(t *testing.T) {
	ts, st := newTestServerWrapped(t)

	if err := st.SetAPIKey("hash-x", time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/api-key", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	hash, _ := st.GetAPIKeyHash()
	if hash != "" {
		t.Errorf("expected empty hash after revoke, got %q", hash)
	}
}

func TestRevokeAPIKey_RejectsAPIKeyAuth(t *testing.T) {
	srv, st := newTestServer(t)

	plain, _ := auth.GenerateAPIKey()
	if err := st.SetAPIKey(auth.HashAPIKey(plain), time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/api-key", nil)
	req.Header.Set("X-API-Key", plain)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}
