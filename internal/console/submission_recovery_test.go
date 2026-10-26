// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/workstore"
	"testing"
)

func TestSubmissionRecoveryResolvesTrustedCanonicalReference(t *testing.T) {
	db := durableTestDB(t)
	ctx := context.Background()
	identity := identitydb.Store{DB: db}
	for _, n := range []string{"Bearer alice", "Bearer bob"} {
		if err := identity.PutMember(ctx, "bootstrap", identitydb.Member{Issuer: "synthetic", Subject: n, Principal: n, Team: "team-a", Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO agenova_projects VALUES('team-a','payments');INSERT INTO agenova_project_grants VALUES('Bearer alice','team-a','payments','member'),('Bearer bob','team-a','payments','member')`); err != nil {
		t.Fatal(err)
	}
	store, err := workstore.Acquire(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	loader := &policy.Loader{}
	if err = loader.Load(app.ReferencePolicy()); err != nil {
		t.Fatal(err)
	}
	provider := &httpProvider{}
	service, err := NewServiceWithOptions(&verticalBackend{}, httpExecutor{}, provider, app.ReferencePrincipalTeamA, Options{Durable: store, Setup: memberTestSetup, Configure: memberTestConfigure, Access: identity, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, principalSource{p}, loader, registeredTemplate{})
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	h, err := AuthenticatedHandler(service, &testAuth{})
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Authorization": "Bearer alice"}
	body := httpInput("lost-response")
	first := httpCall(h, "POST", "/api/requests", body, headers)
	if first.Code != 202 {
		t.Fatal(first.Code, first.Body)
	}
	original := httpView(t, first)
	if original.RequestRef == "lost-response" {
		t.Fatal("expected canonicalized reference")
	}
	recovered := httpCall(h, "GET", "/api/submissions/lost-response/evidence", nil, headers)
	if recovered.Code != 200 {
		t.Fatal(recovered.Code, recovered.Body)
	}
	if httpView(t, recovered).RequestRef != original.RequestRef {
		t.Fatal("wrong recovered Work")
	}
	retry := httpCall(h, "POST", "/api/requests", body, headers)
	if retry.Code != 202 && retry.Code != 200 {
		t.Fatal(retry.Code, retry.Body)
	}
	if httpView(t, retry).RequestRef != original.RequestRef {
		t.Fatal("retry created another Work")
	}
	if got := httpCall(h, "GET", "/api/submissions/lost-response/evidence", nil, map[string]string{"Authorization": "Bearer bob"}); got.Code != 404 {
		t.Fatal("cross-member recovery", got.Code)
	}
	var changed v0.ClaimRequest
	if json.Unmarshal(body, &changed) != nil {
		t.Fatal("fixture")
	}
	changed.Spec.Task.Input["objective"] = "Different task"
	different, _ := json.Marshal(changed)
	if got := httpCall(h, "POST", "/api/requests", different, headers); got.Code != 409 {
		t.Fatal("same name changed body", got.Code)
	}
	awaitVertical(t, service, original.RequestRef)
	if provider.calls.Load() != 1 {
		t.Fatal("duplicate model invocation", provider.calls.Load())
	}
}
