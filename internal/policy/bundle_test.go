// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"sync"
	"testing"
)

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
	if got.ID != "reference-default-deny" || got.Version != "1" {
		t.Fatalf("Current() identity = %q@%q", got.ID, got.Version)
	}
	if !got.Allows("claim.create", "payments", "engineer") {
		t.Fatal("Allows() denied the exact configured rule")
	}

	bundle.Rules[0].Action = "changed-input"
	fromInput, _ := loader.Current()
	if fromInput.Rules[0].Action != "claim.create" {
		t.Fatal("Load() retained mutable input state")
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
		"missing version": {
			ID: "replacement",
		},
		"missing action": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{Project: "payments", TemplateRef: "engineer"}},
		},
		"missing project": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{Action: "claim.create", TemplateRef: "engineer"}},
		},
		"missing template": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{Action: "claim.create", Project: "payments"}},
		},
		"malformed rule": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{}},
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
			if !ok || current.ID != "reference-default-deny" || current.Version != "1" {
				t.Fatalf("failed load replaced current bundle: %#v, %v", current, ok)
			}
		})
	}
}

func TestLoaderRejectsChangedContentForSameIdentity(t *testing.T) {
	loader := &Loader{}
	original := validBundle()
	if err := loader.Load(original); err != nil {
		t.Fatalf("load initial bundle: %v", err)
	}

	if err := loader.Load(clone(original)); err != nil {
		t.Fatalf("identical reload error = %v", err)
	}

	replacement := clone(original)
	replacement.Rules[0].Action = "claim.delete"
	if err := loader.Load(replacement); err == nil {
		t.Fatal("Load() accepted changed content under the same ID and version")
	}

	current, ok := loader.Current()
	if !ok {
		t.Fatal("changed reload removed the current bundle")
	}
	if !current.Allows("claim.create", "payments", "engineer") {
		t.Fatal("changed reload replaced the original rule")
	}
	if current.Allows("claim.delete", "payments", "engineer") {
		t.Fatal("changed reload activated the replacement rule")
	}
}

func TestLoaderSupportsConcurrentReadersAndWriters(t *testing.T) {
	loader := &Loader{}
	if err := loader.Load(validBundle()); err != nil {
		t.Fatalf("load initial bundle: %v", err)
	}

	const workers = 20
	var group sync.WaitGroup
	group.Add(workers * 2)
	for index := 0; index < workers; index++ {
		index := index
		go func() {
			defer group.Done()
			bundle := validBundle()
			bundle.Version = fmt.Sprintf("version-%d", index)
			if err := loader.Load(bundle); err != nil {
				t.Errorf("concurrent Load() error = %v", err)
			}
		}()
		go func() {
			defer group.Done()
			if bundle, ok := loader.Current(); !ok || bundle.ID != "reference-default-deny" {
				t.Errorf("concurrent Current() = %#v, %v", bundle, ok)
			}
		}()
	}
	group.Wait()
}

func validBundle() PolicyBundle {
	return PolicyBundle{
		ID:      "reference-default-deny",
		Version: "1",
		Rules: []Rule{{
			Action:      "claim.create",
			Project:     "payments",
			TemplateRef: "engineer",
		}},
	}
}
