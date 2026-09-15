// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolgateway

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/gateway/gatewaytest"
	"github.com/wunderforge/agenova/internal/governance"
)

type spyAdapter struct {
	calls []spyCall
	err   error
}

type spyCall struct {
	id  string
	req Request
}

type configuredAdapter struct {
	providerConfiguration string
	calls                 int
	received              Request
}

func (a *configuredAdapter) Invoke(_ string, req Request) error {
	if a.providerConfiguration == "" {
		return errors.New("adapter provider configuration is missing")
	}
	a.calls++
	a.received = req
	return nil
}

func (s *spyAdapter) Invoke(id string, req Request) error {
	s.calls = append(s.calls, spyCall{id: id, req: req})
	return s.err
}

func fixture(t *testing.T, options ...Option) (*Gateway, *gatewaytest.Claims, *facts.Store, *governance.Lineage, *spyAdapter) {
	t.Helper()
	claims := gatewaytest.NewClaims()
	store := facts.NewStore()
	lineage := governance.NewLineage()
	adapter := &spyAdapter{}
	base := []Option{WithAdapter(adapter), WithIDSource(gateway.SequenceIDSource("inv-test"))}
	return NewGateway(claims, lineage, store, append(base, options...)...), claims, store, lineage, adapter
}

func teamARequest(t *testing.T) Request {
	t.Helper()
	authority := gatewaytest.LoadTeamAAuthority(t)
	operation := strings.SplitN(authority.Tools[0], ".", 2)
	if len(operation) != 2 {
		t.Fatalf("fixture tool %q is not tool.action", authority.Tools[0])
	}
	return Request{ClaimID: authority.ClaimID, Tool: operation[0], Action: operation[1], ResourceScope: authority.ResourceScopes[0]}
}

func invoke(t *testing.T, gw *Gateway, req Request) gateway.Decision {
	t.Helper()
	result, err := gw.Invoke(req)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	return result
}

func TestGatewayAllowCorrelatesDecisionAttemptAndFact(t *testing.T) {
	gw, claims, store, _, adapter := fixture(t)
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)

	decision := invoke(t, gw, req)
	if decision.Result != gateway.ResultAllow || decision.InvocationID != "inv-test-1" {
		t.Fatalf("decision = %+v, want correlated Allow", decision)
	}
	if len(adapter.calls) != 1 || adapter.calls[0].id != decision.InvocationID {
		t.Fatalf("adapter calls = %+v, want one correlated call", adapter.calls)
	}
	found := store.ToolInvocations(req.ClaimID)
	if len(found) != 1 || found[0].InvocationID != decision.InvocationID || found[0].Result != gateway.ResultAllow || found[0].ToolName != req.Tool+"."+req.Action {
		t.Fatalf("facts = %+v, want one correlated Allow", found)
	}
}

func TestGatewayUsesAdapterPrivateProviderConfiguration(t *testing.T) {
	claims := gatewaytest.NewClaims()
	adapter := &configuredAdapter{providerConfiguration: "adapter-owned-test-configuration"}
	gw := NewGateway(claims, nil, nil, WithAdapter(adapter), WithIDSource(gateway.SequenceIDSource("inv-private")))
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)

	decision := invoke(t, gw, req)
	if decision.Result != gateway.ResultAllow || adapter.calls != 1 {
		t.Fatalf("decision=%+v adapter calls=%d", decision, adapter.calls)
	}
	if key, found := gateway.FindReservedCredentialKey(adapter.received.Parameters); found {
		t.Fatalf("worker request carried adapter configuration through %q", key)
	}
}

