// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package websession

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/login"
	"golang.org/x/oauth2"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("AGENOVA_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(postgres.Schema); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(db, []byte(strings.Repeat("x", 32)))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestEncryptedDurableSessionAndConcurrentLogout(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	handle, _ := randomHandle()
	value := map[string]string{"token": "secret-access-refresh-value"}
	if err := s.Create(ctx, handle, value, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := s.DB.QueryRow(`SELECT payload FROM agenova_browser_sessions WHERE id_hash=$1`, hash(handle)).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-access") {
		t.Fatal("plaintext credential in DB")
	}
	var loaded map[string]string
	version, err := s.Load(ctx, handle, &loaded)
	if err != nil || loaded["token"] != value["token"] {
		t.Fatal("durable session load")
	}
	if s.Delete(ctx, handle) != nil {
		t.Fatal("delete")
	}
	if s.Save(ctx, handle, version, value) == nil {
		t.Fatal("concurrent refresh resurrected logout")
	}
	attemptHandle, _ := randomHandle()
	if s.PutAttempt(ctx, attemptHandle, value) != nil {
		t.Fatal("attempt save")
	}
	if s.TakeAttempt(ctx, attemptHandle, &loaded) != nil {
		t.Fatal("attempt consume")
	}
	if s.TakeAttempt(ctx, attemptHandle, &loaded) == nil {
		t.Fatal("attempt replay accepted")
	}
}

type verifier struct{}

func (verifier) Verify(_ context.Context, token string) (authentication.Identity, error) {
	if token != "private-access" {
		return authentication.Identity{}, authentication.ErrUnauthenticated
	}
	return authentication.Identity{Issuer: "issuer", Subject: "alice", SessionID: "session", TokenID: "token", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

type member struct{ revoked bool }

func (m *member) Resolve(context.Context, authentication.Identity) (v0.Principal, error) {
	if m.revoked {
		return v0.Principal{}, authentication.ErrForbidden
	}
	return v0.Principal{Subject: "alice", Team: "team", AuthenticationContext: "oidc"}, nil
}
func (m *member) Revoke(context.Context, authentication.Identity) error { m.revoked = true; return nil }
func TestWebLoginCookieCSRFAndLogout(t *testing.T) {
	store := testStore(t)
	membership := &member{}
	boundary, _ := authentication.New(verifier{}, membership)
	tokens := 0
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "private-access", "refresh_token": "private-refresh", "token_type": "Bearer", "expires_in": 3600})
	}))
	defer idp.Close()
	native := login.Config{Issuer: idp.URL + "/issuer", ClientID: "native", AuthorizationEndpoint: idp.URL + "/authorize", TokenEndpoint: idp.URL + "/token", RedirectURI: login.Callback, Scopes: []string{"openid"}}
	h, err := New(Config{Origin: "https://service.example", Native: native, Web: oauth2.Config{ClientID: "web", RedirectURL: "https://service.example/auth/callback", Scopes: []string{"openid"}, Endpoint: oauth2.Endpoint{AuthURL: native.AuthorizationEndpoint, TokenURL: native.TokenEndpoint, AuthStyle: oauth2.AuthStyleInParams}}, Store: store, Boundary: boundary, Revoker: membership, HTTPClient: idp.Client(), Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("protected portal")) })})
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://service.example"+path, nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/", "", nil); w.Code != 303 || w.Header().Get("Location") != "/login" {
		t.Fatal("anonymous root not redirected")
	}
	if w := call("POST", "/login", "https://evil.example", nil); w.Code != 403 {
		t.Fatal("cross-site login allowed")
	}
	start := call("POST", "/login", "https://service.example", nil)
	if start.Code != 303 {
		t.Fatal(start.Code)
	}
	location, _ := url.Parse(start.Header().Get("Location"))
	state := location.Query().Get("state")
	cookies := start.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe login cookie")
	}
	callback := call("GET", "/auth/callback?state="+url.QueryEscape(state)+"&code=one-time-code", "", cookies[0])
	if callback.Code != 303 {
		t.Fatalf("callback %d", callback.Code)
	}
	var sessionCookieValue *http.Cookie
	for _, c := range callback.Result().Cookies() {
		if c.Name == sessionCookie {
			sessionCookieValue = c
		}
	}
	if sessionCookieValue == nil || strings.Contains(sessionCookieValue.Value, "private-") {
		t.Fatal("missing opaque session")
	}
	if call("GET", "/auth/callback?state="+url.QueryEscape(state)+"&code=one-time-code", "", cookies[0]).Code != 400 || tokens != 1 {
		t.Fatal("callback replay reached token endpoint")
	}
	if call("GET", "/", "", sessionCookieValue).Code != 200 {
		t.Fatal("valid session failed")
	}
	if call("POST", "/logout", "https://evil.example", sessionCookieValue).Code != 403 || membership.revoked {
		t.Fatal("cross-site logout")
	}
	if call("POST", "/logout", "https://service.example", sessionCookieValue).Code != 303 || !membership.revoked {
		t.Fatal("logout not revoked")
	}
	if call("GET", "/api/requests", "", sessionCookieValue).Code != 401 {
		t.Fatal("logged-out session usable")
	}
}
