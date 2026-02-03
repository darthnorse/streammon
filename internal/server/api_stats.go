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

	logAndFail := func(name string, err error) bool {
		if err != nil {
			log.Printf("%s error: %v", name, err)
			writeError(w, http.StatusInternalServerError, "internal")
			return true
		}
		return false
	}

	var err error
	if resp.TopMovies, err = s.store.TopMovies(10, days); logAndFail("TopMovies", err) {
		return
	}
	if resp.TopTVShows, err = s.store.TopTVShows(10, days); logAndFail("TopTVShows", err) {
		return
	}
	if resp.TopUsers, err = s.store.TopUsers(10, days); logAndFail("TopUsers", err) {
		return
	}
	if resp.Library, err = s.store.LibraryStats(); logAndFail("LibraryStats", err) {
		return
	}

	var peakAt time.Time
	if resp.ConcurrentPeak, peakAt, err = s.store.ConcurrentStreamsPeak(); logAndFail("ConcurrentStreamsPeak", err) {
		return
	}
	if !peakAt.IsZero() {
		resp.ConcurrentPeakAt = peakAt.Format(time.RFC3339)
	}

	if resp.Locations, err = s.store.AllWatchLocations(); logAndFail("AllWatchLocations", err) {
		return
	}
	if resp.PotentialSharers, err = s.store.PotentialSharers(store.DefaultSharerMinIPs, store.DefaultSharerWindowDays); logAndFail("PotentialSharers", err) {
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
