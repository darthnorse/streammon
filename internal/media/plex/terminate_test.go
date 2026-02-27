package plex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/models"
)

func TestTerminateSession_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status/sessions/terminate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("sessionId") != "abc-uuid-123" {
			t.Errorf("unexpected sessionId: %s", r.URL.Query().Get("sessionId"))
		}
		if r.URL.Query().Get("reason") != "test message" {
			t.Errorf("unexpected reason: %s", r.URL.Query().Get("reason"))
		}
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Error("missing plex token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, Name: "TestPlex", URL: ts.URL, APIKey: "test-token"})
	err := srv.TerminateSession(context.Background(), "abc-uuid-123", "test message")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestTerminateSession_EmptyMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("reason") != "" {
			t.Errorf("expected no reason param, got: %q", r.URL.Query().Get("reason"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, Name: "TestPlex", URL: ts.URL, APIKey: "test-token"})
	err := srv.TerminateSession(context.Background(), "abc-uuid-123", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestTerminateSession_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, Name: "TestPlex", URL: ts.URL, APIKey: "test-token"})
	err := srv.TerminateSession(context.Background(), "abc-uuid-123", "bye")
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !errors.Is(err, models.ErrPlexPassRequired) {
		t.Errorf("expected ErrPlexPassRequired, got: %v", err)
	}
}

func TestTerminateSession_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, Name: "TestPlex", URL: ts.URL, APIKey: "test-token"})
	err := srv.TerminateSession(context.Background(), "abc-uuid-123", "bye")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}
