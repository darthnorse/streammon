package version

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInfo_InitialState(t *testing.T) {
	c := NewChecker("1.0.0")
	info := c.Info()
	if info.Current != "1.0.0" {
		t.Fatalf("expected current=1.0.0, got %s", info.Current)
	}
	if info.UpdateAvailable {
		t.Fatal("expected no update available initially")
	}
	if info.Latest != "" {
		t.Fatalf("expected empty latest, got %s", info.Latest)
	}
}

func TestInfo_DevVersion(t *testing.T) {
	c := NewChecker("dev")
	info := c.Info()
	if info.Current != "dev" {
		t.Fatalf("expected current=dev, got %s", info.Current)
	}
}

func TestNewChecker_StripsVPrefix(t *testing.T) {
	c := NewChecker("v1.2.3")
	info := c.Info()
	if info.Current != "1.2.3" {
		t.Fatalf("expected current=1.2.3, got %s", info.Current)
	}
}

func TestCompare_NewerAvailable(t *testing.T) {
	c := NewChecker("1.0.0")
	c.mu.Lock()
	c.latest = "1.1.0"
	c.releaseURL = "https://github.com/example/releases/tag/v1.1.0"
	c.mu.Unlock()

	info := c.Info()
	if !info.UpdateAvailable {
		t.Fatal("expected update available")
	}
	if info.Latest != "1.1.0" {
		t.Fatalf("expected latest=1.1.0, got %s", info.Latest)
	}
	if info.ReleaseURL != "https://github.com/example/releases/tag/v1.1.0" {
		t.Fatalf("unexpected release URL: %s", info.ReleaseURL)
	}
}

func TestCompare_SameVersion(t *testing.T) {
	c := NewChecker("1.0.0")
	c.mu.Lock()
	c.latest = "1.0.0"
	c.mu.Unlock()

	info := c.Info()
	if info.UpdateAvailable {
		t.Fatal("expected no update when versions are the same")
	}
}

func TestCompare_OlderAvailable(t *testing.T) {
	c := NewChecker("2.0.0")
	c.mu.Lock()
	c.latest = "1.0.0"
	c.mu.Unlock()

	info := c.Info()
	if info.UpdateAvailable {
		t.Fatal("expected no update when running newer version")
	}
}

func TestCompare_DevSkipped(t *testing.T) {
	c := NewChecker("dev")
	c.mu.Lock()
	c.latest = "1.0.0"
	c.mu.Unlock()

	info := c.Info()
	if info.UpdateAvailable {
		t.Fatal("expected no update for dev version")
	}
}

func TestCompare_MultiDigitVersions(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"1.9.0", "1.10.0", true},
		{"1.10.0", "1.9.0", false},
		{"2.0.0", "10.0.0", true},
		{"10.0.0", "2.0.0", false},
		{"0.9.9", "0.10.0", true},
		{"1.0.0", "1.0.1", true},
		{"1.0.10", "1.0.9", false},
	}
	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.latest, func(t *testing.T) {
			c := NewChecker(tt.current)
			c.mu.Lock()
			c.latest = tt.latest
			c.mu.Unlock()

			info := c.Info()
			if info.UpdateAvailable != tt.want {
				t.Fatalf("current=%s latest=%s: got UpdateAvailable=%v, want %v",
					tt.current, tt.latest, info.UpdateAvailable, tt.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.9.0", "1.10.0", -1},
		{"1.10.0", "1.9.0", 1},
		{"2.0.0", "10.0.0", -1},
		{"10.0.0", "2.0.0", 1},
		{"1.0.0-rc1", "1.0.0", 0},
		{"1.0.0", "1.0.1-beta", -1},
		{"1.0.0-rc1", "1.0.1-rc2", -1},
		{"1.2.0+build123", "1.2.0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareSemver(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCheck_HTTPMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != "StreamMon/1.0.0" {
			t.Errorf("expected User-Agent=StreamMon/1.0.0, got %s", ua)
		}
		resp := gitHubRelease{
			TagName: "v2.0.0",
			HTMLURL: "https://github.com/example/releases/tag/v2.0.0",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewChecker("1.0.0")
	c.releaseAPI = srv.URL

	c.check(context.Background())

	info := c.Info()
	if !info.UpdateAvailable {
		t.Fatal("expected update available after check")
	}
	if info.Latest != "2.0.0" {
		t.Fatalf("expected latest=2.0.0, got %s", info.Latest)
	}
	if info.ReleaseURL != "https://github.com/example/releases/tag/v2.0.0" {
		t.Fatalf("unexpected release URL: %s", info.ReleaseURL)
	}
}

func TestCheck_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewChecker("1.0.0")
	c.releaseAPI = srv.URL

	c.check(context.Background())

	info := c.Info()
	if info.UpdateAvailable {
		t.Fatal("expected no update on HTTP error")
	}
}

func TestCheck_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>error page</html>"))
	}))
	defer srv.Close()

	c := NewChecker("1.0.0")
	c.releaseAPI = srv.URL

	c.check(context.Background())

	info := c.Info()
	if info.UpdateAvailable {
		t.Fatal("expected no update on malformed JSON")
	}
	if info.Latest != "" {
		t.Fatalf("expected empty latest, got %s", info.Latest)
	}
}

func TestCheck_DevSkipsHTTP(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := NewChecker("dev")
	c.releaseAPI = srv.URL

	c.check(context.Background())

	if called {
		t.Fatal("expected dev version to skip HTTP check")
	}
}

func TestStart_Cancellation(t *testing.T) {
	c := NewChecker("dev")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}
