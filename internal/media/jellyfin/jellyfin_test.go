package jellyfin

import (
	"context"
	"testing"

	"streammon/internal/models"
)

type mediaServer interface {
	Name() string
	Type() models.ServerType
	GetSessions(ctx context.Context) ([]models.ActiveStream, error)
	TestConnection(ctx context.Context) error
}

func TestImplementsMediaServer(t *testing.T) {
	var _ mediaServer = (*Server)(nil)
}

func TestType(t *testing.T) {
	s := New(models.Server{})
	if s.Type() != models.ServerTypeJellyfin {
		t.Errorf("type = %q, want jellyfin", s.Type())
	}
}
