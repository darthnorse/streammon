package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"

	"streammon/internal/store"
)

func oidcMigrationsDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "migrations")
}

func newOIDCTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	dir := oidcMigrationsDir()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	if err := s.Migrate(dir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

// oidcTestEnv wires a mock OIDC identity provider (discovery + JWKS + token
// endpoint) to a real *OIDCProvider so HandleLogin/HandleCallback can be
// exercised end-to-end against real, signed ID tokens.
type oidcTestEnv struct {
	provider *OIDCProvider
	store    *store.Store
	srv      *httptest.Server
	priv     *rsa.PrivateKey
	keyID    string
	clientID string
	issuer   string

	nextIDToken string // raw ID token the mock /token endpoint hands back next
}

func newOIDCTestEnv(t *testing.T) *oidcTestEnv {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	env := &oidcTestEnv{
		priv:     priv,
		keyID:    "test-key",
		clientID: "test-client",
	}

	discovery := &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{
			{PublicKey: priv.Public(), KeyID: env.keyID, Algorithm: gooidc.RS256},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     env.nextIDToken,
		})
	})
	mux.Handle("/", discovery)

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	discovery.SetIssuer(srv.URL)
	env.srv = srv
	env.issuer = srv.URL

	env.store = newOIDCTestStore(t)
	mgr := NewManager(env.store)

	p := &OIDCProvider{store: env.store, manager: mgr}
	if err := p.Reload(env.clientCtx(), Config{
		Issuer:       srv.URL,
		ClientID:     env.clientID,
		ClientSecret: "test-secret",
		RedirectURL:  "https://app.example.com/callback",
	}); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	env.provider = p

	return env
}

// clientCtx carries the HTTP client that trusts the mock IdP's TLS cert;
// it's needed both for discovery and for the token exchange in HandleCallback.
func (e *oidcTestEnv) clientCtx() context.Context {
	return gooidc.ClientContext(context.Background(), e.srv.Client())
}

// signIDToken mints a signed ID token for the mock IdP. An empty nonce omits
// the nonce claim entirely (simulating an IdP/attacker response with no nonce).
func (e *oidcTestEnv) signIDToken(t *testing.T, nonce string) string {
	t.Helper()
	now := time.Now().UTC()
	nonceClaim := ""
	if nonce != "" {
		nonceClaim = fmt.Sprintf(`, "nonce": %q`, nonce)
	}
	claims := fmt.Sprintf(`{
		"iss": %q,
		"aud": %q,
		"sub": "user-123",
		"exp": %d,
		"iat": %d,
		"email": "user@example.com",
		"email_verified": true,
		"name": "Test User"%s
	}`, e.issuer, e.clientID, now.Add(time.Hour).Unix(), now.Unix(), nonceClaim)
	return oidctest.SignIDToken(e.priv, e.keyID, gooidc.RS256, claims)
}

// startLogin drives HandleLogin and returns the redirect URL sent to the IdP
// plus the state/nonce cookies it set on the response.
func (e *oidcTestEnv) startLogin(t *testing.T) (redirectURL *url.URL, stateCookie, nonceCookie *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil)
	w := httptest.NewRecorder()
	e.provider.HandleLogin(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("HandleLogin: expected 302, got %d", res.StatusCode)
	}
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parsing redirect location: %v", err)
	}
	for _, c := range res.Cookies() {
		switch c.Name {
		case stateCookieName:
			stateCookie = c
		case nonceCookieName:
			nonceCookie = c
		}
	}
	return loc, stateCookie, nonceCookie
}

func (e *oidcTestEnv) callback(t *testing.T, state string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?state="+url.QueryEscape(state)+"&code=test-code", nil)
	req = req.WithContext(e.clientCtx())
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	w := httptest.NewRecorder()
	e.provider.HandleCallback(w, req)
	return w
}

