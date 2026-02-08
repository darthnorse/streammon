package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// isSecureRequest checks if the request came over HTTPS.
// SECURITY: Trusts X-Forwarded-Proto for reverse proxy setups.
// Ensure your reverse proxy strips/overwrites this header from clients.
func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func makeCookie(name, value, path string, maxAge int, r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	}
}

func clearCookie(name, path string, r *http.Request) *http.Cookie {
	return makeCookie(name, "", path, -1, r)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
