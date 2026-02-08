package server

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
	"streammon/internal/store"
)

// testSessionToken is the session token for the test admin user
var testSessionToken string

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, f, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(f), "..", "..", "migrations")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	if err := s.Migrate(dir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	// Create a test admin user and session for authenticated requests
	user, err := s.CreateLocalUser("test-admin", "admin@test.local", "", models.RoleAdmin)
	if err != nil {
		t.Fatalf("creating test admin: %v", err)
	}
	token, err := s.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("creating test session: %v", err)
	}
	testSessionToken = token

	authMgr := auth.NewManager(s)
	authMgr.RegisterProvider(auth.NewLocalProvider(s, authMgr))
	srv := NewServer(s, WithAuthManager(authMgr))
	return srv, s
}

// addAuthCookie adds the test session cookie to a request
func addAuthCookie(r *http.Request) {
	r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: testSessionToken})
}

// testServer wraps Server to automatically add auth cookies in tests
type testServer struct {
	*Server
}

func (ts *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addAuthCookie(r)
	ts.Server.ServeHTTP(w, r)
}

// Unwrap returns the underlying Server for tests that need direct access
func (ts *testServer) Unwrap() *Server {
	return ts.Server
}

func newTestServerWrapped(t *testing.T) (*testServer, *store.Store) {
	srv, s := newTestServer(t)
	return &testServer{srv}, s
}

// newEmptyStore creates a store with migrations but no users.
// Use this for tests that need to verify behavior when no users exist.
func newEmptyStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, f, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(f), "..", "..", "migrations")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	if err := s.Migrate(dir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
