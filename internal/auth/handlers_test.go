package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/clientip"
)

func TestIsSecureRequest_XForwardedProtoRequiresTrustedPeer(t *testing.T) {
	trusted, err := clientip.ParseTrustedProxies("192.168.0.0/16", true)
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}

	run := func(remoteAddr string) bool {
		var got bool
		h := clientip.Middleware(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = isSecureRequest(r)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remoteAddr
		req.Header.Set("X-Forwarded-Proto", "https")
		h.ServeHTTP(httptest.NewRecorder(), req)
		return got
	}

	if !run("192.168.0.24:5555") {
		t.Error("X-Forwarded-Proto: https from a trusted proxy should mark the request secure")
	}
	if run("203.0.113.9:5555") {
		t.Error("X-Forwarded-Proto: https from an untrusted peer must be ignored")
	}
}
