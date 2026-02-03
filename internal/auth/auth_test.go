package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestReload_DisabledToDisabled(t *testing.T) {
	svc, err := NewService(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if svc.Enabled() {
		t.Fatal("expected disabled")
	}

	err = svc.Reload(context.Background(), Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Enabled() {
		t.Error("expected still disabled after reload with empty config")
	}
}

func TestReload_EnabledToDisabled(t *testing.T) {
	svc := &Service{enabled: true}

	err := svc.Reload(context.Background(), Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Enabled() {
		t.Error("expected disabled after reload with empty config")
	}
}

func TestReload_InvalidConfig(t *testing.T) {
	svc := &Service{enabled: true}

	err := svc.Reload(context.Background(), Config{Issuer: "https://example.com"})
	if err == nil {
		t.Error("expected error for incomplete config")
	}
	if !svc.Enabled() {
		t.Error("expected still enabled after failed reload")
	}
}

func TestReload_ConcurrentAccess(t *testing.T) {
	svc := &Service{enabled: true}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			svc.Enabled()
		}()
		go func() {
			defer wg.Done()
			svc.Reload(context.Background(), Config{})
		}()
	}
	wg.Wait()
}

func TestReload_ConcurrentHandlerAccess(t *testing.T) {
	svc := &Service{enabled: false}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/auth/login", nil)
			w := httptest.NewRecorder()
			svc.HandleLogin(w, req)
		}()
		go func() {
			defer wg.Done()
			svc.Reload(context.Background(), Config{})
		}()
	}
	wg.Wait()
}
