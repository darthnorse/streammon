package server

import (
	"log"
	"net/http"
	"sync"
	"time"

	"streammon/internal/models"
)

type LibrariesResponse struct {
	Libraries []models.Library `json:"libraries"`
	Errors    []string         `json:"errors,omitempty"`
}

type libraryCache struct {
	mu        sync.RWMutex
	libraries []models.Library
	errors    []string
	expiresAt time.Time
}

const libraryCacheTTL = 5 * time.Minute

func (s *Server) handleGetLibraries(w http.ResponseWriter, r *http.Request) {
	if s.poller == nil {
		writeJSON(w, http.StatusOK, LibrariesResponse{Libraries: []models.Library{}})
		return
	}

	s.libCache.mu.RLock()
	if time.Now().UTC().Before(s.libCache.expiresAt) {
		resp := LibrariesResponse{
			Libraries: s.libCache.libraries,
			Errors:    s.libCache.errors,
		}
		s.libCache.mu.RUnlock()
		writeJSON(w, http.StatusOK, resp)
		return
	}
	s.libCache.mu.RUnlock()

	servers, err := s.store.ListServers()
	if err != nil {
		log.Printf("list servers: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	var allLibraries []models.Library
	var errors []string

	for _, srv := range servers {
		if r.Context().Err() != nil {
			break
		}

		if !srv.Enabled {
			continue
		}

		ms, ok := s.poller.GetServer(srv.ID)
		if !ok {
			continue
		}

		libs, err := ms.GetLibraries(r.Context())
		if err != nil {
			log.Printf("libraries from %s: %v", ms.Name(), err)
			errors = append(errors, ms.Name()+": "+err.Error())
			continue
		}

		allLibraries = append(allLibraries, libs...)
	}

	s.libCache.mu.Lock()
	s.libCache.libraries = allLibraries
	s.libCache.errors = errors
	s.libCache.expiresAt = time.Now().UTC().Add(libraryCacheTTL)
	s.libCache.mu.Unlock()

	writeJSON(w, http.StatusOK, LibrariesResponse{
		Libraries: allLibraries,
		Errors:    errors,
	})
}

func (s *Server) InvalidateLibraryCache() {
	s.libCache.mu.Lock()
	s.libCache.expiresAt = time.Time{}
	s.libCache.mu.Unlock()
}
