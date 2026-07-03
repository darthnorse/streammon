package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"streammon/internal/models"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encoding response: %v", err)
	}
}

func writeRawJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		log.Printf("writing response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseServerIDs(raw string) ([]int64, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, s := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid server_id: %s", s)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// tz offset bounds in minutes east of UTC. ±14:00 covers every real-world
// timezone (e.g. +14:00 Kiribati, -12:00 Baker Island).
const (
	tzOffsetMinMinutes = -840
	tzOffsetMaxMinutes = 840
)

// parseTZOffset reads the optional "tz_offset" query param (minutes east of
// UTC). Absent means 0 (UTC). Out-of-range values are clamped; a non-numeric
// value is an error.
func parseTZOffset(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("tz_offset")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid tz_offset: %s", raw)
	}
	if n < tzOffsetMinMinutes {
		n = tzOffsetMinMinutes
	} else if n > tzOffsetMaxMinutes {
		n = tzOffsetMaxMinutes
	}
	return n, nil
}

func viewerName(r *http.Request) string {
	user := UserFromContext(r.Context())
	if user != nil && user.Role == models.RoleViewer {
		return user.Name
	}
	return ""
}

func isValidPathSegment(s string) bool {
	return !strings.Contains(s, "..") && !strings.Contains(s, "?") && !strings.Contains(s, "#")
}
