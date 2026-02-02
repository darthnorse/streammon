package server

import (
	"errors"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"

	"streammon/internal/models"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	user, err := s.store.GetUser(name)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleGetUserLocations(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	ips, err := s.store.DistinctIPsForUser(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	locations := []models.GeoResult{}
	for _, ipStr := range ips {
		cached, err := s.store.GetCachedGeo(ipStr)
		if err == nil && cached != nil {
			locations = append(locations, *cached)
			continue
		}
		if s.geoResolver != nil {
			ip := net.ParseIP(ipStr)
			if geo := s.geoResolver.Lookup(ip); geo != nil {
				s.store.SetCachedGeo(geo)
				locations = append(locations, *geo)
			}
		}
	}

	writeJSON(w, http.StatusOK, locations)
}
