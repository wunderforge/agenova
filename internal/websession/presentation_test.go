// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package websession

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/authentication/postgres"
)

func TestJuneAccessStatusPagesKeepAuthenticationUnchanged(t *testing.T) {
	membership := &member{}
	boundary, err := authentication.New(verifier{}, membership)
	if err != nil {
		t.Fatal(err)
	}
	nextCalls := 0
	h, err := New(Config{Origin: "https://service.example", Google: &googleFixture{}, ApplicationSessions: postgres.Store{}, Registration: postgres.Store{}, RegistrationTeam: "team", Store: &Store{}, Boundary: boundary, Revoker: membership, Next: http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalls++ })})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/login", "/access-pending", "/access-declined"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "https://service.example"+path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
		for _, marker := range []string{`name="color-scheme" content="only light"`, `name="darkreader-lock"`, `name="theme-color" content="#faf9fc"`} {
			if !strings.Contains(w.Body.String(), marker) {
				t.Fatalf("%s missing %s", path, marker)
			}
		}
		if len(w.Result().Cookies()) != 0 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "style-src 'sha256-") {
			t.Fatal("status presentation issued credentials or lost CSP")
		}
	}
	if nextCalls != 0 {
		t.Fatal("public status page reached private application")
	}
}
