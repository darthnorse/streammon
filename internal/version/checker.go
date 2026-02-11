package version

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultReleaseAPI = "https://api.github.com/repos/darthnorse/streammon/releases/latest"

// Info holds version information returned by the API.
type Info struct {
	Current         string `json:"version"`
	Latest          string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
}

type gitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Checker polls GitHub for the latest release and compares against the current version.
type Checker struct {
	current    string
	releaseAPI string
	client     *http.Client

	mu         sync.RWMutex
	latest     string
	releaseURL string
}

// NewChecker creates a new version checker for the given current version.
// Set VERSION_CHECK_URL to override the GitHub API endpoint (useful for testing).
func NewChecker(currentVersion string) *Checker {
	api := defaultReleaseAPI
	if u := os.Getenv("VERSION_CHECK_URL"); u != "" {
		api = u
	}
	return &Checker{
		current:    strings.TrimPrefix(currentVersion, "v"),
		releaseAPI: api,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Start begins periodic version checks. It checks immediately, then every 6 hours.
// Blocks until ctx is cancelled.
func (c *Checker) Start(ctx context.Context) {
	c.check(ctx)
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.check(ctx)
		}
	}
}

// Info returns the current version state.
func (c *Checker) Info() Info {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := Info{
		Current: c.current,
	}
	if c.latest != "" {
		info.Latest = c.latest
		info.ReleaseURL = c.releaseURL
		if c.current != "dev" && compareSemver(c.latest, c.current) > 0 {
			info.UpdateAvailable = true
		}
	}
	return info
}

// compareSemver compares two dotted version strings numerically.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// Pre-release suffixes (e.g. "1.0.0-rc1") are stripped before comparison.
func compareSemver(a, b string) int {
	a = stripPreRelease(a)
	b = stripPreRelease(b)
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(aParts) {
			av, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bv, _ = strconv.Atoi(bParts[i])
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func stripPreRelease(v string) string {
	if i := strings.IndexAny(v, "-+"); i != -1 {
		return v[:i]
	}
	return v
}

func (c *Checker) check(ctx context.Context) {
	if c.current == "dev" {
		return
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.releaseAPI, nil)
	if err != nil {
		log.Printf("version check: %v", err)
		return
	}
	req.Header.Set("User-Agent", "StreamMon/"+c.current)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("version check: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("version check: GitHub returned %d", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		log.Printf("version check: read error: %v", err)
		return
	}

	var release gitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		log.Printf("version check: parse error: %v", err)
		return
	}

	latest := strings.TrimPrefix(release.TagName, "v")

	c.mu.Lock()
	c.latest = latest
	c.releaseURL = release.HTMLURL
	c.mu.Unlock()
}
