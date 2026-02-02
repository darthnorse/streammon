package media

import (
	"fmt"

	"streammon/internal/media/emby"
	"streammon/internal/media/jellyfin"
	"streammon/internal/media/plex"
	"streammon/internal/models"
)

func NewMediaServer(srv models.Server) (MediaServer, error) {
	switch srv.Type {
	case models.ServerTypePlex:
		return plex.New(srv), nil
	case models.ServerTypeEmby:
		return emby.New(srv), nil
	case models.ServerTypeJellyfin:
		return jellyfin.New(srv), nil
	default:
		return nil, fmt.Errorf("unsupported server type: %s", srv.Type)
	}
}
