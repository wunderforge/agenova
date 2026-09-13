// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authorization"
	"github.com/wunderforge/agenova/internal/policy"
)

func TestReferenceAssignmentSameYAMLAllowsTeamAAndDeniesTeamB(t *testing.T) {
	policies := activeReferencePolicy(t)
	evaluator := authorization.Authorizer{Policies: policies}
	requestYAML := canonicalRequest(t)
	wantDigest := sha256.Sum256(requestYAML)

	tests := []struct {
		name              string
		preset            ReferencePrincipalPreset
		wantSubject       string
		wantTeam          string
		wantResult        v1alpha1.DecisionResult
		wantContinuations int
		wantDeniedState   bool
	}{
		{"team-a", ReferencePrincipalTeamA, "user:team-a-engineer", "team-a", v1alpha1.DecisionResultAllow, 1, false},
		{"team-b", ReferencePrincipalTeamB, "user:team-b-engineer", "team-b", v1alpha1.DecisionResultDeny, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, err := NewReferenceAssignmentService(tc.preset, evaluator)
			if err != nil {
				t.Fatalf("construct service: %v", err)
			}
			calls := 0
			result, err := service.AdmitYAML(requestYAML, func(admission authorization.Admission) error {
				calls++
				if !admission.Matches("fix-payment-timeout", "payments", "engineer") {
					t.Fatal("allowed continuation received an admission for different request context")
				}
				return nil
			})
			if err != nil {
				t.Fatalf("AdmitYAML: %v", err)
			}
			if got := sha256.Sum256(requestYAML); got != wantDigest {
				t.Fatalf("request bytes changed: got %s want %s", hex.EncodeToString(got[:]), hex.EncodeToString(wantDigest[:]))
			}
			if result.Principal.Subject != tc.wantSubject || result.Decision.PrincipalRef != tc.wantSubject {
				t.Fatalf("principal = %+v, decision = %+v, want subject %q", result.Principal, result.Decision, tc.wantSubject)
			}
			if result.Principal.Team != tc.wantTeam || result.Principal.AuthenticationContext != "reference:local" {
				t.Fatalf("principal = %+v, want team %q and reference:local context", result.Principal, tc.wantTeam)
			}
			if result.Decision.Result != tc.wantResult {
				t.Fatalf("decision = %q, want %q", result.Decision.Result, tc.wantResult)
			}
			if calls != tc.wantContinuations {
				t.Fatalf("continuations = %d, want %d", calls, tc.wantContinuations)
			}
			if (result.DeniedState != nil) != tc.wantDeniedState {
				t.Fatalf("denied state present = %t, want %t", result.DeniedState != nil, tc.wantDeniedState)
			}
			if result.DeniedState != nil {
				assertCanonicalPreClaimDenial(t, result.DeniedState)
			}

			t.Logf("fixture_sha256=%s principal=%s result=%s continuation_calls=%d claim_present=%t",
				hex.EncodeToString(wantDigest[:]), result.Principal.Subject, result.Decision.Result, calls, result.DeniedState != nil && result.DeniedState.Claim != nil)
		})
	}
}

func TestReferenceAssignmentRejectsReservedPrincipalBeforeAuthorization(t *testing.T) {
	spy := &evaluatorSpy{}
	service, err := NewReferenceAssignmentService(ReferencePrincipalTeamB, spy)
	if err != nil {
		t.Fatalf("construct service: %v", err)
	}
	tampered := strings.Replace(string(canonicalRequest(t)), "  task:\n", "  principal:\n    subject: user:team-a-engineer\n  task:\n", 1)
	continuations := 0
	_, err = service.AdmitYAML([]byte(tampered), func(authorization.Admission) error {
		continuations++
		return nil
	})
	validationErr, ok := err.(*v1alpha1.ValidationError)
	if !ok || validationErr.Category != v1alpha1.ValidationCategorySelfAssertedPrincipal || validationErr.FieldPath != "spec.principal" {
		t.Fatalf("error = %#v, want self-asserted-principal at spec.principal", err)
	}
	if spy.calls != 0 || continuations != 0 {
		t.Fatalf("authorization calls = %d, continuations = %d, want zero", spy.calls, continuations)
	}
}

