// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"errors"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func TestRegistrationCreateIdempotentConflictAndSnapshot(t *testing.T) {
	store := NewMemoryStore()
	svc := Service{Store: store}
	policyYAML := []byte(`apiVersion: agenova.io/v1alpha1
kind: PolicyBundle
metadata:
  name: demo
  version: "1"
spec:
  rules:
    - team: team-a
      action: claim.create
      project: payments
      templateRef: engineer
`)
	first, err := svc.ApplyPolicy(policyYAML)
	if err != nil || !first.Changed || !first.Active {
		t.Fatalf("first policy apply = %#v, %v", first, err)
	}
	second, err := svc.ApplyPolicy(policyYAML)
	if err != nil || second.Changed {
		t.Fatalf("repeat policy apply = %#v, %v", second, err)
	}
	bundle, err := store.ActivePolicy()
	if err != nil || len(bundle.Rules) != 1 {
		t.Fatalf("active policy = %#v, %v", bundle, err)
	}
	bundle.Rules[0].Team = "team-b"
	again, _ := store.ActivePolicy()
	if again.Rules[0].Team != "team-a" {
		t.Fatal("caller mutated active policy snapshot")
	}
	conflicting := []byte(`apiVersion: agenova.io/v1alpha1
kind: PolicyBundle
metadata: {name: demo, version: "1"}
spec: {rules: []}
`)
	if _, err := svc.ApplyPolicy(conflicting); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting policy = %v", err)
	}

	template := []byte(`apiVersion: agenova.io/v1alpha1
kind: AgentTemplate
metadata: {name: engineer}
spec:
  artifact: {image: agenova-testworker:kind}
  entrypoint: {command: [/agenova-workerctl, serve]}
  capabilityCeiling:
    modelProfiles: [coding-standard]
    runtimeProfiles: [standard-isolated]
`)
	created, err := svc.ApplyTemplate(template)
	if err != nil || !created.Changed {
		t.Fatalf("first template = %#v, %v", created, err)
	}
	repeated, err := svc.ApplyTemplate(template)
	if err != nil || repeated.Changed {
		t.Fatalf("repeat template = %#v, %v", repeated, err)
	}
	registered, err := store.Template("engineer")
	if err != nil || registered.Kind != v0.AgentTemplateKind {
		t.Fatalf("registered template = %#v, %v", registered, err)
	}
	registered.Spec.Artifact.Image = "tampered"
	againTemplate, _ := store.Template("engineer")
	if againTemplate.Spec.Artifact.Image != "agenova-testworker:kind" {
		t.Fatal("caller mutated registered template snapshot")
	}
	if _, err := svc.ApplyTemplate([]byte(`apiVersion: agenova.io/v1alpha1
kind: AgentTemplate
metadata: {name: engineer}
spec:
  artifact: {image: different:tag}
  entrypoint: {command: [/agenova-workerctl, serve]}
  capabilityCeiling:
    modelProfiles: [coding-standard]
    runtimeProfiles: [standard-isolated]
`)); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting template = %v", err)
	}
}
