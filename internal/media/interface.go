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
	GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error)
	GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error)
}

// RealtimeSubscriber is optionally implemented by adapters that support
// persistent WebSocket connections for real-time state updates.
type RealtimeSubscriber interface {
	Subscribe(ctx context.Context) (<-chan models.SessionUpdate, error)
}
