// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package authority

import (
	"github.com/wunderforge/agenova/internal/policy"
	"testing"
)

func TestModelCallBudgetDefaultExplicitAndCeiling(t *testing.T) {
	request, template, _ := fixtures(t)
	legacy, err := Resolve(request, template, admit(t, request))
	if err != nil || legacy.Runtime.MaxModelCalls != 0 {
		t.Fatal("legacy representation changed", err)
	}
	ceiling := 20
	template.Spec.CapabilityCeiling.MaxModelCalls = &ceiling
	for _, n := range []int{1, 2, 8, 20, 21, 0, -1} {
		value := n
		request.Spec.Runtime.MaxModelCalls = &value
		got, e := Resolve(request, template, admit(t, request))
		if n < 1 || n > 20 {
			if e == nil {
				t.Fatal("invalid/over-ceiling budget accepted", n)
			}
		} else if e != nil || got.Runtime.MaxModelCalls != n {
			t.Fatal("budget lost", n, e)
		}
	}
	request.Spec.Runtime.MaxModelCalls = nil
	two := 2
	template.Spec.CapabilityCeiling.MaxModelCalls = &two
	got, e := Resolve(request, template, admit(t, request))
	if e != nil || got.Runtime.MaxModelCalls != 2 {
		t.Fatal("omitted budget exceeded ceiling", e)
	}
}

func TestPolicyModelBudgetIsNarrowerThanTemplate(t *testing.T) {
	request, template, _ := fixtures(t)
	twenty, two, eight := 20, 2, 8
	template.Spec.CapabilityCeiling.MaxModelCalls = &twenty
	caps := &policy.AuthorityConstraints{MaxModelCalls: &two}
	admission := admitWithConstraints(t, request, caps)
	caps.MaxModelCalls = &twenty // Caller mutation must not expand Gate's snapshot.
	got, err := Resolve(request, template, admission)
	if err != nil || got.Runtime.MaxModelCalls != 2 {
		t.Fatal("lost policy cap", err)
	}
	request.Spec.Runtime.MaxModelCalls = &eight
	if _, err := Resolve(request, template, admission); err == nil {
		t.Fatal("explicit request bypassed policy cap")
	}
}
