// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelgateway

import (
	"errors"
	"fmt"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
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
	return Request{ClaimID: authority.ClaimID, Profile: authority.ModelProfile}
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
		t.Fatalf("decision = %+v", decision)
	}
	if len(adapter.calls) != 1 || adapter.calls[0].id != decision.InvocationID {
		t.Fatalf("adapter calls = %+v", adapter.calls)
	}
	found := store.ModelInvocations(req.ClaimID)
	if len(found) != 1 || found[0].ModelName != req.Profile || found[0].InvocationID != decision.InvocationID || found[0].Result != gateway.ResultAllow {
		t.Fatalf("facts = %+v", found)
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

func TestGatewayEnforcesEffectiveModelProfileBeforePolicy(t *testing.T) {
	policyCalls := 0
	gw, claims, store, _, adapter := fixture(t, WithPolicy(func(Request) gateway.Outcome {
		policyCalls++
		return gateway.Allowed()
	}))
	req := teamARequest(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	req.Profile = "ungranted-premium-model"

	decision := invoke(t, gw, req)
	if decision.Result != gateway.ResultDeny || decision.Category != gateway.CategoryModelProfileNotGranted {
		t.Fatalf("decision = %+v, want Deny/%s", decision, gateway.CategoryModelProfileNotGranted)
	}
	if policyCalls != 0 || len(adapter.calls) != 0 {
		t.Fatalf("authority denial reached policy or provider: policy=%d adapter=%d", policyCalls, len(adapter.calls))
	}
	found := store.ModelInvocations(req.ClaimID)
	if len(found) != 1 || found[0].Result != gateway.ResultDeny || found[0].InvocationID != decision.InvocationID {
		t.Fatalf("denial fact = %+v, want one correlated Deny", found)
	}
}

func TestGatewayRejectsMissingOrMismatchedModelAuthority(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string, *gatewaytest.Claims)
	}{
		{
			name: "missing",
			mutate: func(claimID string, claims *gatewaytest.Claims) {
				delete(claims.Authorities, claimID)
			},
		},
		{
			name: "mismatched",
			mutate: func(claimID string, claims *gatewaytest.Claims) {
				authority := claims.Authorities[claimID]
				authority.ID = "authority:other"
				claims.Authorities[claimID] = authority
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			gw, claims, store, _, adapter := fixture(t)
			req := teamARequest(t)
			claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
			test.mutate(req.ClaimID, claims)
			decision := invoke(t, gw, req)
			if decision.Result != gateway.ResultDeny || decision.Category != gateway.CategoryAuthorityUnavailable {
				t.Fatalf("decision = %+v", decision)
			}
			if len(adapter.calls) != 0 || len(store.ModelInvocations(req.ClaimID)) != 1 {
				t.Fatal("invalid authority must record one Deny and make zero provider calls")
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
		{"missing profile", func(r *Request) { r.Profile = "\t" }, gateway.CategoryIncompleteOperation},
		{"reserved credential", func(r *Request) { r.Parameters = map[string]string{gatewaytest.SecretParameterKey(t): "not-inspected"} }, gateway.CategorySecretValue},
		{"OAuth client secret", func(r *Request) { r.Parameters = map[string]string{"client_secret": "not-inspected"} }, gateway.CategorySecretValue},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gw, claims, store, _, adapter := fixture(t)
			claims.Put(base.ClaimID, v1alpha1.ClaimPhaseRunning)
			req := base
			test.mutate(&req)
			decision := invoke(t, gw, req)
			if decision.Result != gateway.ResultDeny || decision.Category != test.category || decision.InvocationID == "" {
				t.Fatalf("decision = %+v", decision)
			}
			if len(adapter.calls) != 0 || len(store.ModelInvocations(req.ClaimID)) != 0 {
				t.Fatal("pre-resolution rejection reached adapter or fabricated a fact")
			}
		})
	}
}

func TestGatewayUnknownAndInactiveClaimsFailClosed(t *testing.T) {
	req := teamARequest(t)
	gw, claims, store, _, adapter := fixture(t)
	unknown := invoke(t, gw, req)
	if unknown.Result != gateway.ResultDeny || unknown.Category != gateway.CategoryUnknownClaim || len(store.ModelInvocations(req.ClaimID)) != 0 {
		t.Fatalf("unknown decision=%+v facts=%+v", unknown, store.ModelInvocations(req.ClaimID))
	}
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseSucceeded)
	inactive := invoke(t, gw, req)
	if inactive.Result != gateway.ResultDeny || inactive.Category != gateway.CategoryClaimNotActive || len(store.ModelInvocations(req.ClaimID)) != 1 || len(adapter.calls) != 0 {
		t.Fatalf("inactive decision=%+v facts=%+v calls=%d", inactive, store.ModelInvocations(req.ClaimID), len(adapter.calls))
	}
	malformed := gatewaytest.ClaimSnapshot(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	malformed.BackendIdentity = nil
	claims.Items[req.ClaimID] = malformed
	if got := invoke(t, gw, req); got.Result != gateway.ResultDeny || got.Category != gateway.CategoryClaimNotActive {
		t.Fatalf("malformed Running snapshot was not denied: %+v", got)
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
			found := store.ModelInvocations(req.ClaimID)
			if decision.Result != test.result || decision.Category != test.category || len(adapter.calls) != 0 || len(found) != 1 || found[0].Result != test.result || found[0].InvocationID != decision.InvocationID {
				t.Fatalf("decision=%+v facts=%+v calls=%d", decision, found, len(adapter.calls))
			}
		})
	}
}

