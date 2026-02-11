package server

import (
	"log"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGeoIPLookup(w http.ResponseWriter, r *http.Request) {
	ipStr := chi.URLParam(r, "ip")
	ip := net.ParseIP(ipStr)
	if ip == nil {
		writeError(w, http.StatusBadRequest, "invalid IP")
		return
	}
	if s.geoResolver == nil {
		writeError(w, http.StatusNotFound, "no geo data")
		return
	}
	result := s.geoResolver.Lookup(ip)
	if result == nil {
		writeError(w, http.StatusNotFound, "no geo data")
		return
	}
	if err := s.store.SetCachedGeo(result); err != nil {
		log.Printf("caching geo for %s: %v", ipStr, err)
	}
	writeJSON(w, http.StatusOK, result)
}
