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

func TestReloadASNBadPath(t *testing.T) {
	r := NewResolver("")
	err := r.ReloadASN("/nonexistent/GeoLite2-ASN.mmdb")
	if err == nil {
		t.Fatal("expected error for nonexistent ASN database")
	}
}

func TestLookupWithoutASNReturnsEmptyISP(t *testing.T) {
	r := NewResolver("")
	if r.asnDB != nil {
		t.Fatal("asnDB should be nil when not loaded")
	}
}

func TestCloseHandlesNilDatabases(t *testing.T) {
	r := NewResolver("")
	// Should not panic when closing resolver with nil databases
	err := r.Close()
	if err != nil {
		t.Fatalf("Close should not error with nil databases: %v", err)
	}
}
