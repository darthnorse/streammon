package server

import (
	"encoding/json"
	"net/http"

	"streammon/internal/store"
	"streammon/internal/units"
)

type displaySettingsResponse struct {
	UnitSystem     string `json:"unit_system"`
	DiscoverRegion string `json:"discover_region"`
	// MapTileURL is always populated — the store substitutes its default when
	// no override is configured. MapTileAttribution is the override only:
	// empty means the client renders the credit for its default basemap.
	MapTileURL         string `json:"map_tile_url"`
	MapTileAttribution string `json:"map_tile_attribution"`
	MapDarkFilter      bool   `json:"map_dark_filter"`
}

type displaySettingsRequest struct {
	UnitSystem         string  `json:"unit_system"`
	DiscoverRegion     *string `json:"discover_region,omitempty"`
	MapTileURL         *string `json:"map_tile_url,omitempty"`
	MapTileAttribution *string `json:"map_tile_attribution,omitempty"`
	MapDarkFilter      *bool   `json:"map_dark_filter,omitempty"`
}

func (s *Server) readDisplaySettings() (displaySettingsResponse, error) {
	var resp displaySettingsResponse
	var err error

	if resp.UnitSystem, err = s.store.GetUnitSystem(); err != nil {
		return resp, err
	}
	if resp.DiscoverRegion, err = s.store.GetDiscoverRegion(); err != nil {
		return resp, err
	}
	if resp.MapTileURL, err = s.store.GetMapTileURL(); err != nil {
		return resp, err
	}
	if resp.MapTileAttribution, err = s.store.GetMapTileAttribution(); err != nil {
		return resp, err
	}
	if resp.MapDarkFilter, err = s.store.GetMapDarkFilter(); err != nil {
		return resp, err
	}
	return resp, nil
}

func (s *Server) handleGetDisplaySettings(w http.ResponseWriter, r *http.Request) {
	resp, err := s.readDisplaySettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateDisplaySettings(w http.ResponseWriter, r *http.Request) {
	var req displaySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.UnitSystem != "" {
		if !units.IsValid(req.UnitSystem) {
			writeError(w, http.StatusBadRequest, "invalid unit system")
			return
		}
		if err := s.store.SetUnitSystem(req.UnitSystem); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}

	if req.DiscoverRegion != nil {
		region := *req.DiscoverRegion
		if region != "" && !store.IsValidRegionCode(region) {
			writeError(w, http.StatusBadRequest, "invalid region code")
			return
		}
		if err := s.store.SetDiscoverRegion(region); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}

	if req.MapTileURL != nil {
		tileURL := *req.MapTileURL
		if tileURL != "" && !store.IsValidTileURL(tileURL) {
			writeError(w, http.StatusBadRequest, "invalid tile URL")
			return
		}
		if err := s.store.SetMapTileURL(tileURL); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}

	if req.MapTileAttribution != nil {
		if !store.IsValidTileAttribution(*req.MapTileAttribution) {
			writeError(w, http.StatusBadRequest, "invalid tile attribution")
			return
		}
		if err := s.store.SetMapTileAttribution(*req.MapTileAttribution); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}

	if req.MapDarkFilter != nil {
		if err := s.store.SetMapDarkFilter(*req.MapDarkFilter); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}

	resp, err := s.readDisplaySettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
