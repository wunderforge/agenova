// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package invitations

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"net/http/httptest"
	"strings"
	"testing"
)

type denyMemberStore struct{ calls int }

func (s *denyMemberStore) ListMembers(context.Context, v0.Principal) ([]MemberView, error) {
	s.calls++
	return nil, authentication.ErrForbidden
}
func (s *denyMemberStore) ChangeMember(context.Context, v0.Principal, MemberChange) error {
	s.calls++
	return authentication.ErrForbidden
}
func TestMemberHTTPRejectsCallerIdentityAndOversizedBodies(t *testing.T) {
	s := &denyMemberStore{}
	h := MemberHandler(s, memberAuth{})
	for _, tc := range []struct {
		body, token string
		want        int
	}{
		{`{"principal":"other","action":"disable"}`, "", 401},
		{`{"principal":"other","action":"disable","team":"other"}`, "member", 400},
		{`{"principal":"other","action":"disable","actor":"owner"}`, "member", 400},
		{`{"principal":"` + strings.Repeat("a", 3000) + `","action":"disable"}`, "member", 400},
		{`{"principal":"other","action":"disable"}`, "member", 403},
	} {
		r := httptest.NewRequest("POST", "https://app.example/api/members", strings.NewReader(tc.body))
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
	if s.calls != 1 {
		t.Fatal("invalid management reached store")
	}
}
