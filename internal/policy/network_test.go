// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package policy

import "testing"

func TestNetworkConstraintsValidationAndImmutableCopies(t *testing.T) {
	for _, profiles := range [][]string{nil, {}, {"public-web"}, {"unknown"}, {"public-web", "public-web"}} {
		bundle := ReferenceBundle()
		bundle.Rules[0].Constraints = &AuthorityConstraints{NetworkProfiles: &StringSetConstraint{Values: profiles}}
		valid := len(profiles) == 0 || len(profiles) == 1 && profiles[0] == "public-web"
		if err := ValidateBundle(bundle); (err == nil) != valid {
			t.Fatalf("validation %q = %v, valid %v", profiles, err, valid)
		}
	}
	bundle := ReferenceBundle()
	bundle.Rules[0].Constraints = &AuthorityConstraints{NetworkProfiles: &StringSetConstraint{Values: []string{"public-web"}}}
	loader := &Loader{}
	if err := loader.Load(bundle); err != nil {
		t.Fatal(err)
	}
	bundle.Rules[0].Constraints.NetworkProfiles.Values[0] = "mutated"
	current, ok := loader.Current()
	if !ok || current.Rules[0].Constraints.NetworkProfiles.Values[0] != "public-web" {
		t.Fatal("loader network snapshot aliases source")
	}
	evaluator, ok := loader.Snapshot()
	if !ok {
		t.Fatal("snapshot unavailable")
	}
	evaluation, err := evaluator.Evaluate(validEvaluationContext())
	if err != nil {
		t.Fatal(err)
	}
	cloned := CloneEvaluation(evaluation)
	evaluation.Constraints.NetworkProfiles.Values[0] = "mutated"
	if cloned.Constraints.NetworkProfiles.Values[0] != "public-web" {
		t.Fatal("evaluation network snapshot aliases source")
	}
	if err := ValidateEvaluation(cloned.Context, evaluation); err == nil {
		t.Fatal("custom evaluator unsupported network profile accepted")
	}
	if err := ValidateEvaluation(cloned.Context, cloned); err != nil {
		t.Fatal(err)
	}
}
