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
	if !got.Allows(validMatch()) {
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
		team, action, project, template string
	}{
		"missing team":     {action: "claim.create", project: "payments", template: "engineer"},
		"unknown team":     {team: "team-b", action: "claim.create", project: "payments", template: "engineer"},
		"missing action":   {team: "team-a", project: "payments", template: "engineer"},
		"unknown action":   {team: "team-a", action: "claim.delete", project: "payments", template: "engineer"},
		"unknown project":  {team: "team-a", action: "claim.create", project: "ledger", template: "engineer"},
		"unknown template": {team: "team-a", action: "claim.create", project: "payments", template: "reviewer"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if bundle.Allows(Match{Team: test.team, Action: test.action, Project: test.project, TemplateRef: test.template}) {
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
		"missing team": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{Action: "claim.create", Project: "payments", TemplateRef: "engineer"}},
		},
		"missing action": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{Team: "team-a", Project: "payments", TemplateRef: "engineer"}},
		},
		"missing project": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{Team: "team-a", Action: "claim.create", TemplateRef: "engineer"}},
		},
		"missing template": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{Team: "team-a", Action: "claim.create", Project: "payments"}},
		},
		"malformed rule": {
			ID: "replacement", Version: "2",
			Rules: []Rule{{}},
		},
		"duplicate rule": {
			ID: "replacement", Version: "2",
			Rules: []Rule{
				{Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer"},
				{Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer"},
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
	replacement.Rules[0].Team = "team-b"
	if err := loader.Load(replacement); err == nil {
		t.Fatal("Load() accepted changed content under the same ID and version")
	}

	current, ok := loader.Current()
	if !ok {
		t.Fatal("changed reload removed the current bundle")
	}
	if !current.Allows(validMatch()) {
		t.Fatal("changed reload replaced the original rule")
	}
	teamB := validMatch()
	teamB.Team = "team-b"
	if current.Allows(teamB) {
		t.Fatal("changed reload activated the replacement rule")
	}
}

func TestLoaderRejectsChangedContentForPreviouslySeenIdentity(t *testing.T) {
	loader := &Loader{}
	versionOne := validBundle()
	if err := loader.Load(versionOne); err != nil {
		t.Fatalf("load version one: %v", err)
	}

	versionTwo := validBundle()
	versionTwo.Version = "2"
	versionTwo.Rules[0].Team = "team-b"
	if err := loader.Load(versionTwo); err != nil {
		t.Fatalf("load version two: %v", err)
	}
	if err := loader.Load(clone(versionOne)); err != nil {
		t.Fatalf("reload identical historical version: %v", err)
	}
	if err := loader.Load(clone(versionTwo)); err != nil {
		t.Fatalf("restore identical active version: %v", err)
	}

	changedVersionOne := clone(versionOne)
	changedVersionOne.Rules[0].Action = "claim.delete"
	if err := loader.Load(changedVersionOne); err == nil {
		t.Fatal("Load() accepted changed content for a previously seen ID and version")
	}

	current, ok := loader.Current()
	if !ok || current.Version != "2" {
		t.Fatalf("historical mutation replaced current bundle: %#v, %v", current, ok)
	}
	teamB := validMatch()
	teamB.Team = "team-b"
	if !current.Allows(teamB) {
		t.Fatal("historical mutation replaced the active version")
	}
}

func TestLoaderAllowsSameRuleForDifferentTeams(t *testing.T) {
	loader := &Loader{}
	bundle := validBundle()
	teamBRule := bundle.Rules[0]
	teamBRule.Team = "team-b"
	bundle.Rules = append(bundle.Rules, teamBRule)

	if err := loader.Load(bundle); err != nil {
		t.Fatalf("Load() rejected distinct team-scoped rules: %v", err)
	}
	current, ok := loader.Current()
	if !ok {
		t.Fatal("Current() did not return the team-scoped bundle")
	}
	teamB := validMatch()
	teamB.Team = "team-b"
	if !current.Allows(validMatch()) || !current.Allows(teamB) {
		t.Fatal("team-scoped rules did not allow both exact teams")
	}
}

func TestLoaderProtectsSeenIdentityFromCallerMutation(t *testing.T) {
	loader := &Loader{}
	original := validBundle()
	if err := loader.Load(original); err != nil {
		t.Fatalf("load original bundle: %v", err)
	}

	mutated := clone(original)
	original.Rules[0].Team = "team-b"
	original.Rules[0].Action = "claim.delete"
	if err := loader.Load(mutated); err != nil {
		t.Fatalf("caller mutation changed retained identity content: %v", err)
	}
	if err := loader.Load(original); err == nil {
		t.Fatal("Load() accepted caller-mutated content under the same identity")
	}

	current, ok := loader.Current()
	if !ok || !current.Allows(validMatch()) {
		t.Fatalf("caller mutation changed active bundle: %#v, %v", current, ok)
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
			Team:        "team-a",
			Action:      "claim.create",
			Project:     "payments",
			TemplateRef: "engineer",
		}},
	}
}

func validMatch() Match {
	return Match{
		Team:        "team-a",
		Action:      "claim.create",
		Project:     "payments",
		TemplateRef: "engineer",
	}
}
