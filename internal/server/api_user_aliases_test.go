package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"streammon/internal/auth"
	"streammon/internal/models"
)

type aliasesResp struct {
	Aliases           []string `json:"aliases"`
	WatchHistoryMoved int      `json:"watch_history_moved"`
}

func doAliasRequest(t *testing.T, srv http.Handler, method, path, body string) (*httptest.ResponseRecorder, aliasesResp) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var resp aliasesResp
	if w.Body.Len() > 0 {
		json.NewDecoder(w.Body).Decode(&resp)
	}
	return w, resp
}

func TestUserAliasesAPI_LinkListUnlink(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	for _, n := range []string{"alice", "bob"} {
		if _, err := st.GetOrCreateUser(n); err != nil {
			t.Fatal(err)
		}
	}

	w, resp := doAliasRequest(t, srv, http.MethodGet, "/api/users/alice/aliases", "")
	if w.Code != http.StatusOK || resp.Aliases == nil || len(resp.Aliases) != 0 {
		t.Fatalf("initial GET: %d %+v, want 200 with empty list", w.Code, resp)
	}

	w, resp = doAliasRequest(t, srv, http.MethodPost, "/api/users/alice/aliases", `{"alias":"bob"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("POST: status %d, body %s", w.Code, w.Body)
	}
	if !reflect.DeepEqual(resp.Aliases, []string{"bob"}) {
		t.Errorf("POST aliases = %v, want [bob]", resp.Aliases)
	}

	w, resp = doAliasRequest(t, srv, http.MethodDelete, "/api/users/alice/aliases/bob", "")
	if w.Code != http.StatusOK || len(resp.Aliases) != 0 {
		t.Fatalf("DELETE: %d %+v, want 200 with empty list", w.Code, resp)
	}

	w, _ = doAliasRequest(t, srv, http.MethodDelete, "/api/users/alice/aliases/bob", "")
	if w.Code != http.StatusNotFound {
		t.Errorf("second DELETE: status %d, want 404", w.Code)
	}
}

func TestUserAliasesAPI_RejectsBadLinks(t *testing.T) {
	srv, st := newTestServerWrapped(t)
	if _, err := st.CreateLocalUser("alice", "alice@test.local", "", models.RoleAdmin); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, path, body string
		want             int
	}{
		{"self", "/api/users/alice/aliases", `{"alias":"alice"}`, http.StatusBadRequest},
		{"blank", "/api/users/alice/aliases", `{"alias":" "}`, http.StatusBadRequest},
		{"bad json", "/api/users/alice/aliases", `{`, http.StatusBadRequest},
		{"current admin", "/api/users/alice/aliases", `{"alias":"test-admin"}`, http.StatusBadRequest},
		{"unknown user", "/api/users/nobody/aliases", `{"alias":"bob"}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := doAliasRequest(t, srv, http.MethodPost, tc.path, tc.body)
			if w.Code != tc.want {
				t.Errorf("status %d, want %d (body %s)", w.Code, tc.want, w.Body)
			}
		})
	}
}

func TestUserAliasesAPI_ForbiddenForViewer(t *testing.T) {
	srv, st := newTestServer(t)
	viewerToken := createViewerSession(t, st, "vic")

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/users/alice/aliases", ""},
		{http.MethodPost, "/api/users/alice/aliases", `{"alias":"bob"}`},
		{http.MethodDelete, "/api/users/alice/aliases/bob", ""},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: viewerToken})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s: status %d, want 403", tc.method, tc.path, w.Code)
		}
	}
}
