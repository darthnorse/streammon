package media

import (
	"context"

	"streammon/internal/models"
)

type MediaServer interface {
	Name() string
	Type() models.ServerType
	ServerID() int64
	GetSessions(ctx context.Context) ([]models.ActiveStream, error)
	TestConnection(ctx context.Context) error
	GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error)
	GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error)
	GetLibraries(ctx context.Context) ([]models.Library, error)
	GetUsers(ctx context.Context) ([]models.MediaUser, error)
	GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error)
	DeleteItem(ctx context.Context, itemID string) error
	GetSeasons(ctx context.Context, showID string) ([]models.Season, error)
}

// RealtimeSubscriber is optionally implemented by adapters that support
// persistent WebSocket connections for real-time state updates.
type RealtimeSubscriber interface {
	Subscribe(ctx context.Context) (<-chan models.SessionUpdate, error)
}
