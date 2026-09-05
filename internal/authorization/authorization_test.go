// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package authorization

import (
	"os"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

func TestCanonicalTeamAAllowAndTeamBDenyBeforeSideEffects(t *testing.T) {
	request := loadRequestFixture(t)
	teamA := loadIssuedFixture(t, "valid-team-a-engineer.json").Principal
	teamB := loadIssuedFixture(t, "valid-team-b-denial.json").Principal
	loader := matchingPolicy(t)
	gate := Gate{Evaluator: Authorizer{Policies: loader}}

	for _, test := range []struct {
		name      string
		principal v1alpha1.Principal
		want      v1alpha1.DecisionResult
		calls     int
	}{
		{"canonical Team A allow", teamA, v1alpha1.DecisionResultAllow, 1},
		{"canonical Team B pre-claim denial", teamB, v1alpha1.DecisionResultDeny, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			claimCalls, backendCalls := 0, 0
			decision, err := gate.Admit(inputFor(request, test.principal), func(v1alpha1.Decision) error {
				claimCalls++
				backendCalls++
				return nil
			})
			if err != nil {
				t.Fatalf("Admit() error = %v", err)
			}
			if decision.Result != test.want {
				t.Fatalf("result = %q, want %q", decision.Result, test.want)
			}
			if claimCalls != test.calls || backendCalls != test.calls {
				t.Fatalf("side effects = claim:%d backend:%d, want %d each", claimCalls, backendCalls, test.calls)
			}
			if decision.PrincipalRef != test.principal.Subject || decision.Action != "claim.create" || decision.Reason == "" {
				t.Fatalf("decision is not evidence-ready: %+v", decision)
			}
		})
	}
}

func TestAuthorizerDefaultsToDenyWithoutPolicyOrExactMatch(t *testing.T) {
	request := loadRequestFixture(t)
	base := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)

	tests := map[string]struct {
		authorizer Authorizer
		mutate     func(*Request)
	}{
		"missing policy": {authorizer: Authorizer{}},
		"unknown team": {authorizer: Authorizer{Policies: matchingPolicy(t)}, mutate: func(in *Request) {
			in.Principal.Team = "unknown"
		}},
		"unknown action": {authorizer: Authorizer{Policies: matchingPolicy(t)}, mutate: func(in *Request) {
			in.Action.Name = "claim.delete"
		}},
		"unknown project": {authorizer: Authorizer{Policies: matchingPolicy(t)}, mutate: func(in *Request) {
			in.Action.Project = "ledger"
		}},
		"unknown template": {authorizer: Authorizer{Policies: matchingPolicy(t)}, mutate: func(in *Request) {
			in.Action.TemplateRef = "reviewer"
		}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			input := base
			if test.mutate != nil {
				test.mutate(&input)
			}
			calls := 0
			decision, err := (Gate{Evaluator: test.authorizer}).Admit(input, func(v1alpha1.Decision) error {
				calls++
				return nil
			})
			if err != nil {
				t.Fatalf("Admit() error = %v", err)
			}
			if decision.Result != v1alpha1.DecisionResultDeny || decision.Reason == "" || calls != 0 {
				t.Fatalf("decision/calls = %+v/%d, want evidence-ready deny and zero calls", decision, calls)
			}
		})
	}
}

func TestAuthorizerRejectsMissingContext(t *testing.T) {
	request := loadRequestFixture(t)
	base := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	tests := map[string]func(*Request){
		"requestRef":                      func(in *Request) { in.RequestRef = "" },
		"principal.subject":               func(in *Request) { in.Principal.Subject = "" },
		"principal.team":                  func(in *Request) { in.Principal.Team = "" },
		"principal.authenticationContext": func(in *Request) { in.Principal.AuthenticationContext = "" },
		"action.name":                     func(in *Request) { in.Action.Name = "" },
		"action.project":                  func(in *Request) { in.Action.Project = "" },
		"action.templateRef":              func(in *Request) { in.Action.TemplateRef = "" },
	}
	for path, mutate := range tests {
		t.Run(path, func(t *testing.T) {
			input := base
			mutate(&input)
			calls := 0
			_, err := (Gate{Evaluator: Authorizer{Policies: matchingPolicy(t)}}).Admit(input, func(v1alpha1.Decision) error {
				calls++
				return nil
			})
			validationErr, ok := err.(*v1alpha1.ValidationError)
			if !ok || validationErr.FieldPath != path || calls != 0 {
				t.Fatalf("error/calls = %#v/%d, want validation at %q and zero calls", err, calls, path)
			}
		})
	}
}

func TestGateDoesNotTreatApprovalRequiredAsAllow(t *testing.T) {
	calls := 0
	gate := Gate{Evaluator: fixedEvaluator{decision: v1alpha1.Decision{Result: v1alpha1.DecisionResultApprovalRequired}}}
	decision, err := gate.Admit(Request{}, func(v1alpha1.Decision) error { calls++; return nil })
	if err != nil || decision.Result != v1alpha1.DecisionResultApprovalRequired || calls != 0 {
		t.Fatalf("decision/error/calls = %+v/%v/%d", decision, err, calls)
	}
}

type fixedEvaluator struct{ decision v1alpha1.Decision }

func (f fixedEvaluator) Evaluate(Request) (v1alpha1.Decision, error) { return f.decision, nil }

func inputFor(request *v1alpha1.ClaimRequest, principal v1alpha1.Principal) Request {
	return Request{
		RequestRef: request.Metadata.Name,
		Principal:  principal,
		Action: v1alpha1.Action{
			Name:        "claim.create",
			Project:     request.Spec.ProjectRef,
			TemplateRef: request.Spec.TemplateRef,
		},
	}
}

func matchingPolicy(t *testing.T) *policy.Loader {
	t.Helper()
	loader := &policy.Loader{}
	err := loader.Load(policy.PolicyBundle{
		ID: "reference-default-deny", Version: "1",
		Rules: []policy.Rule{{Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer"}},
	})
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	return loader
}

func loadRequestFixture(t *testing.T) *v1alpha1.ClaimRequest {
	t.Helper()
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml")
	if err != nil {
		t.Fatal(err)
	}
	request, validationErr := v1alpha1.ParseClaimRequestYAML(data)
	if validationErr != nil {
		t.Fatal(validationErr)
	}
	return request
}

func loadIssuedFixture(t *testing.T, name string) *v1alpha1.IssuedState {
	t.Helper()
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/issued-state/" + name)
	if err != nil {
		t.Fatal(err)
	}
	state, validationErr := v1alpha1.ParseSystemIssuedState(data)
	if validationErr != nil {
		t.Fatal(validationErr)
	}
	return state
}
