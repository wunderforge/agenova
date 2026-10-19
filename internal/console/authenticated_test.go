// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/policy"
)

type memberAccess struct{}

func (memberAccess) Authorize(_ context.Context, _ v0.Principal, action, project string) error {
	if project == "payments" && (action == "work.submit" || action == "work.read-own") {
		return nil
	}
	return authentication.ErrForbidden
}

type principalSource struct{ p v0.Principal }

func (p principalSource) Principal() v0.Principal { return p.p }

type registeredTemplate struct{}

func (registeredTemplate) Lookup(name string) (*v0.AgentTemplate, error) {
	if name != "engineer" {
		return nil, errors.New("not found")
	}
	return app.ReferenceTemplate(), nil
}

type testAuth struct {
	mu       sync.Mutex
	disabled bool
}

func (a *testAuth) Authenticate(r *http.Request) (v0.Principal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.disabled {
		return v0.Principal{}, authentication.ErrForbidden
	}
	name := r.Header.Get("Authorization")
	if name != "Bearer alice" && name != "Bearer bob" {
		return v0.Principal{}, authentication.ErrUnauthenticated
	}
	return v0.Principal{Subject: name, Team: "team-a", AuthenticationContext: "oidc"}, nil
}
func (a *testAuth) AuthenticateSession(r *http.Request) (v0.Principal, func(context.Context) error, error) {
	p, err := a.Authenticate(r)
	if err != nil {
		return p, nil, err
	}
	return p, func(context.Context) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.disabled {
			return authentication.ErrForbidden
		}
		return nil
	}, nil
}
func TestAuthenticatedOwnEvidenceAndRevocation(t *testing.T) {
	provider := &httpProvider{}
	b := &verticalBackend{}
	loader := &policy.Loader{}
	if err := loader.Load(app.ReferencePolicy()); err != nil {
		t.Fatal(err)
	}
	s, err := NewServiceWithOptions(b, httpExecutor{}, provider, app.ReferencePrincipalTeamA, Options{Setup: memberTestSetup, Configure: memberTestConfigure, Access: memberAccess{}, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, principalSource{p}, loader, registeredTemplate{})
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	auth := &testAuth{}
	h, err := AuthenticatedHandler(s, auth)
	if err != nil {
		t.Fatal(err)
	}
	headers := func(name string) map[string]string { return map[string]string{"Authorization": "Bearer " + name} }
	if got := httpCall(h, "POST", "/api/requests", httpInput("anonymous"), nil); got.Code != 401 {
		t.Fatal(got.Code)
	}
	if b.calls.Load() != 0 || provider.calls.Load() != 0 {
		t.Fatal("unauthenticated request allocated")
	}
	for _, name := range []string{"alice", "bob"} {
		got := httpCall(h, "POST", "/api/requests", httpInput(name), headers(name))
		if got.Code != 202 {
			t.Fatalf("%s: %d %s", name, got.Code, got.Body)
		}
		awaitVertical(t, s, name)
	}
	alice, _ := s.QueryRequest("alice")
	for _, path := range []string{"/api/requests/alice/evidence", "/api/claims/" + alice.State.Claim.ID + "/evidence", "/api/requests/missing/evidence"} {
		got := httpCall(h, "GET", path, nil, headers("bob"))
		if got.Code != 404 {
			t.Fatalf("foreign read %s: %d", path, got.Code)
		}
	}
	got := httpCall(h, "GET", "/api/requests", nil, headers("bob"))
	var views []evidence.View
	if err := json.Unmarshal(got.Body.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].RequestRef != "bob" || views[0].State.Principal.Subject != "Bearer bob" {
		t.Fatal("owner list/attribution failed")
	}
	if got := httpCall(h, "POST", "/api/requests", httpInput("alice"), headers("bob")); got.Code != 409 {
		t.Fatal("collision must not return existing evidence")
	}
	if got := httpCall(h, "GET", "/api/provider-observations", nil, headers("bob")); got.Code != 403 {
		t.Fatal("diagnostics leaked")
	}
	if got := httpCall(Handler(s), "GET", "/api/requests", nil, nil); got.Code != 503 {
		t.Fatal("reference handler bypass")
	}
	if _, err := s.Submit(httpInput("bypass")); err == nil {
		t.Fatal("reference submit bypass")
	}
	auth.mu.Lock()
	auth.disabled = true
	auth.mu.Unlock()
	for _, method := range []string{"GET", "POST"} {
		if got := httpCall(h, method, "/api/requests", httpInput("removed"), headers("alice")); got.Code != 403 {
			t.Fatal("removed member allowed")
		}
	}
	if provider.calls.Load() != 2 {
		t.Fatalf("unexpected provider calls: %d", provider.calls.Load())
	}
}

type revocableAccess struct{ disabled atomic.Bool }

func (a *revocableAccess) Authorize(ctx context.Context, p v0.Principal, action, project string) error {
	if a.disabled.Load() {
		return authentication.ErrForbidden
	}
	return memberAccess{}.Authorize(ctx, p, action, project)
}
func TestMembershipRevocationCancelsActiveProvider(t *testing.T) {
	provider := &httpProvider{started: make(chan struct{}), release: make(chan struct{})}
	access := &revocableAccess{}
	loader := &policy.Loader{}
	if err := loader.Load(app.ReferencePolicy()); err != nil {
		t.Fatal(err)
	}
	s, err := NewServiceWithOptions(&verticalBackend{}, httpExecutor{}, provider, app.ReferencePrincipalTeamA, Options{Setup: memberTestSetup, Configure: memberTestConfigure, Access: access, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, principalSource{p}, loader, registeredTemplate{})
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := v0.Principal{Subject: "alice", Team: "team-a", AuthenticationContext: "oidc"}
	if _, err := s.SubmitFor(httpInput("revoked-running"), p); err != nil {
		t.Fatal(err)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	access.disabled.Store(true)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v, _ := s.QueryRequest("revoked-running")
		if v.Outcome != nil {
			if v.Outcome.Status != "Cancelled" {
				t.Fatalf("unexpected status: %s", v.Outcome.Status)
			}
			if provider.calls.Load() != 1 {
				t.Fatal("revocation replayed provider")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("membership revocation did not cancel active provider within polling bound")
}

func memberTestSetup() (Setup, error) {
	return Setup{Template: app.ReferenceTemplate(), Policy: app.ReferencePolicy()}, nil
}
func memberTestConfigure(*v0.AgentTemplate) (app.ResolvedLaunch, error) {
	return app.ResolvedLaunch{TemplateRef: app.ReferenceRuntimeTemplateRef}, nil
}