func TestGatewayIssuesFreshTrustedIDBeforeValidation(t *testing.T) {
	gw, claims, store, _, adapter := fixture(t)
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	req.CallerReference = "caller-chosen"
	first := invoke(t, gw, req)
	second := invoke(t, gw, req)
	if first.InvocationID != "inv-test-1" || second.InvocationID != "inv-test-2" || first.InvocationID == req.CallerReference {
		t.Fatalf("trusted ids = %q, %q; caller=%q", first.InvocationID, second.InvocationID, req.CallerReference)
	}
	if len(adapter.calls) != 2 || len(store.ToolInvocations(req.ClaimID)) != 2 {
		t.Fatal("each allowed attempt must correlate one adapter call and one fact")
	}
}

func TestGatewayIssuesInvocationIDBeforePolicyEvaluation(t *testing.T) {
	var issued []string
	var visibleAtPolicy []string
	ids := func() string {
		id := fmt.Sprintf("inv-order-%d", len(issued)+1)
		issued = append(issued, id)
		return id
	}
	policy := func(Request) gateway.Outcome {
		visibleAtPolicy = append([]string(nil), issued...)
		return gateway.Allowed()
	}
	gw, claims, _, _, _ := fixture(t, WithIDSource(ids), WithPolicy(policy))
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	decision := invoke(t, gw, req)
	if len(visibleAtPolicy) != 1 || visibleAtPolicy[0] != decision.InvocationID || len(issued) != 1 {
		t.Fatalf("ids at policy=%v issued=%v decision=%q", visibleAtPolicy, issued, decision.InvocationID)
	}
}

func TestGatewayAllowCarriesPolicyReason(t *testing.T) {
	const reason = "permitted by reference policy v1"
	gw, claims, _, _, adapter := fixture(t, WithPolicy(func(Request) gateway.Outcome {
		return gateway.Outcome{Result: gateway.ResultAllow, Reason: reason}
	}))
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	decision := invoke(t, gw, req)
	if decision.Result != gateway.ResultAllow || decision.Reason != reason || decision.Category != "" || len(adapter.calls) != 1 {
		t.Fatalf("decision=%+v calls=%d", decision, len(adapter.calls))
	}
}

func TestGatewayEnforcesEffectiveToolAndResourceAuthorityBeforePolicy(t *testing.T) {
	base := teamARequest(t)
	tests := []struct {
		name     string
		mutate   func(*Request, *gatewaytest.Claims)
		category string
	}{
		{
			name:     "forbidden tool",
			mutate:   func(req *Request, _ *gatewaytest.Claims) { req.Action = "write" },
			category: gateway.CategoryToolNotGranted,
		},
		{
			name:     "wrong repository",
			mutate:   func(req *Request, _ *gatewaytest.Claims) { req.ResourceScope = "repo:acme/billing" },
			category: gateway.CategoryResourceNotGranted,
		},
		{
			name: "missing authority",
			mutate: func(req *Request, claims *gatewaytest.Claims) {
				delete(claims.Authorities, req.ClaimID)
			},
			category: gateway.CategoryAuthorityUnavailable,
		},
		{
			name: "mismatched authority",
			mutate: func(req *Request, claims *gatewaytest.Claims) {
				authority := claims.Authorities[req.ClaimID]
				authority.ID = "authority:other"
				claims.Authorities[req.ClaimID] = authority
			},
			category: gateway.CategoryAuthorityUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policyCalls := 0
			gw, claims, store, _, adapter := fixture(t, WithPolicy(func(Request) gateway.Outcome {
				policyCalls++
				return gateway.Allowed()
			}))
			claims.Put(base.ClaimID, v1alpha1.ClaimPhaseRunning)
			req := base
			test.mutate(&req, claims)

			decision := invoke(t, gw, req)
			if decision.Result != gateway.ResultDeny || decision.Category != test.category {
				t.Fatalf("decision = %+v, want Deny/%s", decision, test.category)
			}
			if policyCalls != 0 || len(adapter.calls) != 0 {
				t.Fatalf("authority denial reached policy or adapter: policy=%d adapter=%d", policyCalls, len(adapter.calls))
			}
			found := store.ToolInvocations(req.ClaimID)
			if len(found) != 1 || found[0].Result != gateway.ResultDeny || found[0].InvocationID != decision.InvocationID {
				t.Fatalf("denial fact = %+v, want one correlated Deny", found)
			}
		})
	}
}

