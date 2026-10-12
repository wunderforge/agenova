// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package websession

import (
	"context"
	"encoding/json"
	"github.com/wunderforge/agenova/internal/login"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/invitations"
)

type googleFixture struct {
	nonce, verifier string
	calls           int
}

func (g *googleFixture) AuthorizationURL(state, nonce, verifier string) (string, error) {
	g.nonce = nonce
	g.verifier = verifier
	return "https://accounts.google.com/authorize?state=" + state, nil
}
func (g *googleFixture) Exchange(_ context.Context, code, verifier, nonce string) (googleoidc.LoginIdentity, error) {
	g.calls++
	if code != "one-use" || verifier != g.verifier || nonce != g.nonce || nonce == "" {
		return googleoidc.LoginIdentity{}, authentication.ErrUnauthenticated
	}
	return googleoidc.LoginIdentity{Issuer: googleoidc.Issuer, Subject: "person", Email: "owner@gmail.com", EmailAuthoritative: true}, nil
}

func TestGoogleBrowserInvitationLoginAndLogout(t *testing.T) {
	store := testStore(t)
	members := postgres.Store{DB: store.DB}
	ctx := context.Background()
	_, proof, err := members.ReserveGoogleOwner(ctx, "team", invitations.Request{Email: "owner@gmail.com", Project: "project", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	boundary, err := authentication.New(postgres.ApplicationSessions{Store: members}, members)
	if err != nil {
		t.Fatal(err)
	}
	google := &googleFixture{}
	h, err := New(Config{Origin: "https://service.example", Google: google, ApplicationSessions: members, Invitations: members, Store: store, Boundary: boundary, Revoker: members, Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := boundary.Authenticate(r); err != nil {
			t.Fatal("API identity missing")
		}
		w.WriteHeader(204)
	})})
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://service.example"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
	if w := call("GET", "/login", "", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "Continue with Google") {
		t.Fatal("Google login missing")
	}
	if w := call("POST", "/login", "", "https://evil.example", nil); w.Code != 403 {
		t.Fatal("cross-site login accepted")
	}
	start := call("POST", "/login", "invitation="+proof, "https://service.example", nil)
	if start.Code != 303 {
		t.Fatal("start", start.Code)
	}
	location, _ := url.Parse(start.Header().Get("Location"))
	state := location.Query().Get("state")
	loginCookie := start.Result().Cookies()[0]
	callback := call("GET", "/auth/callback?code=one-use&state="+state, "", "", loginCookie)
	if callback.Code != 303 {
		t.Fatal("callback", callback.Code)
	}
	var browserCookie *http.Cookie
	for _, c := range callback.Result().Cookies() {
		if c.Name == sessionCookie {
			browserCookie = c
		}
	}
	if browserCookie == nil || !browserCookie.Secure || !browserCookie.HttpOnly || len(browserCookie.Value) != 43 {
		t.Fatal("invalid session cookie")
	}
	if w := call("GET", "/api/work", "", "", browserCookie); w.Code != 204 {
		t.Fatal("authenticated API", w.Code)
	}
	if w := call("GET", "/auth/callback?code=one-use&state="+state, "", "", loginCookie); w.Code != 400 || google.calls != 1 {
		t.Fatal("callback replay")
	}
	if w := call("POST", "/logout", "", "https://evil.example", browserCookie); w.Code != 403 {
		t.Fatal("cross-site logout accepted")
	}
	if w := call("POST", "/logout", "", "https://service.example", browserCookie); w.Code != 303 {
		t.Fatal("logout", w.Code)
	}
	if w := call("GET", "/api/work", "", "", browserCookie); w.Code != 401 {
		t.Fatal("session survived logout")
	}
	// Google assertions themselves never authorize the API.
	r := httptest.NewRequest("GET", "https://service.example/api/work", nil)
	r.Header.Set("Authorization", "Bearer google-id-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("Google token accepted as API credential")
	}
}

