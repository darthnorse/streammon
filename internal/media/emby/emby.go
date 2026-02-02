package emby

import (
	"streammon/internal/media/embybase"
	"streammon/internal/models"
)

type Server struct {
	*embybase.Client
}

func New(srv models.Server) *Server {
	return &Server{Client: embybase.New(srv, models.ServerTypeEmby)}
}
