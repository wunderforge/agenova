// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package platform

import (
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"testing"
)

func TestAdvisorConfigurationChangesRevisionAndLock(t *testing.T) {
	input := referencePlatform()
	old, _, e := Resolve(input, referenceLookup())
	if e != nil {
		t.Fatal(e)
	}
	input.Spec.Services.AccessAdvisor = &v0.AccessAdvisorConfig{Enabled: true, Engine: "rules", DefaultMode: "automatic"}
	resolved, lock, e := Resolve(input, referenceLookup())
	if e != nil {
		t.Fatal(e)
	}
	if resolved.Revision == old.Revision || resolved.AccessAdvisor == nil || VerifyResolvedLock(resolved, lock) != nil {
		t.Fatal("Advisor config omitted from effective state")
	}
	resolved.AccessAdvisor.MaxCalls++
	if VerifyResolvedLock(resolved, lock) == nil {
		t.Fatal("config drift bypassed revision")
	}
	input.Spec.Services.AccessAdvisor = &v0.AccessAdvisorConfig{Enabled: true, Engine: "model", ModelProfile: "uninstalled"}
	if _, _, e := Resolve(input, referenceLookup()); e == nil {
		t.Fatal("uninstalled planner profile accepted")
	}
}