func TestGatewayDoesNotAdoptCallerIdentifier(t *testing.T) {
	gw, claims, _, _, _ := fixture(t)
	req := teamARequest(t)
	req.CallerReference = "caller-chosen"
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	decision := invoke(t, gw, req)
	if decision.InvocationID != "inv-test-1" || decision.InvocationID == req.CallerReference {
		t.Fatalf("decision id=%q caller=%q", decision.InvocationID, req.CallerReference)
	}
}

func TestGatewayAdapterFailureAndUnconfiguredAdapterRemainExecutionErrors(t *testing.T) {
	req := teamARequest(t)
	gw, claims, store, _, adapter := fixture(t)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	adapter.err = errors.New("provider unavailable")
	decision, err := gw.Invoke(req)
	if err == nil || decision.Result != gateway.ResultAllow || len(adapter.calls) != 1 || len(store.ModelInvocations(req.ClaimID)) != 1 {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}

	withoutAdapter := NewGateway(claims, nil, nil, WithAdapter(nil), WithIDSource(nil), WithPolicy(nil))
	decision, err = withoutAdapter.Invoke(req)
	if err == nil || decision.Result != gateway.ResultAllow || decision.InvocationID == "" {
		t.Fatalf("unconfigured decision=%+v err=%v", decision, err)
	}
}

func TestGatewayPreservesExperimentalParentScope(t *testing.T) {
	gw, claims, store, lineage, adapter := fixture(t)
	req := teamARequest(t)
	claims.Put("parent", v1alpha1.ClaimPhaseRunning)
	claims.Put(req.ClaimID, v1alpha1.ClaimPhaseRunning)
	if err := lineage.RegisterChild("parent", req.ClaimID); err != nil {
		t.Fatal(err)
	}
	if got := invoke(t, gw, req); got.Result != gateway.ResultAllow {
		t.Fatalf("running parent/child should allow: %+v", got)
	}
	claims.Put("parent", v1alpha1.ClaimPhaseFailed)
	if got := invoke(t, gw, req); got.Result != gateway.ResultDeny || got.Category != gateway.CategoryOutOfParentScope {
		t.Fatalf("terminal parent should deny: %+v", got)
	}
	if len(adapter.calls) != 1 || len(store.ModelInvocations(req.ClaimID)) != 2 {
		t.Fatal("expected one attempted Allow and one recorded Deny")
	}
}
