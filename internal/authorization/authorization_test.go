// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package authorization

import (
	"os"
	"testing"
	"time"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

func TestEvaluatorContractReferenceAndExternal(t *testing.T) {
	request := loadRequestFixture(t)
	teamA := loadIssuedFixture(t, "valid-team-a-engineer.json").Principal
	teamB := loadIssuedFixture(t, "valid-team-b-denial.json").Principal
	load := matchingPolicy(t)
	sources := map[string]policy.SnapshotSource{
		"reference": load,
		"external fake": staticPolicySource{evaluator: policyEvaluatorFunc(func(context policy.EvaluationContext) (policy.Evaluation, error) {
			result := policy.Evaluation{
				Context:   context,
				Decision:  policy.DecisionDeny,
				PolicyRef: policy.PolicyReference{ID: "company-policy", Version: "2026-10-06"},
				Reasons:   []policy.Reason{{Code: "company-deny", Message: "company policy denied the assignment"}},
			}
			if context.Principal.Subject == teamA.Subject {
				result.Decision = policy.DecisionAllow
				result.Reasons = []policy.Reason{{Code: "company-allow", Message: "company policy allowed the assignment"}}
			}
			return result, nil
		})},
	}

	for sourceName, source := range sources {
		t.Run(sourceName, func(t *testing.T) {
			for _, test := range []struct {
				name      string
				principal v1alpha1.Principal
				want      v1alpha1.DecisionResult
				calls     int
			}{
				{"allow", teamA, v1alpha1.DecisionResultAllow, 1},
				{"deny", teamB, v1alpha1.DecisionResultDeny, 0},
			} {
				t.Run(test.name, func(t *testing.T) {
					calls := 0
					decision, err := (Gate{Evaluator: Authorizer{Policies: source}}).Admit(inputFor(request, test.principal), func(Admission) error {
						calls++
						return nil
					})
					if err != nil || decision.Result != test.want || calls != test.calls {
						t.Fatalf("decision/error/calls = %+v/%v/%d, want %s/<nil>/%d", decision, err, calls, test.want, test.calls)
					}
				})
			}
		})
	}
}

func TestAdmissionDefensivelyCopiesExternalConstraints(t *testing.T) {
	request := loadRequestFixture(t)
	input := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	timeout := 5 * time.Minute
	returned := policy.Evaluation{
		Context:   policyContext(input),
		Decision:  policy.DecisionAllow,
		PolicyRef: policy.PolicyReference{ID: "company-policy", Version: "1"},
		Reasons:   []policy.Reason{{Code: "company-allow", Message: "allowed"}},
		Constraints: &policy.AuthorityConstraints{
			Tools:      &policy.StringSetConstraint{Values: []string{"git.read"}},
			MaxTimeout: &timeout,
		},
	}
	source := staticPolicySource{evaluator: policyEvaluatorFunc(func(policy.EvaluationContext) (policy.Evaluation, error) {
		return returned, nil
	})}
	var admission Admission
	_, err := (Gate{Evaluator: Authorizer{Policies: source}}).Admit(input, func(value Admission) error {
		admission = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	returned.Constraints.Tools.Values[0] = "git.write"
	*returned.Constraints.MaxTimeout = time.Hour
	first := admission.Constraints()
	first.Tools.Values[0] = "mutated-by-consumer"
	second := admission.Constraints()
	if second.Tools.Values[0] != "git.read" || *second.MaxTimeout != 5*time.Minute {
		t.Fatalf("admission constraints retained mutable caller memory: %+v", second)
	}
}

func TestExternalEvaluatorContextMismatchFailsBeforeContinuation(t *testing.T) {
	request := loadRequestFixture(t)
	input := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	source := staticPolicySource{evaluator: policyEvaluatorFunc(func(context policy.EvaluationContext) (policy.Evaluation, error) {
		context.Resource.Attributes[0].Values[0] = "ledger"
		return policy.Evaluation{
			Context: context, Decision: policy.DecisionAllow,
			PolicyRef: policy.PolicyReference{ID: "company-policy", Version: "1"},
			Reasons:   []policy.Reason{{Code: "allow", Message: "allowed"}},
		}, nil
	})}
	calls := 0
	_, err := (Gate{Evaluator: Authorizer{Policies: source}}).Admit(input, func(Admission) error { calls++; return nil })
	validationErr, ok := err.(*v1alpha1.ValidationError)
	if !ok || validationErr.FieldPath != "evaluation.request" || calls != 0 {
		t.Fatalf("error/calls = %#v/%d, want evaluation.request and zero calls", err, calls)
	}
}

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
			input := inputFor(request, test.principal)
			decision, err := gate.Admit(input, func(admission Admission) error {
				if !admission.Matches(input.RequestRef, input.Action.Project, input.Action.TemplateRef) {
					t.Fatal("allow continuation received an admission bound to different request context")
				}
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
		"unknown team": {authorizer: Authorizer{Policies: matchingPolicy(t)}, mutate: func(in *Request) {
			in.Principal.Team = "unknown"
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
			decision, err := (Gate{Evaluator: test.authorizer}).Admit(input, func(Admission) error {
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

func TestAuthorizerFailsClosedWithoutPolicySnapshot(t *testing.T) {
	request := loadRequestFixture(t)
	input := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	for name, authorizer := range map[string]Authorizer{
		"missing source":   {},
		"missing snapshot": {Policies: staticPolicySource{}},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			_, err := (Gate{Evaluator: authorizer}).Admit(input, func(Admission) error { calls++; return nil })
			validationErr, ok := err.(*v1alpha1.ValidationError)
			if !ok || validationErr.FieldPath != "evaluator.snapshot" || calls != 0 {
				t.Fatalf("error/calls = %#v/%d, want evaluator.snapshot and zero calls", err, calls)
			}
		})
	}
}

func TestAuthorizerRejectsNonCreateActionEvenWhenPolicyAllowsIt(t *testing.T) {
	request := loadRequestFixture(t)
	input := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	input.Action.Name = "claim.delete"
	loader := &policy.Loader{}
	if err := loader.Load(policy.PolicyBundle{
		ID: "reference-default-deny", Version: "1",
		Rules: []policy.Rule{{Team: input.Principal.Team, Action: "claim.delete", Project: input.Action.Project, TemplateRef: input.Action.TemplateRef}},
	}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err := (Gate{Evaluator: Authorizer{Policies: loader}}).Admit(input, func(Admission) error {
		calls++
		return nil
	})
	validationErr, ok := err.(*v1alpha1.ValidationError)
	if !ok || validationErr.FieldPath != "action.name" || calls != 0 {
		t.Fatalf("error/calls = %#v/%d, want action.name validation and zero calls", err, calls)
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
			_, err := (Gate{Evaluator: Authorizer{Policies: matchingPolicy(t)}}).Admit(input, func(Admission) error {
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
	request := loadRequestFixture(t)
	input := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	calls := 0
	evaluation := completeEvaluation(input, v1alpha1.Decision{
		PrincipalRef: input.Principal.Subject,
		Action:       input.Action.Name,
		Result:       v1alpha1.DecisionResultApprovalRequired,
		PolicyRef:    v1alpha1.PolicyReference{ID: "reference-default-deny", Version: "1"},
		Reason:       "approval is required",
	})
	gate := Gate{Evaluator: fixedEvaluator{evaluation: evaluation}}
	decision, err := gate.Admit(input, func(Admission) error { calls++; return nil })
	if err != nil || decision.Result != v1alpha1.DecisionResultApprovalRequired || calls != 0 {
		t.Fatalf("decision/error/calls = %+v/%v/%d", decision, err, calls)
	}
}

func TestGateRejectsMalformedAllowBeforeContinuation(t *testing.T) {
	request := loadRequestFixture(t)
	input := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	validEvaluation, err := (Authorizer{Policies: matchingPolicy(t)}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	valid := validEvaluation.Decision()

	tests := map[string]struct {
		path   string
		mutate func(*v1alpha1.Decision)
	}{
		"missing id":            {"decision.id", func(d *v1alpha1.Decision) { d.ID = "" }},
		"blank id":              {"decision.id", func(d *v1alpha1.Decision) { d.ID = " \t" }},
		"wrong principal":       {"decision.principalRef", func(d *v1alpha1.Decision) { d.PrincipalRef = "user:other" }},
		"wrong action":          {"decision.action", func(d *v1alpha1.Decision) { d.Action = "claim.delete" }},
		"missing policy":        {"decision.policyRef", func(d *v1alpha1.Decision) { d.PolicyRef = v1alpha1.PolicyReference{} }},
		"blank policy identity": {"decision.policyRef", func(d *v1alpha1.Decision) { d.PolicyRef.ID = "  " }},
		"missing explanation":   {"decision.reason", func(d *v1alpha1.Decision) { d.Reason = "" }},
		"blank explanation":     {"decision.reason", func(d *v1alpha1.Decision) { d.Reason = "\t" }},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			decision := valid
			test.mutate(&decision)
			calls := 0
			evaluation := completeEvaluation(input, decision)
			// Preserve the deliberately malformed public value after the helper
			// constructs the complete generic Policy snapshot for this context.
			evaluation.decision = decision
			_, validationErr := (Gate{Evaluator: fixedEvaluator{evaluation: evaluation}}).Admit(input, func(Admission) error {
				calls++
				return nil
			})
			got, ok := validationErr.(*v1alpha1.ValidationError)
			if !ok || got.FieldPath != test.path || calls != 0 {
				t.Fatalf("error/calls = %#v/%d, want validation at %q and zero calls", validationErr, calls, test.path)
			}
		})
	}
}

func TestGateValidatesInputBeforeEvaluator(t *testing.T) {
	request := loadRequestFixture(t)
	input := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	input.Action.Name = "claim.delete"
	evaluator := &spyEvaluator{}
	calls := 0

	_, err := (Gate{Evaluator: evaluator}).Admit(input, func(Admission) error { calls++; return nil })
	validationErr, ok := err.(*v1alpha1.ValidationError)
	if !ok || validationErr.FieldPath != "action.name" || evaluator.calls != 0 || calls != 0 {
		t.Fatalf("error/evaluator/continuation calls = %#v/%d/%d, want action.name validation and zero calls", err, evaluator.calls, calls)
	}
}

func TestGateRejectsEvaluationBoundToDifferentRequest(t *testing.T) {
	request := loadRequestFixture(t)
	principal := loadIssuedFixture(t, "valid-team-a-engineer.json").Principal
	original := inputFor(request, principal)
	evaluation, err := (Authorizer{Policies: matchingPolicy(t)}).Evaluate(original)
	if err != nil {
		t.Fatal(err)
	}
	replayed := original
	replayed.Action.Project = "ledger"
	calls := 0

	_, err = (Gate{Evaluator: fixedEvaluator{evaluation: evaluation}}).Admit(replayed, func(Admission) error { calls++; return nil })
	validationErr, ok := err.(*v1alpha1.ValidationError)
	if !ok || validationErr.FieldPath != "evaluation.request" || calls != 0 {
		t.Fatalf("error/calls = %#v/%d, want bound-request validation and zero calls", err, calls)
	}
}

func TestDecisionIDsDistinguishEvaluationContextAndPolicyVersion(t *testing.T) {
	request := loadRequestFixture(t)
	teamA := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	teamB := inputFor(request, loadIssuedFixture(t, "valid-team-b-denial.json").Principal)
	teamAEvaluation, err := (Authorizer{Policies: matchingPolicy(t)}).Evaluate(teamA)
	if err != nil {
		t.Fatal(err)
	}
	teamBEvaluation, err := (Authorizer{Policies: matchingPolicy(t)}).Evaluate(teamB)
	if err != nil {
		t.Fatal(err)
	}
	versionTwo := &policy.Loader{}
	if err := versionTwo.Load(policy.PolicyBundle{
		ID: "reference-default-deny", Version: "2",
		Rules: []policy.Rule{{Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer"}},
	}); err != nil {
		t.Fatal(err)
	}
	versionTwoEvaluation, err := (Authorizer{Policies: versionTwo}).Evaluate(teamA)
	if err != nil {
		t.Fatal(err)
	}

	teamAID := teamAEvaluation.Decision().ID
	if teamAID == teamBEvaluation.Decision().ID || teamAID == versionTwoEvaluation.Decision().ID {
		t.Fatalf("decision IDs must distinguish principal and policy context: %q", teamAID)
	}
}

func TestDecisionIDEncodingIsUnambiguousWhenFieldsContainNUL(t *testing.T) {
	request := loadRequestFixture(t)
	base := inputFor(request, loadIssuedFixture(t, "valid-team-a-engineer.json").Principal)
	first := base
	first.Principal.Subject = "a\x00b"
	first.Principal.Team = "c"
	second := base
	second.Principal.Subject = "a"
	second.Principal.Team = "b\x00c"
	decision := v1alpha1.Decision{
		Action:    assignmentCreateAction,
		Result:    v1alpha1.DecisionResultDeny,
		PolicyRef: v1alpha1.PolicyReference{ID: "reference-default-deny", Version: "1"},
		Reason:    "no exact policy rule matched the trusted principal and requested assignment",
	}

	if decisionID(first, decision) == decisionID(second, decision) {
		t.Fatal("length-prefixed decision ID encoding must distinguish field boundaries")
	}
}

type fixedEvaluator struct{ evaluation Evaluation }

func (f fixedEvaluator) Evaluate(Request) (Evaluation, error) { return f.evaluation, nil }

type spyEvaluator struct{ calls int }

func (s *spyEvaluator) Evaluate(Request) (Evaluation, error) {
	s.calls++
	return Evaluation{}, nil
}

type policyEvaluatorFunc func(policy.EvaluationContext) (policy.Evaluation, error)

func (f policyEvaluatorFunc) Evaluate(context policy.EvaluationContext) (policy.Evaluation, error) {
	return f(context)
}

type staticPolicySource struct{ evaluator policy.Evaluator }

func (s staticPolicySource) Snapshot() (policy.Evaluator, bool) {
	return s.evaluator, s.evaluator != nil
}

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
