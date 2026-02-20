package server

import (
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"streammon/internal/media"
	"streammon/internal/models"
	"streammon/internal/store"
)

func viewerCanAccessUser(r *http.Request, targetName string) bool {
	user := UserFromContext(r.Context())
	if user == nil {
		return false
	}
	if user.Role == models.RoleAdmin {
		return true
	}
	return user.Name == targetName
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	if user != nil && user.Role == models.RoleViewer {
		viewerUser, err := s.store.GetUser(user.Name)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				writeJSON(w, http.StatusOK, []models.User{})
				return
			}
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		writeJSON(w, http.StatusOK, []models.User{*viewerUser})
		return
	}

	users, err := s.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleListUserSummaries(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.store.ListUserSummaries()
	if err != nil {
		log.Printf("ListUserSummaries error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	user := UserFromContext(r.Context())
	if user != nil && user.Role == models.RoleViewer {
		filtered := make([]store.UserSummary, 0)
		for _, s := range summaries {
			if s.Name == user.Name {
				filtered = append(filtered, s)
				break
			}
		}
		writeJSON(w, http.StatusOK, filtered)
		return
	}

	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if !viewerCanAccessUser(r, name) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

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

	if !s.requireGuestVisibility(w, r, name, "visible_watch_history") {
		return
	}

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

func (s *Server) handleGetUserStats(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if !viewerCanAccessUser(r, name) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	user := UserFromContext(r.Context())
	if user != nil && user.Role != models.RoleAdmin {
		profileVisible, err := s.store.GetGuestSetting("visible_profile")
		if err != nil {
			log.Printf("GetGuestSetting error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		if !profileVisible {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	stats, err := s.store.UserDetailStats(r.Context(), name)
	if err != nil {
		log.Printf("UserDetailStats error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if stats.SessionCount == 0 {
		_, err := s.store.GetUser(name)
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		} else if err != nil {
			log.Printf("GetUser error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}

	if user != nil && user.Role != models.RoleAdmin {
		devicesVisible, err := s.store.GetGuestSetting("visible_devices")
		if err != nil {
			log.Printf("GetGuestSetting error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		ispsVisible, err := s.store.GetGuestSetting("visible_isps")
		if err != nil {
			log.Printf("GetGuestSetting error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		if !devicesVisible {
			stats.Devices = nil
		}
		if !ispsVisible {
			stats.ISPs = nil
		}
	}

	writeJSON(w, http.StatusOK, stats)
}

type SyncUserAvatarsResponse struct {
	store.SyncUserAvatarsResult
	Errors []string `json:"errors,omitempty"`
}

func (s *Server) handleSyncUserAvatars(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListServers()
	if err != nil {
		log.Printf("ListServers error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	response := SyncUserAvatarsResponse{}

	for _, srv := range servers {
		if !srv.Enabled {
			continue
		}

		ms, err := media.NewMediaServer(srv)
		if err != nil {
			response.Errors = append(response.Errors, srv.Name+": "+err.Error())
			continue
		}

		users, err := ms.GetUsers(r.Context())
		if err != nil {
			log.Printf("GetUsers from %s: %v", srv.Name, err)
			response.Errors = append(response.Errors, srv.Name+": "+err.Error())
			continue
		}

		result, err := s.store.SyncUsersFromServer(srv.ID, users)
		if err != nil {
			log.Printf("SyncUsersFromServer %s: %v", srv.Name, err)
			response.Errors = append(response.Errors, srv.Name+": "+err.Error())
			continue
		}

		response.Synced += result.Synced
		response.Updated += result.Updated
	}

	writeJSON(w, http.StatusOK, response)
}
