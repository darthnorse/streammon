package geoip

import (
	"net"
	"testing"
)

func TestLookupNilWhenNoDB(t *testing.T) {
	r := NewResolver("")
	result := r.Lookup(net.ParseIP("8.8.8.8"))
	if result != nil {
		t.Fatal("expected nil when DB path is empty")
	}
}

func TestLookupNilForPrivateIP(t *testing.T) {
	r := NewResolver("")
	result := r.Lookup(net.ParseIP("192.168.1.1"))
	if result != nil {
		t.Fatal("expected nil for private IP")
	}
}

func TestLookupNilForNilIP(t *testing.T) {
	r := NewResolver("")
	result := r.Lookup(nil)
	if result != nil {
		t.Fatal("expected nil for nil IP")
	}
}

func TestLookupNilForBadDBPath(t *testing.T) {
	r := NewResolver("/nonexistent/GeoLite2-City.mmdb")
	result := r.Lookup(net.ParseIP("8.8.8.8"))
	if result != nil {
		t.Fatal("expected nil when DB file doesn't exist")
	}
}

func TestNewResolverEmptyPath(t *testing.T) {
	r := NewResolver("")
	if r == nil {
		t.Fatal("resolver should never be nil")
	}
}
