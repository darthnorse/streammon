package plex

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"

	"streammon/internal/models"
)

type plexWSMessage struct {
	NotificationContainer notificationContainer `json:"NotificationContainer"`
}

type notificationContainer struct {
	Type                         string             `json:"type"`
	PlaySessionStateNotification []playSessionState `json:"PlaySessionStateNotification"`
}

type playSessionState struct {
	SessionKey string `json:"sessionKey"`
	RatingKey  string `json:"ratingKey"`
	State      string `json:"state"`
	ViewOffset int64  `json:"viewOffset"`
}

func (s *Server) Subscribe(ctx context.Context) (<-chan models.SessionUpdate, error) {
	ch := make(chan models.SessionUpdate, 16)
	go s.wsLoop(ctx, ch)
	return ch, nil
}

func (s *Server) wsLoop(ctx context.Context, ch chan<- models.SessionUpdate) {
	defer close(ch)
	backoff := time.Second

	for {
		connected := time.Now()
		err := s.wsConnect(ctx, ch)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("plex ws %s: %v", s.serverName, err)
		}
		// Reset backoff if the connection lasted more than 30 seconds
		if time.Since(connected) > 30*time.Second {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			backoff = min(backoff*2, 30*time.Second)
		}
	}
}

func (s *Server) wsConnect(ctx context.Context, ch chan<- models.SessionUpdate) error {
	u, err := url.Parse(s.url)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path += "/:/websockets/notifications"

	header := http.Header{"X-Plex-Token": {s.token}}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), header)
	if err != nil {
		return err
	}

	// Use a local context to stop the ping goroutine before closing the connection
	pingCtx, pingCancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteControl(
					websocket.PingMessage, nil,
					time.Now().Add(5*time.Second),
				); err != nil {
					return
				}
			}
		}
	}()

	defer func() {
		pingCancel()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		updates := parsePlexWSMessage(msg)
		for _, u := range updates {
			select {
			case ch <- u:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func parsePlexWSMessage(data []byte) []models.SessionUpdate {
	var msg plexWSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil
	}
	if msg.NotificationContainer.Type != "playing" {
		return nil
	}
	updates := make([]models.SessionUpdate, 0, len(msg.NotificationContainer.PlaySessionStateNotification))
	for _, ps := range msg.NotificationContainer.PlaySessionStateNotification {
		updates = append(updates, models.SessionUpdate{
			SessionKey: ps.SessionKey,
			RatingKey:  ps.RatingKey,
			State:      ps.State,
			ViewOffset: ps.ViewOffset,
		})
	}
	return updates
}
