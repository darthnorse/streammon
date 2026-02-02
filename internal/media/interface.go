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
