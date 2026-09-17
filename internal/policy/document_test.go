// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package policy

import "testing"

func TestParseEmptyDefaultDenyRulesAsCollection(t *testing.T) {
	bundle, err := ParseDocumentYAML([]byte("apiVersion: agenova.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: empty\n  version: '1'\nspec:\n  rules: []\n"))
	if err != nil || bundle.Rules == nil || len(bundle.Rules) != 0 {
		t.Fatalf("empty default-deny bundle = %#v, %v", bundle, err)
	}
}

func TestParseDocumentYAMLStrictAndDefaultDeny(t *testing.T) {
	input := []byte(`apiVersion: agenova.io/v1alpha1
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
	bundle, err := ParseDocumentYAML(input)
	if err != nil || bundle.ID != "demo" || !bundle.Allows(Match{Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer"}) || bundle.Allows(Match{Team: "team-b", Action: "claim.create", Project: "payments", TemplateRef: "engineer"}) {
		t.Fatalf("parsed bundle = %#v, %v", bundle, err)
	}
	for _, invalid := range [][]byte{
		append(append([]byte{}, input...), []byte("\n---\nkind: PolicyBundle\n")...),
		[]byte("apiVersion: agenova.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: demo\n  version: '1'\n  principal: admin\nspec:\n  rules: []\n"),
		[]byte("apiVersion: agenova.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: demo\n  version: '1'\nspec:\n  rules:\n    - team: team-a\n      action: claim.create\n      project: payments\n      templateRef: engineer\n      secret: x\n"),
	} {
		if _, err := ParseDocumentYAML(invalid); err == nil {
			t.Fatalf("invalid PolicyBundle accepted: %q", invalid)
		}
	}
}
