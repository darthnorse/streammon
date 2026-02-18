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

// parseServerIDs parses a comma-separated list of int64 server IDs.
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

// viewerName returns the viewer's name if the request is from a viewer, or empty string otherwise.
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