func TestGatewayRejectsInvalidRequestsBeforeClaimAttributionOrAdapter(t *testing.T) {
	base := teamARequest(t)
	tests := []struct {
		name     string
		mutate   func(*Request)
		category string
	}{
		{"missing claim", func(r *Request) { r.ClaimID = " " }, gateway.CategoryMissingClaimIdentity},
		{"missing tool", func(r *Request) { r.Tool = "" }, gateway.CategoryIncompleteOperation},
		{"missing action", func(r *Request) { r.Action = "\t" }, gateway.CategoryIncompleteOperation},
		{"empty scope", func(r *Request) { r.ResourceScope = "" }, gateway.CategoryAmbiguousResourceScope},
		{"wildcard scope", func(r *Request) { r.ResourceScope = "repo:*" }, gateway.CategoryAmbiguousResourceScope},
		{"reserved credential", func(r *Request) { r.Parameters = map[string]string{gatewaytest.SecretParameterKey(t): "not-inspected"} }, gateway.CategorySecretValue},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gw, claims, store, _, adapter := fixture(t)
			claims.Put(base.ClaimID, v1alpha1.ClaimPhaseRunning)
			req := base
			test.mutate(&req)
			decision := invoke(t, gw, req)
			if decision.Result != gateway.ResultDeny || decision.Category != test.category || decision.InvocationID == "" {
				t.Fatalf("decision = %+v, want Deny/%s with id", decision, test.category)
			}
			if len(adapter.calls) != 0 || len(store.ToolInvocations(req.ClaimID)) != 0 {
				t.Fatal("pre-resolution rejection reached adapter or fabricated a claim fact")
			}
		})
	}
}

func TestGatewayUnknownOrUnavailableClaimViewCreatesNoFact(t *testing.T) {
	req := teamARequest(t)
	for _, test := range []struct {
		name        string
		unavailable bool
		category    string
	}{
		{"unknown", false, gateway.CategoryUnknownClaim},
		{"unavailable", true, gateway.CategoryGatewayUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := facts.NewStore()
			claims := gatewaytest.NewClaims()
			var reader app.ClaimAuthorityReader = claims
			if test.unavailable {
				reader = nil
			}
			gw := NewGateway(reader, nil, store, WithIDSource(gateway.SequenceIDSource("inv")))
			decision, err := gw.Invoke(req)
			if err != nil || decision.Result != gateway.ResultDeny || decision.Category != test.category {
				t.Fatalf("decision=%+v err=%v, want Deny/%s", decision, err, test.category)
			}
			if len(store.ToolInvocations(req.ClaimID)) != 0 {
				t.Fatal("unresolved claim gained a fact")
			}
		})
	}
}

