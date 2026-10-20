// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"reflect"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

type fixedPolicyPrincipal struct{ value v1alpha1.Principal }

func (p fixedPolicyPrincipal) Principal() v1alpha1.Principal { return p.value }

func TestSameWorkSpecGetsUserSpecificIssuedAuthority(t *testing.T) {
	request := canonicalRequest(t)
	loader := &policy.Loader{}
	bundle := policy.PolicyBundle{ID: "user-caps", Version: "1", Rules: []policy.Rule{
		{Subject: "user:alice", Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer", Constraints: &policy.AuthorityConstraints{Tools: &policy.StringSetConstraint{Values: []string{"git.read"}}}},
		{Subject: "user:bob", Team: "team-a", Action: "claim.create", Project: "payments", TemplateRef: "engineer", Constraints: &policy.AuthorityConstraints{Tools: &policy.StringSetConstraint{Values: []string{"git.read", "git.write"}}}},
	}}
	if err := loader.Load(bundle); err != nil {
		t.Fatal(err)
	}
	var issued []*v1alpha1.IssuedState
	for _, subject := range []string{"user:alice", "user:bob"} {
		prepared, err := PrepareAssignment(request, fixedPolicyPrincipal{v1alpha1.Principal{Subject: subject, Team: "team-a", AuthenticationContext: "test:verified"}}, loader, templateSourceFunc(referenceAgentTemplate))
		if err != nil {
			t.Fatalf("%s: %v", subject, err)
		}
		if prepared.Issued == nil || prepared.Issued.PolicyRef.Version != "1" {
			t.Fatalf("%s: missing issued snapshot: %+v", subject, prepared.Issued)
		}
		issued = append(issued, prepared.Issued)
	}
	if reflect.DeepEqual(issued[0].EffectiveAuthority.Tools, issued[1].EffectiveAuthority.Tools) {
		t.Fatalf("user-specific constraints did not narrow differently: %v / %v", issued[0].EffectiveAuthority.Tools, issued[1].EffectiveAuthority.Tools)
	}
	bundle.Version = "2"
	bundle.Rules[0].Constraints.Tools.Values = []string{"git.write"}
	if err := loader.Load(bundle); err != nil {
		t.Fatal(err)
	}
	if issued[0].PolicyRef.Version != "1" || !reflect.DeepEqual(issued[0].EffectiveAuthority.Tools, []string{"git.read"}) {
		t.Fatalf("policy update changed issued authority: %+v", issued[0])
	}
}