func TestHandleLogin_SetsNonceCookieAndAuthURLParam(t *testing.T) {
	env := newOIDCTestEnv(t)
	loc, stateCookie, nonceCookie := env.startLogin(t)

	if stateCookie == nil {
		t.Fatal("expected state cookie to be set")
	}
	if nonceCookie == nil {
		t.Fatal("expected nonce cookie to be set")
	}
	if nonceCookie.Value == "" {
		t.Fatal("expected non-empty nonce cookie value")
	}
	if !nonceCookie.HttpOnly {
		t.Error("expected nonce cookie to be HttpOnly")
	}
	if nonceCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", nonceCookie.SameSite)
	}
	if nonceCookie.Value == stateCookie.Value {
		t.Error("nonce and state should be independently generated, not equal")
	}

	gotNonce := loc.Query().Get("nonce")
	if gotNonce != nonceCookie.Value {
		t.Errorf("AuthCodeURL nonce param = %q, want cookie value %q", gotNonce, nonceCookie.Value)
	}
}

func TestHandleCallback_NonceMatch_Succeeds(t *testing.T) {
	env := newOIDCTestEnv(t)
	if err := env.store.SetGuestAccess(true); err != nil {
		t.Fatalf("SetGuestAccess: %v", err)
	}
	_, stateCookie, nonceCookie := env.startLogin(t)

	env.nextIDToken = env.signIDToken(t, nonceCookie.Value)

	w := env.callback(t, stateCookie.Value, []*http.Cookie{stateCookie, nonceCookie})
	res := w.Result()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", res.StatusCode, w.Body.String())
	}
	if loc := res.Header.Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %q", loc)
	}

	var sawSession, clearedState, clearedNonce bool
	for _, c := range res.Cookies() {
		switch c.Name {
		case CookieName:
			sawSession = c.Value != ""
		case stateCookieName:
			clearedState = c.MaxAge < 0
		case nonceCookieName:
			clearedNonce = c.MaxAge < 0
		}
	}
	if !sawSession {
		t.Error("expected session cookie to be set on success")
	}
	if !clearedState {
		t.Error("expected state cookie to be cleared on success")
	}
	if !clearedNonce {
		t.Error("expected nonce cookie to be cleared on success")
	}
}

func TestHandleCallback_NonceMismatch_Rejected(t *testing.T) {
	env := newOIDCTestEnv(t)
	_, stateCookie, nonceCookie := env.startLogin(t)

	env.nextIDToken = env.signIDToken(t, "attacker-supplied-nonce")

	w := env.callback(t, stateCookie.Value, []*http.Cookie{stateCookie, nonceCookie})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			t.Error("session cookie must not be set when nonce mismatches")
		}
	}
}

func TestHandleCallback_NonceMissingFromToken_Rejected(t *testing.T) {
	env := newOIDCTestEnv(t)
	_, stateCookie, nonceCookie := env.startLogin(t)

	env.nextIDToken = env.signIDToken(t, "") // IdP returns a token with no nonce claim

	w := env.callback(t, stateCookie.Value, []*http.Cookie{stateCookie, nonceCookie})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCallback_NonceCookieMissing_Rejected(t *testing.T) {
	env := newOIDCTestEnv(t)
	_, stateCookie, nonceCookie := env.startLogin(t)

	env.nextIDToken = env.signIDToken(t, nonceCookie.Value)

	// Only the state cookie made it back - e.g. the nonce cookie was
	// stripped or never set. Must not be treated as a valid login.
	w := env.callback(t, stateCookie.Value, []*http.Cookie{stateCookie})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCallback_StateMismatch_StillRejected(t *testing.T) {
	env := newOIDCTestEnv(t)
	_, stateCookie, nonceCookie := env.startLogin(t)

	env.nextIDToken = env.signIDToken(t, nonceCookie.Value)

	w := env.callback(t, "wrong-state", []*http.Cookie{stateCookie, nonceCookie})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