func TestGatewayUsesAuthoritativeRunningValidation(t *testing.T) {
	req := teamARequest(t)
	for _, phase := range []v1alpha1.ClaimPhase{
		v1alpha1.ClaimPhasePending,
		v1alpha1.ClaimPhaseBound,
		v1alpha1.ClaimPhaseSucceeded,
		v1alpha1.ClaimPhaseFailed,
		v1alpha1.ClaimPhaseExpired,
	} {
		t.Run(string(phase), func(t *testing.T) {
			gw, claims, store, _, adapter := fixture(t)
			claims.Put(req.ClaimID, phase)
			decision := invoke(t, gw, req)
			if decision.Result != gateway.ResultDeny || decision.Category != gateway.CategoryClaimNotActive {
				t.Fatalf("decision = %+v", decision)
			}
			if len(adapter.calls) != 0 || len(store.ToolInvocations(req.ClaimID)) != 1 {
				t.Fatal("resolved inactive claim must record one Deny and make no adapter call")
			}
		})
	}

	gw, claims, _, _, adapter := fixture(t)
	malformed := gatewaytest.ClaimSnapshot(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	malformed.AuthorityRef = ""
	claims.Items[req.ClaimID] = malformed
	decision := invoke(t, gw, req)
	if decision.Result != gateway.ResultDeny || decision.Category != gateway.CategoryClaimNotActive || len(adapter.calls) != 0 {
		t.Fatalf("malformed authoritative snapshot was not denied: %+v", decision)
	}
}

func TestGatewayPolicyOutcomesAreTypedAndNeverBypassAdapter(t *testing.T) {
	req := teamARequest(t)
	tests := []struct {
		name     string
		outcome  gateway.Outcome
		result   gateway.DecisionResult
		category string
	}{
		{"deny", gateway.Outcome{Result: gateway.ResultDeny, Category: "policy-denied"}, gateway.ResultDeny, "policy-denied"},
		{"approval", gateway.Outcome{Result: gateway.ResultApprovalRequired, Category: "approval"}, gateway.ResultApprovalRequired, "approval"},
		{"invalid", gateway.Outcome{Result: gateway.DecisionResult("Maybe")}, gateway.ResultDeny, gateway.CategoryInvalidPolicyOutcome},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gw, claims, store, _, adapter := fixture(t, WithPolicy(func(Request) gateway.Outcome { return test.outcome }))
			claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
			decision := invoke(t, gw, req)
			if decision.Result != test.result || decision.Category != test.category {
				t.Fatalf("decision = %+v", decision)
			}
			facts := store.ToolInvocations(req.ClaimID)
			if len(adapter.calls) != 0 || len(facts) != 1 || facts[0].Result != test.result || facts[0].InvocationID != decision.InvocationID {
				t.Fatal("non-Allow must make zero adapter calls and record one correlated fact")
			}
		})
	}
}

func TestGatewayAdapterFailureRemainsSeparateFromAllowDecision(t *testing.T) {
	gw, claims, store, _, adapter := fixture(t)
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	adapter.err = errors.New("provider unavailable")
	decision, err := gw.Invoke(req)
	if err == nil || decision.Result != gateway.ResultAllow || len(adapter.calls) != 1 || len(store.ToolInvocations(req.ClaimID)) != 1 {
		t.Fatalf("decision=%+v err=%v calls=%d", decision, err, len(adapter.calls))
	}
}

func TestGatewayFailsClosedWithoutAdapterAndAcceptsNilOptionalDependencies(t *testing.T) {
	claims := gatewaytest.NewClaims()
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	gw := NewGateway(claims, nil, nil, WithAdapter(nil), WithIDSource(nil), WithPolicy(nil))
	decision, err := gw.Invoke(req)
	if err == nil || decision.Result != gateway.ResultAllow || decision.InvocationID == "" {
		t.Fatalf("decision=%+v err=%v, want correlatable Allow plus adapter error", decision, err)
	}
}

func TestGatewayPreservesExperimentalParentScopeWithoutMakingLineageRequired(t *testing.T) {
	gw, claims, store, lineage, adapter := fixture(t)
	req := teamARequest(t)
	parentID := "parent"
	claims.Put(parentID, v1alpha1.ClaimPhaseRunning)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	if err := lineage.RegisterChild(parentID, req.ClaimID); err != nil {
		t.Fatal(err)
	}
	if got := invoke(t, gw, req); got.Result != gateway.ResultAllow {
		t.Fatalf("running parent/child should allow: %+v", got)
	}
	claims.Put(parentID, v1alpha1.ClaimPhaseSucceeded)
	if got := invoke(t, gw, req); got.Result != gateway.ResultDeny || got.Category != gateway.CategoryOutOfParentScope {
		t.Fatalf("terminal parent should deny: %+v", got)
	}
	if len(adapter.calls) != 1 || len(store.ToolInvocations(req.ClaimID)) != 2 {
		t.Fatal("expected one Allow attempt and one recorded Deny")
	}
}
