package server

import (
	"errors"
	"log"
	"net"
	"net/http"
	"time"

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

	ipResults, err := s.store.DistinctIPsForUser(name)
	if err != nil {
		log.Printf("DistinctIPsForUser error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	ips := make([]string, len(ipResults))
	lastSeenMap := make(map[string]string, len(ipResults))
	for i, ipResult := range ipResults {
		ips[i] = ipResult.IP
		lastSeenMap[ipResult.IP] = ipResult.LastSeen.Format(time.RFC3339)
	}

	cached, err := s.store.GetCachedGeos(ips)
	if err != nil {
		log.Printf("GetCachedGeos error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	locations := make([]models.GeoResult, 0, len(ips))
	for _, ipStr := range ips {
		geo := s.resolveGeo(ipStr, cached)
		if geo != nil {
			result := *geo
			lastSeen := lastSeenMap[ipStr]
			result.LastSeen = &lastSeen
			locations = append(locations, result)
		}
	}

	writeJSON(w, http.StatusOK, locations)
}

func (s *Server) resolveGeo(ipStr string, cached map[string]*models.GeoResult) *models.GeoResult {
	if geo, ok := cached[ipStr]; ok {
		return geo
	}
	if s.geoResolver == nil {
		return nil
	}
	ip := net.ParseIP(ipStr)
	geo := s.geoResolver.Lookup(ip)
	if geo != nil {
		if err := s.store.SetCachedGeo(geo); err != nil {
			log.Printf("caching geo for %s: %v", ipStr, err)
		}
	}
	return geo
}
