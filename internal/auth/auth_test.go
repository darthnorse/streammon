package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewService_Disabled(t *testing.T) {
	cfg := Config{}
	svc, err := NewService(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if svc.Enabled() {
		t.Error("expected auth disabled when config is empty")
	}
}

func TestNewService_MissingFields(t *testing.T) {
	cfg := Config{Issuer: "https://example.com"}
	_, err := NewService(cfg, nil)
	if err == nil {
		t.Error("expected error when only issuer set")
	}
}

func TestHandleLogin_Disabled(t *testing.T) {
	svc, _ := NewService(Config{}, nil)
	req := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()

	svc.HandleLogin(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleLogout_ClearsCookie(t *testing.T) {
	svc, _ := NewService(Config{}, nil)
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()

	svc.HandleLogout(w, req)

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == CookieName && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected session cookie to be cleared")
	}
}

func TestHandleCallback_Disabled(t *testing.T) {
	svc, _ := NewService(Config{}, nil)
	req := httptest.NewRequest("GET", "/auth/callback", nil)
	w := httptest.NewRecorder()

	svc.HandleCallback(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
