package embybase

import (
	"fmt"
	"net/http"
)

// SetAuth sets the MediaBrowser Authorization header. Jellyfin 12 rejects the
// legacy X-Emby-Token and X-Emby-Authorization headers; Emby accepts this form too.
// An empty token yields the client-only header used for AuthenticateByName.
func SetAuth(req *http.Request, serverID int64, token string) {
	v := fmt.Sprintf(`MediaBrowser Client="StreamMon", Device="StreamMon", DeviceId="streammon-%d", Version="1.0"`, serverID)
	if token != "" {
		v += fmt.Sprintf(`, Token=%q`, token)
	}
	req.Header.Set("Authorization", v)
}
