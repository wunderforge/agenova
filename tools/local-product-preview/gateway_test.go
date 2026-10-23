// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/toolbackend"
)

type previewTestRegistry struct{}

func (previewTestRegistry) Lookup(string) (*v0.AgentTemplate, error) {
	t := app.ReferenceTemplate()
	t.Spec.CapabilityCeiling.Tools = []string{"web.read", "crm.read"}
	t.Spec.CapabilityCeiling.ResourceScopes = []string{"web:public-pages", "crm:customer-retention"}
	return t, nil
}

type countedPublicProvider struct{ calls atomic.Int32 }

func (p *countedPublicProvider) Invoke(context.Context, toolbackend.Invocation) (toolbackend.Result, error) {
	p.calls.Add(1)
	return toolbackend.Result{Text: `{"source_id":"public-page","content":"synthetic test page"}`, Untrusted: true}, nil
}

// The counter is below the real Gateway, separate from the refusal facts. A
// fabricated UI refusal or a malformed-argument rejection cannot pass this test.
func TestMissingResearchPermissionReachesGatewayButNeverProvider(t *testing.T) {
	for _, tc := range []struct {
		name, tools, scopes string
		allowed             bool
	}{
		{"allowed-control", `["web.read"]`, `["web:public-pages"]`, true},
		{"missing-tool", `["crm.read"]`, `["web:public-pages"]`, false},
		{"missing-scope", `["web.read"]`, `["crm:customer-retention"]`, false},
		{"neither-requested", `[]`, `[]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &countedPublicProvider{}
			tools, err := toolbackend.NewSet([]toolbackend.Binding{{Descriptor: toolbackend.Descriptor{Operation: "web.read", ResourceScope: "web:public-pages", Parameter: "url", InputMode: "text", MaxBytes: 1024, Description: "Read a public HTTPS page."}, Provider: provider, Backend: "test-mcp-boundary", MaxObservationBytes: 4096, MaxConcurrentCalls: 1}})
			if err != nil {
				t.Fatal(err)
			}
			backend := &previewRuntime{claims: map[string]string{}, released: map[string]bool{}, authority: map[string]runtime.NetworkAuthority{}}
			service, err := console.NewServiceWithOptions(backend, previewExecutor{}, syntheticProvider{}, app.ReferencePrincipalTeamA, console.Options{ToolBackend: tools, Prepare: func(data []byte) (app.PreparedAssignment, error) {
				policies := &policy.Loader{}
				if err := policies.Load(app.ReferencePolicy()); err != nil {
					return app.PreparedAssignment{}, err
				}
				principal, err := app.NewReferencePrincipalSource(app.ReferencePrincipalTeamA)
				if err != nil {
					return app.PreparedAssignment{}, err
				}
				return app.PrepareAssignment(data, principal, policies, previewTestRegistry{})
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			data := []byte(fmt.Sprintf(`{"apiVersion":"agenova.io/v1alpha1","kind":"ClaimRequest","metadata":{"name":"%s"},"spec":{"templateRef":"engineer","projectRef":"payments","task":{"type":"public-research","input":{"objective":"Read https://example.org/ and summarize the public page."}},"requestedAccess":{"tools":%s,"resourceScopes":%s,"modelProfile":"approved-coding-model"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}}`, tc.name, tc.tools, tc.scopes))
			if _, err := service.Submit(data); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				view, err := service.QueryRequest(tc.name)
				if err != nil {
					t.Fatal(err)
				}
				if view.Outcome != nil {
					denied, attempts := 0, 0
					for _, fact := range view.Facts {
						if fact.Kind == "ToolDecision" && fact.Result == v0.DecisionResultDeny {
							denied++
						}
						if fact.Kind == "ProviderAttempt" {
							attempts++
						}
					}
					if tc.allowed {
						if view.Outcome.Status != "Succeeded" || provider.calls.Load() != 1 || denied != 0 {
							t.Fatal(view.Outcome, provider.calls.Load(), denied)
						}
					} else if view.Outcome.Status != "Failed" || view.Outcome.Text != "" || view.Diagnosis.Code != "invocation-denied" || denied != 1 || attempts != 0 || provider.calls.Load() != 0 {
						t.Fatalf("missing permission became success or called provider: outcome=%+v diagnosis=%+v denied=%d attempts=%d provider=%d", view.Outcome, view.Diagnosis, denied, attempts, provider.calls.Load())
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("local test did not finish")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}
