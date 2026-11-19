package embybase

import (
	"net/http"
	"strings"
	"testing"
)

func TestSetAuth(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"with token", "abc123", `MediaBrowser Client="StreamMon", Device="StreamMon", DeviceId="streammon-7", Version="1.0", Token="abc123"`},
		{"login without token", "", `MediaBrowser Client="StreamMon", Device="StreamMon", DeviceId="streammon-7", Version="1.0"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
			SetAuth(req, 7, tt.token)
			if got := req.Header.Get("Authorization"); got != tt.want {
				t.Errorf("Authorization = %q, want %q", got, tt.want)
			}
			for _, legacy := range []string{"X-Emby-Token", "X-Emby-Authorization"} {
				if v := req.Header.Get(legacy); v != "" {
					t.Errorf("legacy header %s set to %q", legacy, v)
				}
			}
		})
	}
}

func hasToken(r *http.Request, token string) bool {
	return strings.HasSuffix(r.Header.Get("Authorization"), `Token="`+token+`"`)
}
