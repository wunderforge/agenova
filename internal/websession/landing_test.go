// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package websession

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/landing"
)

func TestPublicLandingPreservesAuthenticationBoundary(t *testing.T) {
	membership := &member{}
	boundary, _ := authentication.New(verifier{}, membership)
	nextCalls := 0
	c := Config{Origin: "https://service.example", Google: &googleFixture{}, ApplicationSessions: postgres.Store{}, Store: &Store{}, Boundary: boundary, Revoker: membership,
		Landing: landing.New(filepath.Join("..", "..", "ui", "public")), Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalls++
			w.WriteHeader(204)
		})}
	h, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, c.Origin+path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/", "/?mode=connected", "/index.html", "/landing/"} {
		w := call("GET", path, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Give agents work.") || strings.Contains(w.Body.String(), "agenova-account-navigation") {
			t.Fatal("anonymous entry did not show only public landing", path, w.Code)
		}
		if len(w.Result().Cookies()) != 0 {
			t.Fatal("public homepage created an authentication session")
		}
	}
	if w := call("HEAD", "/", ""); w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("HEAD landing", w.Code)
	}
	for _, path := range []string{"/landing/styles.css", "/landing/app.js", "/landing/assets/work-explorer.png", "/landing/assets/work-evidence.png"} {
		if w := call("GET", path, ""); w.Code != 200 {
			t.Fatal("public asset unavailable", path)
		}
	}
	for _, path := range []string{"/api/setup", "/api/requests", "/api/governance", "/api/members", "/auth/me"} {
		if w := call("GET", path, ""); w.Code != 401 {
			t.Fatal("private API lost protection", path, w.Code)
		}
	}
	for _, path := range []string{"/admin", "/assets/index.js"} {
		if w := call("GET", path, ""); w.Code != 303 || w.Header().Get("Location") != "/login" {
			t.Fatal("private route lost protection", path)
		}
	}
	if w := call("GET", "/landing/private.txt", ""); w.Code != 404 {
		t.Fatal("unlisted public asset exposed")
	}
	if nextCalls != 0 {
		t.Fatal("anonymous request reached private handler")
	}
	if w := call("GET", "/login", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Continue with Google") || !strings.Contains(w.Body.String(), `method="post" action="/login"`) {
		t.Fatal("existing Google login entry changed")
	}
	if w := call("POST", "/login", ""); w.Code != 403 {
		t.Fatal("login CSRF requirement changed")
	}
	if w := call("GET", "/", "private-access"); w.Code != 204 || nextCalls != 1 {
		t.Fatal("authenticated root no longer reaches private Portal")
	}
	wrongOrigin := httptest.NewRequest("GET", "https://evil.example/landing/styles.css", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, wrongOrigin)
	if w.Code != 400 {
		t.Fatal("public asset bypassed pinned host")
	}
	// Other compositions retain the old redirect unless they explicitly opt in.
	h.c.Landing = nil
	if w := call("GET", "/", ""); w.Code != 303 || w.Header().Get("Location") != "/login" {
		t.Fatal("implicit public surface added to another composition")
	}
}
