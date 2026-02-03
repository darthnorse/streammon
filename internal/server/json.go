package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encoding response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func isValidPathSegment(s string) bool {
	return !strings.Contains(s, "..") && !strings.Contains(s, "?") && !strings.Contains(s, "#")
}
