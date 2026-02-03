package server

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

const (
	FilterDaysWeek  = 7
	FilterDaysMonth = 30
)

type StatsResponse struct {
	TopMovies        []models.MediaStat   `json:"top_movies"`
	TopTVShows       []models.MediaStat   `json:"top_tv_shows"`
	TopUsers         []models.UserStat    `json:"top_users"`
	Library          *models.LibraryStat  `json:"library"`
	ConcurrentPeak   int                  `json:"concurrent_peak"`
	ConcurrentPeakAt string               `json:"concurrent_peak_at,omitempty"`
	Locations        []models.GeoResult   `json:"locations"`
	PotentialSharers []models.SharerAlert `json:"potential_sharers"`
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	days := 0 // default: all time. Valid values: 7, 30, or 0/omitted (all time)
	if d := r.URL.Query().Get("days"); d != "" {
		parsed, err := strconv.Atoi(d)
		if err != nil {
			writeError(w, http.StatusBadRequest, "days must be a number")
			return
		}
		if parsed != 0 && parsed != FilterDaysWeek && parsed != FilterDaysMonth {
			writeError(w, http.StatusBadRequest, "days must be 0, 7, or 30")
			return
		}
		days = parsed
	}

	resp := StatsResponse{}
	var err error

	resp.TopMovies, err = s.store.TopMovies(10, days)
	if err != nil {
		log.Printf("TopMovies error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	resp.TopTVShows, err = s.store.TopTVShows(10, days)
	if err != nil {
		log.Printf("TopTVShows error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	resp.TopUsers, err = s.store.TopUsers(10, days)
	if err != nil {
		log.Printf("TopUsers error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	resp.Library, err = s.store.LibraryStats()
	if err != nil {
		log.Printf("LibraryStats error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	peak, peakAt, err := s.store.ConcurrentStreamsPeak()
	if err != nil {
		log.Printf("ConcurrentStreamsPeak error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp.ConcurrentPeak = peak
	if !peakAt.IsZero() {
		resp.ConcurrentPeakAt = peakAt.Format(time.RFC3339)
	}

	resp.Locations, err = s.store.AllWatchLocations()
	if err != nil {
		log.Printf("AllWatchLocations error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	resp.PotentialSharers, err = s.store.PotentialSharers(store.DefaultSharerMinIPs, store.DefaultSharerWindowDays)
	if err != nil {
		log.Printf("PotentialSharers error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
