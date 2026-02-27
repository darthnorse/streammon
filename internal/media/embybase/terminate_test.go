package embybase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"streammon/internal/models"
)

func TestTerminateSession_WithMessage(t *testing.T) {
	var messageCalled, stopCalled atomic.Bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing auth header")
		}
		switch r.URL.Path {
		case "/Sessions/sess-123/Message":
			messageCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
		case "/Sessions/sess-123/Playing/Stop":
			stopCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, Name: "TestEmby", URL: ts.URL, APIKey: "test-key"}, models.ServerTypeEmby)
	err := c.TerminateSession(context.Background(), "sess-123", "goodbye")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !messageCalled.Load() {
		t.Error("expected message endpoint to be called")
	}
	if !stopCalled.Load() {
		t.Error("expected stop endpoint to be called")
	}
}

func TestTerminateSession_NoMessage(t *testing.T) {
	var messageCalled atomic.Bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Sessions/sess-123/Message":
			messageCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
		case "/Sessions/sess-123/Playing/Stop":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, Name: "TestEmby", URL: ts.URL, APIKey: "test-key"}, models.ServerTypeEmby)
	err := c.TerminateSession(context.Background(), "sess-123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if messageCalled.Load() {
		t.Error("expected message endpoint NOT to be called for empty message")
	}
}

func TestTerminateSession_MessageFailsStillStops(t *testing.T) {
	var stopCalled atomic.Bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Sessions/sess-123/Message":
			w.WriteHeader(http.StatusForbidden) // message fails
		case "/Sessions/sess-123/Playing/Stop":
			stopCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, Name: "TestEmby", URL: ts.URL, APIKey: "test-key"}, models.ServerTypeEmby)
	err := c.TerminateSession(context.Background(), "sess-123", "goodbye")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stopCalled.Load() {
		t.Error("expected stop to proceed even when message fails")
	}
}

func TestTerminateSession_StopFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Sessions/sess-123/Message":
			w.WriteHeader(http.StatusNoContent)
		case "/Sessions/sess-123/Playing/Stop":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	c := New(models.Server{ID: 1, Name: "TestEmby", URL: ts.URL, APIKey: "test-key"}, models.ServerTypeEmby)
	err := c.TerminateSession(context.Background(), "sess-123", "goodbye")
	if err == nil {
		t.Fatal("expected error when stop fails")
	}
}
