// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"context"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"net/http/httptest"
	"testing"
	"time"
)

type verifyFunc func(context.Context, string) (Identity, error)

func (f verifyFunc) Verify(c context.Context, s string) (Identity, error) { return f(c, s) }

type membershipFunc func(context.Context, Identity) (v0.Principal, error)

func (f membershipFunc) Resolve(c context.Context, id Identity) (v0.Principal, error) {
	return f(c, id)
}
func TestCurrentMembershipOnEveryRequest(t *testing.T) {
	calls := 0
	active := true
	b, _ := New(verifyFunc(func(context.Context, string) (Identity, error) {
		return Identity{Issuer: "issuer", Subject: "alice", TokenID: "t1", SessionID: "s1", ExpiresAt: time.Now().Add(time.Minute)}, nil
	}), membershipFunc(func(_ context.Context, id Identity) (v0.Principal, error) {
		calls++
		if !active {
			return v0.Principal{}, errors.New("disabled")
		}
		return v0.Principal{Subject: id.Subject, Team: "project-team", AuthenticationContext: "oidc"}, nil
	}))
	r := httptest.NewRequest("GET", "https://api.example/api/requests?team=admin", nil)
	r.Header.Set("Authorization", "Bearer private-token")
	r.Header.Set("X-User", "bob")
	p, err := b.Authenticate(r)
	if err != nil || p.Subject != "alice" || p.Team != "project-team" {
		t.Fatal("trusted membership not used", p, err)
	}
	active = false
	if _, err = b.Authenticate(r); err != ErrForbidden || calls != 2 {
		t.Fatal("removed member remained authenticated", err, calls)
	}
}
func TestMalformedAuthHasNoMembershipSideEffect(t *testing.T) {
	calls := 0
	b, _ := New(verifyFunc(func(context.Context, string) (Identity, error) { return Identity{}, errors.New("reject") }), membershipFunc(func(context.Context, Identity) (v0.Principal, error) { calls++; return v0.Principal{}, nil }))
	for _, values := range [][]string{nil, {"Basic abc"}, {"Bearer "}, {"Bearer a b"}, {"Bearer a", "Bearer b"}, {"Bearer invalid"}} {
		r := httptest.NewRequest("GET", "https://api.example/?access_token=secret", nil)
		r.Header["Authorization"] = values
		if _, err := b.Authenticate(r); err != ErrUnauthenticated {
			t.Fatal("invalid auth accepted", err)
		}
	}
	if calls != 0 {
		t.Fatal("unauthenticated request reached membership")
	}
	if _, err := New(nil, nil); err == nil {
		t.Fatal("missing authority sources accepted")
	}
}