func TestGoogleNativeHandoffPKCEAndReplay(t *testing.T) {
	store := testStore(t)
	members := postgres.Store{DB: store.DB}
	ctx := context.Background()
	if err := members.PutMember(ctx, "operator", postgres.Member{Issuer: googleoidc.Issuer, Subject: "person", Principal: "person", Team: "team", Active: true}); err != nil {
		t.Fatal(err)
	}
	boundary, _ := authentication.New(postgres.ApplicationSessions{Store: members}, members)
	g := &googleFixture{}
	h, err := New(Config{Origin: "https://service.example", Google: g, ApplicationSessions: members, Store: store, Boundary: boundary, Revoker: members, Next: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://service.example"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "https://service.example")
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	pkce := oauth2.GenerateVerifier()
	state := strings.Repeat("s", 43)
	q := url.Values{"client_id": {"agenova-cli"}, "response_type": {"code"}, "redirect_uri": {login.Callback}, "scope": {"agenova/api"}, "state": {state}, "code_challenge_method": {"S256"}, "code_challenge": {oauth2.S256ChallengeFromVerifier(pkce)}}
	bad := url.Values{}
	for k, v := range q {
		bad[k] = v
	}
	bad.Set("redirect_uri", "https://evil.example")
	if w := call("GET", "/auth/authorize?"+bad.Encode(), ""); w.Code != 400 {
		t.Fatal("untrusted callback accepted")
	}
	auth := call("GET", "/auth/authorize?"+q.Encode(), "")
	if auth.Code != 303 {
		t.Fatal("authorize", auth.Code)
	}
	nc := auth.Result().Cookies()[0]
	page := call("GET", "/login", "", nc)
	if !strings.Contains(page.Body.String(), "Authorize Agenova CLI") {
		t.Fatal("no explicit CLI consent")
	}
	start := call("POST", "/login", "", nc)
	if start.Code != 303 {
		t.Fatal("login", start.Code)
	}
	var lc *http.Cookie
	for _, c := range start.Result().Cookies() {
		if c.Name == loginCookie {
			lc = c
		}
	}
	destination, _ := url.Parse(start.Header().Get("Location"))
	callback := call("GET", "/auth/callback?code=one-use&state="+destination.Query().Get("state"), "", lc)
	redirect, _ := url.Parse(callback.Header().Get("Location"))
	if callback.Code != 303 || redirect.Scheme+"://"+redirect.Host+redirect.Path != login.Callback || redirect.Query().Get("state") != state {
		t.Fatal("bad CLI handoff")
	}
	code := redirect.Query().Get("code")
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {"agenova-cli"}, "redirect_uri": {login.Callback}, "code": {code}, "code_verifier": {pkce}}
	response := call("POST", "/auth/token", form.Encode())
	if response.Code != 200 {
		t.Fatal("exchange", response.Code)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(response.Body.Bytes(), &token) != nil {
		t.Fatal("token response")
	}
	if _, err := (postgres.ApplicationSessions{Store: members}).Verify(ctx, token.AccessToken); err != nil {
		t.Fatal("invalid application token")
	}
	if w := call("POST", "/auth/token", form.Encode()); w.Code != 401 {
		t.Fatal("code replay")
	}
	// A stolen authorization code without its original verifier cannot be used.
	handle, _ := randomHandle()
	if err := store.PutAttempt(ctx, "code:"+handle, nativeCode{Kind: "cli-code", Issuer: googleoidc.Issuer, Subject: "person", Challenge: oauth2.S256ChallengeFromVerifier(pkce)}); err != nil {
		t.Fatal(err)
	}
	form.Set("code", handle)
	form.Set("code_verifier", oauth2.GenerateVerifier())
	if w := call("POST", "/auth/token", form.Encode()); w.Code != 401 {
		t.Fatal("PKCE bypass")
	}
	form.Set("code_verifier", pkce)
	if w := call("POST", "/auth/token", form.Encode()); w.Code != 401 {
		t.Fatal("failed exchange did not consume code")
	}
}