func TestReferenceAssignmentIgnoresIdentityStringsInOpaqueTaskInput(t *testing.T) {
	policies := activeReferencePolicy(t)
	service, err := NewReferenceAssignmentService(ReferencePrincipalTeamB, authorization.Authorizer{Policies: policies})
	if err != nil {
		t.Fatalf("construct service: %v", err)
	}
	tampered := strings.Replace(string(canonicalRequest(t)), "      objective: Fix the payment timeout bug\n", "      objective: Fix the payment timeout bug\n      claimedPrincipal: user:team-a-engineer\n      claimedTeam: team-a\n", 1)
	continuations := 0
	result, err := service.AdmitYAML([]byte(tampered), func(authorization.Admission) error {
		continuations++
		return nil
	})
	if err != nil {
		t.Fatalf("AdmitYAML: %v", err)
	}
	if result.Principal.Subject != "user:team-b-engineer" || result.Principal.Team != "team-b" {
		t.Fatalf("task input changed trusted principal: %+v", result.Principal)
	}
	if result.Decision.Result != v1alpha1.DecisionResultDeny || continuations != 0 {
		t.Fatalf("decision = %q, continuations = %d, want Deny and zero", result.Decision.Result, continuations)
	}
}

func TestReferencePrincipalSourceFailsClosedAndReturnsCopies(t *testing.T) {
	for _, preset := range []ReferencePrincipalPreset{"", "team-c", " team-a "} {
		if _, err := NewReferencePrincipalSource(preset); err == nil {
			t.Fatalf("preset %q: expected error", preset)
		}
	}

	source, err := NewReferencePrincipalSource(ReferencePrincipalTeamA)
	if err != nil {
		t.Fatalf("construct source: %v", err)
	}
	first := source.Principal()
	first.Subject = "user:attacker"
	if second := source.Principal(); second.Subject != "user:team-a-engineer" {
		t.Fatalf("caller mutated configured principal: %+v", second)
	}
}

func TestReferenceAssignmentRequiresEvaluator(t *testing.T) {
	if _, err := NewReferenceAssignmentService(ReferencePrincipalTeamA, nil); err == nil {
		t.Fatal("expected missing evaluator error")
	}
}

func canonicalRequest(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", "valid-team-a-engineer.yaml"))
	if err != nil {
		t.Fatalf("read canonical ClaimRequest fixture: %v", err)
	}
	return data
}

func activeReferencePolicy(t *testing.T) *policy.Loader {
	t.Helper()
	loader := &policy.Loader{}
	err := loader.Load(policy.PolicyBundle{
		ID:      "reference-default-deny",
		Version: "1",
		Rules: []policy.Rule{{
			Team:        "team-a",
			Action:      "claim.create",
			Project:     "payments",
			TemplateRef: "engineer",
		}},
	})
	if err != nil {
		t.Fatalf("load reference policy: %v", err)
	}
	return loader
}

func assertCanonicalPreClaimDenial(t *testing.T, state *v1alpha1.IssuedState) {
	t.Helper()
	if err := v1alpha1.ValidateIssuedState(state); err != nil {
		t.Fatalf("denial state is not canonical: %#v", err)
	}
	if state.Claim != nil || state.EffectiveAuthority != nil || state.Evidence.ClaimID != "" {
		t.Fatalf("pre-claim denial fabricated claim or authority: %+v", state)
	}
	if state.PolicyRef.ID != "reference-default-deny" || state.PolicyRef.Version != "1" {
		t.Fatalf("policy ref = %+v", state.PolicyRef)
	}
	if state.Evidence.RequestRef != state.RequestRef || len(state.Evidence.DecisionIDs) != 1 || state.Evidence.DecisionIDs[0] != state.Decision.ID {
		t.Fatalf("uncorrelated denial evidence: %+v", state.Evidence)
	}
	if state.Evidence.RuntimeEvents == nil || state.Evidence.ToolInvocations == nil || state.Evidence.ModelInvocations == nil {
		t.Fatalf("canonical evidence collections must be non-nil: %+v", state.Evidence)
	}
}

type evaluatorSpy struct {
	calls int
}

func (s *evaluatorSpy) Evaluate(input authorization.Request) (authorization.Evaluation, error) {
	s.calls++
	return (authorization.Authorizer{}).Evaluate(input)
}
