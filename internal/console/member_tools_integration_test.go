// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"fmt"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"github.com/wunderforge/agenova/internal/workstore"
)

// Identity, model and tool are deterministic doubles. The durable variant uses
// actual disposable PostgreSQL; neither variant claims live Google/EKS proof.
func TestMemberConfiguredToolsAndDurability(t *testing.T) {
	for _, durable := range []bool{false, true} {
		t.Run(fmt.Sprintf("durable=%v", durable), func(t *testing.T) {
			for _, denied := range []bool{false, true} {
				t.Run(fmt.Sprintf("denied=%v", denied), func(t *testing.T) {
					ctx := context.Background()
					alice := v0.Principal{Subject: "integration-alice", Team: "team-a", AuthenticationContext: "oidc"}
					bob := v0.Principal{Subject: "integration-bob", Team: "team-a", AuthenticationContext: "oidc"}
					var storage *workstore.Store
					var access AccessAuthorizer = memberAccess{}
					if durable {
						db := durableTestDB(t)
						identity := identitydb.Store{DB: db}
						for _, p := range []v0.Principal{alice, bob} {
							if err := identity.PutMember(ctx, "operator", identitydb.Member{Issuer: "synthetic", Subject: p.Subject, Principal: p.Subject, Team: p.Team, Active: true}); err != nil {
								t.Fatal(err)
							}
						}
						if _, err := db.Exec(`INSERT INTO agenova_projects VALUES('team-a','payments'); INSERT INTO agenova_project_grants VALUES('integration-alice','team-a','payments','member'),('integration-bob','team-a','payments','member')`); err != nil {
							t.Fatal(err)
						}
						var err error
						storage, err = workstore.Acquire(ctx, db)
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { storage.Close() })
						access = identity
					}
					loader := &policy.Loader{}
					if err := loader.Load(app.ReferencePolicy()); err != nil {
						t.Fatal(err)
					}
					tool := &toolDouble{}
					bindings := []toolbackend.Binding{}
					for _, scope := range []string{"repo:acme/payments", "repo:other/private"} {
						bindings = append(bindings, toolbackend.Binding{Descriptor: toolbackend.Descriptor{Operation: "git.read", ResourceScope: scope, Description: "Read test data", Parameter: "file", MaxBytes: 128, AllowedValues: []string{"logs/timeout.log"}}, Provider: tool, Backend: "test", MaxObservationBytes: 64, MaxConcurrentCalls: 1})
					}
					tools, err := toolbackend.NewSet(bindings)
					if err != nil {
						t.Fatal(err)
					}
					var seen workerprotocol.Task
					executor := reactExecutor{seen: &seen}
					if denied {
						executor.scope = "repo:other/private"
					}
					options := Options{ToolBackend: tools, Access: access, Setup: memberTestSetup, Configure: memberTestConfigure, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
						return app.PrepareAssignment(data, principalSource{p}, loader, registeredTemplate{})
					}}
					if durable {
						options.Durable = storage
					}
					service, err := NewServiceWithOptions(&verticalBackend{}, executor, &verticalProvider{}, "", options)
					if err != nil {
						t.Fatal(err)
					}
					defer service.Close()
					if _, err := service.Submit(verticalRequest(t, "bypass")); err == nil {
						t.Fatal("reference submission bypassed member boundary")
					}
					ref := "member-configured"
					initial, err := service.SubmitFor(verticalRequest(t, ref), alice)
					if err != nil {
						t.Fatal(err)
					}
					view := awaitVertical(t, service, initial.RequestRef)
					if view.State.Principal.Subject != alice.Subject {
						t.Fatal("fixed or incorrect principal")
					}
					if len(seen.Tools) != 1 || seen.Tools[0].ResourceScope != "repo:acme/payments" || seen.Tools[0].Synthetic {
						t.Fatalf("wrong catalog: %+v", seen.Tools)
					}
					wantCalls, wantStatus, wantDecision := int32(1), "Succeeded", v0.DecisionResultAllow
					if denied {
						wantCalls, wantStatus, wantDecision = 0, "Failed", v0.DecisionResultDeny
					}
					if tool.calls.Load() != wantCalls || view.Outcome.Status != wantStatus {
						t.Fatalf("calls=%d outcome=%s", tool.calls.Load(), view.Outcome.Status)
					}
					found := false
					for _, f := range view.Facts {
						if f.Kind == "ToolDecision" {
							found = true
							if f.Result != wantDecision {
								t.Fatal("wrong tool decision")
							}
						}
					}
					if !found {
						t.Fatal("lost tool decision")
					}
					if _, err := service.QueryFor(bob, initial.RequestRef, false); err == nil {
						t.Fatal("foreign member read evidence")
					}
					service.Close()
					if durable {
						restarted, err := NewServiceWithOptions(&verticalBackend{}, executor, &verticalProvider{}, "", options)
						if err != nil {
							t.Fatal(err)
						}
						defer restarted.Close()
						restored, err := restarted.QueryFor(alice, initial.RequestRef, false)
						if err != nil || restored.Outcome == nil || restored.Outcome.Status != wantStatus {
							t.Fatal("lost durable tool outcome", err)
						}
						if len(restored.Facts) != len(view.Facts) {
							t.Fatal("lost durable Tool facts")
						}
						if _, err := restarted.QueryFor(bob, initial.RequestRef, false); err == nil {
							t.Fatal("restart lost owner isolation")
						}
						if _, err := restarted.SubmitFor(verticalRequest(t, ref), alice); err != nil {
							t.Fatal("idempotent submission failed", err)
						}
						if tool.calls.Load() != wantCalls {
							t.Fatal("restart replayed provider")
						}
					}
				})
			}
		})
	}
}
