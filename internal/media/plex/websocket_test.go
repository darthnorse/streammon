package plex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"streammon/internal/models"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func TestSubscribeReceivesPlayingEvent(t *testing.T) {
	srv := startWSServer(t, func(conn *websocket.Conn) {
		msg := plexWSMessage{
			NotificationContainer: notificationContainer{
				Type: "playing",
				PlaySessionStateNotification: []playSessionState{
					{SessionKey: "10", RatingKey: "500", State: "playing", ViewOffset: 12345},
				},
			},
		}
		data, _ := json.Marshal(msg)
		conn.WriteMessage(websocket.TextMessage, data)
		// Keep connection open briefly
		time.Sleep(200 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := srv.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case u := <-ch:
		if u.SessionKey != "10" {
			t.Errorf("session key = %q, want 10", u.SessionKey)
		}
		if u.RatingKey != "500" {
			t.Errorf("rating key = %q, want 500", u.RatingKey)
		}
		if u.State != "playing" {
			t.Errorf("state = %q, want playing", u.State)
		}
		if u.ViewOffset != 12345 {
			t.Errorf("view offset = %d, want 12345", u.ViewOffset)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for update")
	}
	cancel()
}

func TestSubscribeIgnoresNonPlayingEvents(t *testing.T) {
	srv := startWSServer(t, func(conn *websocket.Conn) {
		// Send a non-playing event
		msg := plexWSMessage{
			NotificationContainer: notificationContainer{
				Type: "timeline",
			},
		}
		data, _ := json.Marshal(msg)
		conn.WriteMessage(websocket.TextMessage, data)

		// Then send a playing event
		msg2 := plexWSMessage{
			NotificationContainer: notificationContainer{
				Type: "playing",
				PlaySessionStateNotification: []playSessionState{
					{SessionKey: "1", State: "paused", ViewOffset: 999},
				},
			},
		}
		data2, _ := json.Marshal(msg2)
		conn.WriteMessage(websocket.TextMessage, data2)
		time.Sleep(200 * time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := srv.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case u := <-ch:
		if u.State != "paused" {
			t.Errorf("expected paused, got %s", u.State)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}
	cancel()
}

func TestSubscribeStopsOnContextCancel(t *testing.T) {
	srv := startWSServer(t, func(conn *websocket.Conn) {
		// Keep connection open
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := srv.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cancel()

	// Channel should eventually close
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case _, ok := <-ch:
		if ok {
			// Got a value, keep draining
			for range ch {
			}
		}
	case <-timer.C:
		t.Fatal("channel not closed after context cancel")
	}
}

func TestSubscribeReconnectsOnClose(t *testing.T) {
	connectCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		connectCount++
		if connectCount == 1 {
			// Close immediately to trigger reconnect
			conn.Close()
			return
		}
		// Second connection: send an event
		msg := plexWSMessage{
			NotificationContainer: notificationContainer{
				Type: "playing",
				PlaySessionStateNotification: []playSessionState{
					{SessionKey: "reconnected", State: "playing", ViewOffset: 1},
				},
			},
		}
		data, _ := json.Marshal(msg)
		conn.WriteMessage(websocket.TextMessage, data)
		time.Sleep(200 * time.Millisecond)
		conn.Close()
	}))
	t.Cleanup(ts.Close)

	srv := New(models.Server{
		URL:    ts.URL,
		APIKey: "tok",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := srv.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case u := <-ch:
		if u.SessionKey != "reconnected" {
			t.Errorf("session key = %q, want reconnected", u.SessionKey)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for reconnect event")
	}

	if connectCount < 2 {
		t.Errorf("expected at least 2 connections, got %d", connectCount)
	}
	cancel()
}

// Verify interface satisfaction is checked via usage in poller tests.

func startWSServer(t *testing.T, handler func(*websocket.Conn)) *Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/:/websockets/notifications") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Errorf("missing or wrong token: %s", r.Header.Get("X-Plex-Token"))
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	t.Cleanup(ts.Close)

	return New(models.Server{
		URL:    ts.URL,
		APIKey: "test-token",
	})
}
