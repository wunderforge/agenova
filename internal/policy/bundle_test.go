// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package policy

import "testing"

func TestLoaderLoadsVersionedBundle(t *testing.T) {
	loader := &Loader{}
	bundle := validBundle()

	if err := loader.Load(bundle); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got, ok := loader.Current()
	if !ok {
		t.Fatal("Current() did not return the loaded bundle")
	}
	if got.ID != "reference" || got.Version != "2026-08-24" {
		t.Fatalf("Current() identity = %q@%q", got.ID, got.Version)
	}
	if !got.Allows("claim.create", "payments", "engineer") {
		t.Fatal("Allows() denied the exact configured rule")
	}

	got.Rules[0].Action = "changed"
	again, _ := loader.Current()
	if again.Rules[0].Action != "claim.create" {
		t.Fatal("Current() exposed mutable loader state")
	}
}

func TestPolicyBundleDeniesUnmatchedRules(t *testing.T) {
	bundle := validBundle()
	tests := map[string]struct {
		action, project, template string
	}{
		"missing action":   {project: "payments", template: "engineer"},
		"unknown action":   {action: "claim.delete", project: "payments", template: "engineer"},
		"unknown project":  {action: "claim.create", project: "ledger", template: "engineer"},
		"unknown template": {action: "claim.create", project: "payments", template: "reviewer"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if bundle.Allows(test.action, test.project, test.template) {
				t.Fatal("Allows() allowed an unmatched rule")
			}
		})
	}
}

func TestLoaderRejectsMalformedAndDuplicateRulesWithoutReplacement(t *testing.T) {
	loader := &Loader{}
	if err := loader.Load(validBundle()); err != nil {
		t.Fatalf("load initial bundle: %v", err)
	}

	tests := map[string]PolicyBundle{
		"missing ID": {
			Version: "2",
		},
		"malformed rule": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{Action: "claim.create", Project: "payments"}},
		},
		"duplicate rule": {
			ID: "replacement", Version: "2",
			Rules: []Rule{
				{Action: "claim.create", Project: "payments", TemplateRef: "engineer"},
				{Action: "claim.create", Project: "payments", TemplateRef: "engineer"},
			},
		},
	}

	for name, bundle := range tests {
		t.Run(name, func(t *testing.T) {
			if err := loader.Load(bundle); err == nil {
				t.Fatal("Load() accepted an invalid bundle")
			}
			current, ok := loader.Current()
			if !ok || current.ID != "reference" || current.Version != "2026-08-24" {
				t.Fatalf("failed load replaced current bundle: %#v, %v", current, ok)
			}
		})
	}
}

func validBundle() PolicyBundle {
	return PolicyBundle{
		ID:      "reference",
		Version: "2026-08-24",
		Rules: []Rule{{
			Action:      "claim.create",
			Project:     "payments",
			TemplateRef: "engineer",
		}},
	}
}
