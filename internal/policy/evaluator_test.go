// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"
	"time"
)

func TestReferenceEvaluatorPreservesExactMatchBehavior(t *testing.T) {
	evaluator, err := NewReferenceEvaluator(ReferenceBundle())
	if err != nil {
		t.Fatal(err)
	}
	context := validEvaluationContext()

	allowed, err := evaluator.Evaluate(context)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateEvaluation(context, allowed); err != nil {
		t.Fatalf("valid allow rejected: %v", err)
	}
	if allowed.Decision != DecisionAllow || allowed.PolicyRef != (PolicyReference{ID: "reference-default-deny", Version: "1"}) {
		t.Fatalf("allowed evaluation = %+v", allowed)
	}

	context.Principal.Team = "team-b"
	denied, err := evaluator.Evaluate(context)
	if err != nil {
		t.Fatal(err)
	}
	if denied.Decision != DecisionDeny || denied.Reasons[0].Code != "reference-no-exact-match" {
		t.Fatalf("denied evaluation = %+v", denied)
	}
}

func TestLoaderSnapshotDoesNotChangeAfterReload(t *testing.T) {
	loader := &Loader{}
	if err := loader.Load(ReferenceBundle()); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := loader.Snapshot()
	if !ok {
		t.Fatal("Snapshot() = unavailable")
	}
	if err := loader.Load(PolicyBundle{ID: "reference-default-deny", Version: "2"}); err != nil {
		t.Fatal(err)
	}

	evaluation, err := snapshot.Evaluate(validEvaluationContext())
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Decision != DecisionAllow || evaluation.PolicyRef.Version != "1" {
		t.Fatalf("snapshot changed after reload: %+v", evaluation)
	}
}

func TestCloneEvaluationDefensivelyCopiesNestedValues(t *testing.T) {
	timeout := 5 * time.Minute
	original := Evaluation{
		Context:   validEvaluationContext(),
		Decision:  DecisionAllow,
		PolicyRef: PolicyReference{ID: "policy", Version: "1"},
		Reasons:   []Reason{{Code: "allowed", Message: "allowed"}},
		Constraints: &AuthorityConstraints{
			Tools:      &StringSetConstraint{Values: []string{"git.read"}},
			MaxTimeout: &timeout,
		},
	}
	copy := CloneEvaluation(original)
	original.Context.Environment[0].Value = "mutated"
	original.Reasons[0].Message = "mutated"
	original.Constraints.Tools.Values[0] = "git.write"
	*original.Constraints.MaxTimeout = time.Hour

	if copy.Context.Environment[0].Value != "kind" || copy.Reasons[0].Message != "allowed" ||
		copy.Constraints.Tools.Values[0] != "git.read" || *copy.Constraints.MaxTimeout != 5*time.Minute {
		t.Fatalf("clone retained caller-owned memory: %+v", copy)
	}
}

func TestValidateEvaluationFailsClosed(t *testing.T) {
	context := validEvaluationContext()
	valid := Evaluation{
		Context:   context,
		Decision:  DecisionAllow,
		PolicyRef: PolicyReference{ID: "policy", Version: "1"},
		Reasons:   []Reason{{Code: "allowed", Message: "allowed"}},
	}
	tests := map[string]func(*Evaluation){
		"different context": func(e *Evaluation) { e.Context.Resource.Project = "ledger" },
		"invalid decision":  func(e *Evaluation) { e.Decision = Decision("maybe") },
		"missing policy":    func(e *Evaluation) { e.PolicyRef = PolicyReference{} },
		"missing reason":    func(e *Evaluation) { e.Reasons = nil },
		"blank constraint": func(e *Evaluation) {
			e.Constraints = &AuthorityConstraints{Tools: &StringSetConstraint{Values: []string{" "}}}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			evaluation := CloneEvaluation(valid)
			mutate(&evaluation)
			if err := ValidateEvaluation(context, evaluation); err == nil {
				t.Fatal("malformed evaluation accepted")
			}
		})
	}
}

func validEvaluationContext() EvaluationContext {
	return EvaluationContext{
		Principal:   PrincipalContext{Subject: "user:team-a", Team: "team-a", AuthenticationContext: "fixture"},
		Action:      "claim.create",
		Resource:    ResourceContext{RequestRef: "fix-payment-timeout", Project: "payments", TemplateRef: "engineer"},
		Environment: []EnvironmentFact{{Name: "deployment", Value: "kind"}},
	}
}
