package store

import (
	"strings"
	"testing"
)

func TestMapTileURLDefault(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	url, err := s.GetMapTileURL()
	if err != nil {
		t.Fatalf("GetMapTileURL: %v", err)
	}
	if url != DefaultMapTileURL {
		t.Fatalf("expected default %q, got %q", DefaultMapTileURL, url)
	}
	if DefaultMapTileURL != "https://tile.openstreetmap.org/{z}/{x}/{y}.png" {
		t.Fatalf("default tile URL must be the plain OSM host without subdomain sharding, got %q", DefaultMapTileURL)
	}
}

func TestMapTileURLRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	custom := "https://basemaps.cartocdn.com/dark_all/{z}/{x}/{y}.png?api_key=abc"
	if err := s.SetMapTileURL(custom); err != nil {
		t.Fatalf("SetMapTileURL: %v", err)
	}

	url, err := s.GetMapTileURL()
	if err != nil {
		t.Fatalf("GetMapTileURL: %v", err)
	}
	if url != custom {
		t.Fatalf("expected %q, got %q", custom, url)
	}
}

func TestMapTileURLEmptyResetsToDefault(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetMapTileURL("https://tiles.example.com/{z}/{x}/{y}.png"); err != nil {
		t.Fatalf("SetMapTileURL: %v", err)
	}
	if err := s.SetMapTileURL(""); err != nil {
		t.Fatalf("SetMapTileURL(\"\"): %v", err)
	}

	url, err := s.GetMapTileURL()
	if err != nil {
		t.Fatalf("GetMapTileURL: %v", err)
	}
	if url != DefaultMapTileURL {
		t.Fatalf("expected reset to default, got %q", url)
	}
}

func TestMapTileURLInvalidValues(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	cases := map[string]string{
		"missing z placeholder": "https://tiles.example.com/{x}/{y}.png",
		"missing x placeholder": "https://tiles.example.com/{z}/{y}.png",
		"missing y placeholder": "https://tiles.example.com/{z}/{x}.png",
		"plain http":            "http://tiles.example.com/{z}/{x}/{y}.png",
		"javascript scheme":     "javascript:alert({z}{x}{y})",
		"no host":               "https:///{z}/{x}/{y}.png",
		"not a url":             "{z}/{x}/{y}.png",
		"unknown placeholder":   "https://tiles.example.com/{z}/{x}/{y}.png?key={apikey}",
		"unknown token in host": "https://{region}.example.com/{z}/{x}/{y}.png",
	}

	for name, val := range cases {
		t.Run(name, func(t *testing.T) {
			if err := s.SetMapTileURL(val); err == nil {
				t.Fatalf("expected error for %q", val)
			}
			url, err := s.GetMapTileURL()
			if err != nil {
				t.Fatalf("GetMapTileURL: %v", err)
			}
			if url != DefaultMapTileURL {
				t.Fatalf("rejected value must not be stored, got %q", url)
			}
		})
	}
}

func TestIsValidTileURL(t *testing.T) {
	if !IsValidTileURL("https://tile.openstreetmap.org/{z}/{x}/{y}.png") {
		t.Fatal("expected OSM URL to be valid")
	}
	if !IsValidTileURL("https://{s}.example.com/{z}/{x}/{y}{r}.png") {
		t.Fatal("expected subdomain/retina placeholders to be allowed for custom providers")
	}
	if IsValidTileURL("http://tiles.example.com/{z}/{x}/{y}.png") {
		t.Fatal("expected http to be rejected")
	}
	// Leaflet's L.Util.template throws for any brace token it has no value
	// for, which would break every tile for every viewer.
	if IsValidTileURL("https://tiles.example.com/{z}/{x}/{y}.png?key={apikey}") {
		t.Fatal("expected an unknown {placeholder} to be rejected")
	}
	if !IsValidTileURL("https://tiles.example.com/{z}/{x}/{y}{r}.png") {
		t.Fatal("expected {r} to be allowed — Leaflet supplies it")
	}
}

func TestMapSettingLengthCap(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	longURL := "https://tiles.example.com/{z}/{x}/{y}.png?pad=" + strings.Repeat("a", 500)
	if err := s.SetMapTileURL(longURL); err == nil {
		t.Fatal("expected an over-long tile URL to be rejected")
	}
	if err := s.SetMapTileAttribution(strings.Repeat("a", 501)); err == nil {
		t.Fatal("expected an over-long attribution to be rejected")
	}
}

func TestMapTileAttributionDefaultEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	attr, err := s.GetMapTileAttribution()
	if err != nil {
		t.Fatalf("GetMapTileAttribution: %v", err)
	}
	if attr != "" {
		t.Fatalf("expected empty (client renders its own default), got %q", attr)
	}
}

func TestMapTileAttributionRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetMapTileAttribution("© CARTO, © OpenStreetMap contributors"); err != nil {
		t.Fatalf("SetMapTileAttribution: %v", err)
	}

	attr, err := s.GetMapTileAttribution()
	if err != nil {
		t.Fatalf("GetMapTileAttribution: %v", err)
	}
	if attr != "© CARTO, © OpenStreetMap contributors" {
		t.Fatalf("unexpected attribution %q", attr)
	}
}

func TestMapTileAttributionRejectsMarkup(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	for _, val := range []string{
		`<img src=x onerror=alert(1)>`,
		`© <a href="https://carto.com">CARTO</a>`,
		`plain > angle`,
	} {
		if err := s.SetMapTileAttribution(val); err == nil {
			t.Fatalf("expected markup %q to be rejected", val)
		}
	}

	attr, err := s.GetMapTileAttribution()
	if err != nil {
		t.Fatalf("GetMapTileAttribution: %v", err)
	}
	if attr != "" {
		t.Fatalf("rejected value must not be stored, got %q", attr)
	}
}

func TestMapDarkFilterDefaultTrue(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	on, err := s.GetMapDarkFilter()
	if err != nil {
		t.Fatalf("GetMapDarkFilter: %v", err)
	}
	if !on {
		t.Fatal("expected dark tile filter to default to enabled")
	}
}

func TestMapDarkFilterRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetMapDarkFilter(false); err != nil {
		t.Fatalf("SetMapDarkFilter: %v", err)
	}
	on, err := s.GetMapDarkFilter()
	if err != nil {
		t.Fatalf("GetMapDarkFilter: %v", err)
	}
	if on {
		t.Fatal("expected dark tile filter to be disabled")
	}

	if err := s.SetMapDarkFilter(true); err != nil {
		t.Fatalf("SetMapDarkFilter: %v", err)
	}
	on, err = s.GetMapDarkFilter()
	if err != nil {
		t.Fatalf("GetMapDarkFilter: %v", err)
	}
	if !on {
		t.Fatal("expected dark tile filter to be re-enabled")
	}
}
