package media

import (
	"context"

	"streammon/internal/models"
)

type MediaServer interface {
	Name() string
	Type() models.ServerType
	GetSessions(ctx context.Context) ([]models.ActiveStream, error)
	TestConnection(ctx context.Context) error
}

// SessionUpdate represents a lightweight play state change from a WebSocket event.
type SessionUpdate struct {
	SessionKey string
	RatingKey  string
	State      string // "playing", "paused", "stopped", "buffering"
	ViewOffset int64  // progress in milliseconds
}

// RealtimeSubscriber is optionally implemented by adapters that support
// persistent WebSocket connections for real-time state updates.
type RealtimeSubscriber interface {
	Subscribe(ctx context.Context) (<-chan SessionUpdate, error)
}
