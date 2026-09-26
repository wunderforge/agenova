// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/toolgateway"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type toolDouble struct {
	calls  atomic.Int32
	fail   bool
	before func()
}

func (p *toolDouble) Invoke(context.Context, toolbackend.Invocation) (toolbackend.Result, error) {
	p.calls.Add(1)
	if p.before != nil {
		p.before()
	}
	if p.fail {
		return toolbackend.Result{}, toolbackend.ErrUnavailable
	}
	return toolbackend.Result{Text: "private provider observation", ResultRef: "artifact:readme"}, nil
}
func boundTools(t *testing.T, provider toolbackend.Provider, operation, scope string) *toolbackend.Set {
	t.Helper()
	tools, err := toolbackend.NewSet([]toolbackend.Binding{{Descriptor: toolbackend.Descriptor{Description: "Read a fixture file and return untrusted text.", Operation: operation, ResourceScope: scope, Parameter: "file", MaxBytes: 128, AllowedValues: []string{"logs/timeout.log"}}, Provider: provider, MaxObservationBytes: 8}})
	if err != nil {
		t.Fatal(err)
	}
	return tools
}
func TestConfiguredServiceUsesProviderAndPreservesAttemptTarget(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "unavailable"}[fail], func(t *testing.T) {
			provider := &toolDouble{fail: fail}
			tools := boundTools(t, provider, "git.read", "repo:acme/payments")
			service, err := NewServiceWithOptions(&verticalBackend{}, reactExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{ToolBackend: tools})
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			if _, err := service.Submit(verticalRequest(t, "configured")); err != nil {
				t.Fatal(err)
			}
			view := awaitVertical(t, service, "configured")
			var attempt, outcome *facts.Fact
			for i := range view.Facts {
				f := &view.Facts[i]
				if f.Operation != "tool.invoke" {
					continue
				}
				if strings.HasPrefix(f.ReasonCode, "mock") {
					t.Fatal("configured provider became mock")
				}
				if f.Kind == "ProviderAttempt" {
					attempt = f
				}
				if f.Kind == "ProviderOutcome" {
					outcome = f
				}
			}
			if provider.calls.Load() != 1 || attempt == nil || outcome == nil || attempt.Target != "git.read" || attempt.Target != outcome.Target || attempt.InvocationID != outcome.InvocationID || attempt.Sequence >= outcome.Sequence {
				t.Fatalf("attempt=%+v outcome=%+v calls=%d", attempt, outcome, provider.calls.Load())
			}
			if fail && (outcome.ReasonCode != "tool-transport-unavailable" || view.Outcome.Status != "Failed") {
				t.Fatal("unavailable transport silently succeeded")
			}
			if !fail && (outcome.ProviderStatus != "Succeeded" || view.Outcome.Status != "Succeeded") {
				t.Fatal("provider double did not complete composition")
			}
		})
	}
}

type toolClaims struct{ snapshot app.ClaimAuthoritySnapshot }

func (c toolClaims) Claim(id string) (v0.SandboxClaim, bool) {
	return c.snapshot.Claim, id == c.snapshot.Claim.ID
}
func (c toolClaims) ClaimAuthority(id string) (app.ClaimAuthoritySnapshot, bool) {
	return c.snapshot, id == c.snapshot.Claim.ID
}
func runningToolClaims() toolClaims {
	return toolClaims{snapshot: app.ClaimAuthoritySnapshot{Claim: v0.SandboxClaim{ID: "claim", RequestRef: "work", TemplateRef: "template", AuthorityRef: "authority", Phase: v0.ClaimPhaseRunning, BackendIdentity: &v0.SandboxClaimBackendIdentity{Backend: "double", WorkerID: "worker"}}, EffectiveAuthority: v0.EffectiveAuthority{ID: "authority", Tools: []string{"repo.read"}, ResourceScopes: []string{"repo:example/a", "repo:example/b"}}}}
}

func TestProviderBoundaryRejectsBeforeExternalCallAndRecordsBeforeAllow(t *testing.T) {
	for _, name := range []string{"allow", "deny", "scope", "cross-claim", "terminal", "decision-record", "attempt-record", "outcome-record", "argument"} {
		t.Run(name, func(t *testing.T) {
			claims := runningToolClaims()
			provider := &toolDouble{}
			tools := boundTools(t, provider, "repo.read", "repo:example/a")
			recorded := []facts.Fact{}
			if name == "deny" {
				claims.snapshot.EffectiveAuthority.Tools = nil
			}
			if name == "terminal" {
				claims.snapshot.Claim.Phase = v0.ClaimPhaseSucceeded
			}
			appendFact := func(f facts.Fact) (facts.Fact, error) {
				if (name == "attempt-record" && f.Kind == "ProviderAttempt") || (name == "outcome-record" && f.Kind == "ProviderOutcome") {
					return facts.Fact{}, errors.New("injected journal failure")
				}
				recorded = append(recorded, f)
				return f, nil
			}
			adapter := &providerToolAdapter{ctx: context.Background(), claims: claims, appendFact: appendFact, ref: "work", claimID: "claim", tools: tools, results: map[string]workerprotocol.Reply{}}
			if name == "cross-claim" {
				adapter.claimID = "another-bound-context"
			} // test-only mismatch, not caller authentication.
			provider.before = func() {
				if len(recorded) != 2 || recorded[0].Kind != "ToolDecision" || recorded[1].Kind != "ProviderAttempt" {
					t.Fatal("provider reached before durable ordering")
				}
			}
			gw := toolgateway.NewGateway(claims, nil, nil, toolgateway.WithAdapter(adapter), toolgateway.WithObserver(func(req toolgateway.Request, d gateway.Decision) error {
				if name == "decision-record" {
					return errors.New("injected journal failure")
				}
				_, err := appendFact(facts.Fact{Kind: "ToolDecision", Result: d.Result, InvocationID: d.InvocationID})
				return err
			}))
			req := toolgateway.Request{ClaimID: "claim", Tool: "repo", Action: "read", ResourceScope: "repo:example/a", Parameters: map[string]string{"file": "logs/timeout.log"}}
			if name == "scope" {
				req.ResourceScope = "repo:outside"
			}
			if name == "argument" {
				req.Parameters["file"] = "../private"
			}
			decision, err := gw.Invoke(req)
			wantCalls := int32(0)
			if name == "allow" || name == "outcome-record" {
				wantCalls = 1
			}
			if provider.calls.Load() != wantCalls {
				t.Fatalf("calls=%d want=%d", provider.calls.Load(), wantCalls)
			}
			if name == "allow" {
				reply := adapter.results[decision.InvocationID]
				if err != nil || !reply.Untrusted || !reply.Truncated || !strings.Contains(reply.Text, "[TRUNCATED]") {
					t.Fatalf("reply=%+v err=%v", reply, err)
				}
			}
			if name == "outcome-record" && err == nil {
				t.Fatal("post-call journal failure hidden")
			}
		})
	}
}
