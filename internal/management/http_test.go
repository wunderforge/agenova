// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package management

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
)

type rejectingAuth struct{}

type projectAuth struct{}

func (projectAuth) Authenticate(*http.Request) (v0.Principal, error) {
	return v0.Principal{Subject: "test-admin", Team: "test-team", AuthenticationContext: "oidc"}, nil
}

func TestProjectCreationRejectsIdentityGrantsAndWrongMethods(t *testing.T) {
	h := Projects(identitydb.Store{DB: &sql.DB{}}, projectAuth{})
	for _, body := range []string{`{"project":"research","team":"foreign"}`, `{"project":"research","role":"admin"}`, `{"project":"research","grants":[]}`, `{} {}`} {
		r := httptest.NewRequest("POST", "/api/projects", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal("untrusted project fields accepted", w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 405 || w.Header().Get("Allow") != "POST" {
		t.Fatal("method guard", w.Code)
	}
}

func (rejectingAuth) Authenticate(*http.Request) (v0.Principal, error) {
	return v0.Principal{}, errors.New("private authentication detail")
}

func TestManagementRequiresAuthentication(t *testing.T) {
	for _, h := range []http.Handler{Governance(identitydb.Store{DB: &sql.DB{}}, rejectingAuth{}, nil), Organization(identitydb.Store{DB: &sql.DB{}}, rejectingAuth{}), Projects(identitydb.Store{DB: &sql.DB{}}, rejectingAuth{})} {
		for _, method := range []string{"GET", "POST", "DELETE"} {
			r := httptest.NewRequest(method, "/", strings.NewReader(`{}`))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 401 || strings.Contains(w.Body.String(), "private") || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unsafe unauthenticated response: %d", w.Code)
			}
		}
	}
}

func TestManagementDecodeRejectsUnknownOrTrailingData(t *testing.T) {
	for _, body := range []string{`{"unexpected":true}`, `{} {}`, `null {}`, strings.Repeat("x", (1<<20)+5000)} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		var value struct {
			Project string `json:"project"`
		}
		if decode(w, r, &value) || w.Code != 400 {
			t.Fatal("invalid management body accepted")
		}
	}
}
