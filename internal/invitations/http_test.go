// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package invitations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
)

type denyStore struct {
	Store
	calls int
}

func (s *denyStore) Reserve(context.Context, v0.Principal, string, Request) (Invitation, error) {
	s.calls++
	return Invitation{}, authentication.ErrForbidden
}
func (s *denyStore) ListInvitations(context.Context, v0.Principal) ([]Invitation, error) {
	s.calls++
	return nil, authentication.ErrForbidden
}

type memberAuth struct{}

func (memberAuth) Authenticate(r *http.Request) (v0.Principal, error) {
	if r.Header.Get("Authorization") != "Bearer member" {
		return v0.Principal{}, authentication.ErrUnauthenticated
	}
	return v0.Principal{Subject: "member", Team: "team", AuthenticationContext: "oidc"}, nil
}

type unreachableDirectory struct{ Directory }

func TestInviteHTTPRejectsIdentityInjectionAndManagementEscalation(t *testing.T) {
	store := &denyStore{}
	h := Handler(Service{Issuer: "issuer", Directory: unreachableDirectory{}, Store: store}, memberAuth{})
	for _, tc := range []struct {
		body, token string
		want        int
	}{
		{`{"email":"a@example.com","project":"p","role":"member"}`, "", 401},
		{`{"email":"a@example.com","project":"p","role":"member","team":"other"}`, "member", 400},
		{`{"email":"a@example.com","project":"p","role":"admin"}`, "member", 403},
		{`{"email":"a@example.com","project":"p","role":"owner"}`, "member", 400},
	} {
		r := httptest.NewRequest("POST", "https://app.example/api/invitations", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d", w.Code, tc.want)
		}
	}
	if store.calls != 1 {
		t.Fatal("invalid requests reached store")
	}
}

type pageStore struct{ Store }

func (pageStore) ListInvitations(context.Context, v0.Principal) ([]Invitation, error) {
	return []Invitation{}, nil
}
func TestInvitationPageRequiresAdminAndUsesBoundedScripts(t *testing.T) {
	r := httptest.NewRequest("GET", "https://app.example/members", nil)
	r.Header.Set("Authorization", "Bearer member")
	denied := httptest.NewRecorder()
	Page(Service{Store: &denyStore{}}, memberAuth{}).ServeHTTP(denied, r)
	if denied.Code != 403 {
		t.Fatal("member rendered management page")
	}
	w := httptest.NewRecorder()
	Page(Service{Store: pageStore{}}, memberAuth{}).ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Create invitation") || strings.Contains(w.Body.String(), "{{script}}") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'sha256-") || strings.Contains(w.Body.String(), "Bearer member") {
		t.Fatal("unsafe or incomplete page")
	}
}
