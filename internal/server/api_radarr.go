package server

import (
	"streammon/internal/radarr"
)

func (s *Server) radarrDeps() integrationDeps {
	return integrationDeps{
		validateURL:  radarr.ValidateURL,
		newClient:    func(url, apiKey string) (integrationClient, error) { return radarr.NewClient(url, apiKey) },
		getConfig:    s.store.GetRadarrConfig,
		setConfig:    s.store.SetRadarrConfig,
		deleteConfig: s.store.DeleteRadarrConfig,
	}
}
