// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package authority

import (
	"os"
	"reflect"
	"testing"
	"time"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
)

func TestResolveCanonicalTeamAAuthority(t *testing.T) {
	request, template, expected := fixtures(t)
	got, err := Resolve(request, template, allowDecision())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	expected.ID = ""
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("authority mismatch:\n got: %#v\nwant: %#v", got, expected)
	}
}

func TestResolveRequiresExactAllow(t *testing.T) {
	request, template, _ := fixtures(t)
	for _, result := range []v1alpha1.DecisionResult{v1alpha1.DecisionResultDeny, v1alpha1.DecisionResultApprovalRequired, ""} {
		t.Run(string(result), func(t *testing.T) {
			got, err := Resolve(request, template, v1alpha1.Decision{Result: result})
			assertFailure(t, got, err, "admission.result")
		})
	}
}

func TestResolveNarrowsListsInRequestOrder(t *testing.T) {
	request, template, _ := fixtures(t)
	request.Spec.RequestedAccess.Tools = []string{"shell.exec", "git.write", "git.read"}
	request.Spec.RequestedAccess.MemoryScopes = []string{"private", "team-docs"}
	got, err := Resolve(request, template, allowDecision())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Tools, []string{"git.write", "git.read"}) || !reflect.DeepEqual(got.MemoryScopes, []string{"team-docs"}) {
		t.Fatalf("narrowed lists = tools:%v memory:%v", got.Tools, got.MemoryScopes)
	}
}

func TestResolveRejectsCompletelyEmptyRequestedDimensions(t *testing.T) {
	tests := map[string]struct {
		path   string
		mutate func(*v1alpha1.ClaimRequest)
	}{
		"tools":           {"spec.requestedAccess.tools", func(r *v1alpha1.ClaimRequest) { r.Spec.RequestedAccess.Tools = []string{"shell.exec"} }},
		"resource scopes": {"spec.requestedAccess.resourceScopes", func(r *v1alpha1.ClaimRequest) { r.Spec.RequestedAccess.ResourceScopes = []string{"repo:other/project"} }},
		"memory scopes":   {"spec.requestedAccess.memoryScopes", func(r *v1alpha1.ClaimRequest) { r.Spec.RequestedAccess.MemoryScopes = []string{"private"} }},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			request, template, _ := fixtures(t)
			test.mutate(request)
			got, err := Resolve(request, template, allowDecision())
			assertFailure(t, got, err, test.path)
		})
	}
}

func TestResolveResourceScopeContainmentAndWildcardFailures(t *testing.T) {
	t.Run("exact scope", func(t *testing.T) {
		request, template, _ := fixtures(t)
		template.Spec.CapabilityCeiling.ResourceScopes = []string{"repo:acme/payments"}
		got, err := Resolve(request, template, allowDecision())
		if err != nil || !reflect.DeepEqual(got.ResourceScopes, []string{"repo:acme/payments"}) {
			t.Fatalf("got/error = %#v/%v", got, err)
		}
	})
	t.Run("terminal wildcard ceiling emits concrete request", func(t *testing.T) {
		request, template, _ := fixtures(t)
		got, err := Resolve(request, template, allowDecision())
		if err != nil || got.ResourceScopes[0] != "repo:acme/payments" {
			t.Fatalf("got/error = %#v/%v", got, err)
		}
	})
	t.Run("wildcard request", func(t *testing.T) {
		request, template, _ := fixtures(t)
		request.Spec.RequestedAccess.ResourceScopes = []string{"repo:acme/*"}
		got, err := Resolve(request, template, allowDecision())
		assertFailure(t, got, err, "spec.requestedAccess.resourceScopes[0]")
	})
	for _, ceiling := range []string{"*", "repo:*:payments", "repo:acme/**"} {
		t.Run("unsupported ceiling "+ceiling, func(t *testing.T) {
			request, template, _ := fixtures(t)
			template.Spec.CapabilityCeiling.ResourceScopes = []string{ceiling}
			got, err := Resolve(request, template, allowDecision())
			assertFailure(t, got, err, "spec.capabilityCeiling.resourceScopes[0]")
		})
	}
}

func TestResolveRequiresExplicitScalarProfiles(t *testing.T) {
	t.Run("model profile", func(t *testing.T) {
		request, template, _ := fixtures(t)
		request.Spec.RequestedAccess.ModelProfile = "unapproved-model"
		got, err := Resolve(request, template, allowDecision())
		assertFailure(t, got, err, "spec.requestedAccess.modelProfile")
	})
	t.Run("runtime profile", func(t *testing.T) {
		request, template, _ := fixtures(t)
		request.Spec.Runtime.ProfileRef = "privileged"
		got, err := Resolve(request, template, allowDecision())
		assertFailure(t, got, err, "spec.runtime.profileRef")
	})
}

func TestResolveCapsTimeoutAndFailsWithoutUsableCeiling(t *testing.T) {
	request, template, _ := fixtures(t)
	long := v1alpha1.Duration(45 * time.Minute)
	request.Spec.Runtime.Timeout = &long
	got, err := Resolve(request, template, allowDecision())
	if err != nil || time.Duration(got.Runtime.Timeout) != 30*time.Minute {
		t.Fatalf("timeout/error = %s/%v", got.Runtime.Timeout.String(), err)
	}

	template.Spec.CapabilityCeiling.MaxTimeout = nil
	got, err = Resolve(request, template, allowDecision())
	assertFailure(t, got, err, "spec.capabilityCeiling.maxTimeout")
}

func TestResolveDoesNotApplyTemplateDefaults(t *testing.T) {
	request, template, _ := fixtures(t)
	request.Spec.RequestedAccess = v1alpha1.ClaimRequestedAccess{}
	got, err := Resolve(request, template, allowDecision())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tools) != 0 || len(got.ResourceScopes) != 0 || got.ModelProfile != "" || len(got.MemoryScopes) != 0 {
		t.Fatalf("template defaults enlarged omitted request: %+v", got)
	}
}

func TestResolveReturnsIndependentSnapshot(t *testing.T) {
	request, template, _ := fixtures(t)
	got, err := Resolve(request, template, allowDecision())
	if err != nil {
		t.Fatal(err)
	}
	request.Spec.RequestedAccess.Tools[0] = "mutated-request"
	request.Spec.RequestedAccess.ResourceScopes[0] = "mutated-request"
	template.Spec.CapabilityCeiling.Tools[0] = "mutated-template"
	template.Spec.CapabilityCeiling.ResourceScopes[0] = "mutated-template"
	if got.Tools[0] != "git.read" || got.ResourceScopes[0] != "repo:acme/payments" {
		t.Fatalf("source mutation changed snapshot: %+v", got)
	}
}

func TestResolveRejectsMismatchedTemplate(t *testing.T) {
	request, template, _ := fixtures(t)
	template.Metadata.Name = "reviewer"
	got, err := Resolve(request, template, allowDecision())
	assertFailure(t, got, err, "spec.templateRef")
}

func allowDecision() v1alpha1.Decision {
	return v1alpha1.Decision{Result: v1alpha1.DecisionResultAllow}
}

func fixtures(t *testing.T) (*v1alpha1.ClaimRequest, *v1alpha1.AgentTemplate, *v1alpha1.EffectiveAuthority) {
	t.Helper()
	requestData := read(t, "../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml")
	request, requestErr := v1alpha1.ParseClaimRequestYAML(requestData)
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	templateData := read(t, "../../harness/fixtures/contract/v0/inputs/agent-template/valid-engineer.yaml")
	template, templateErr := v1alpha1.ParseAgentTemplateYAML(templateData)
	if templateErr != nil {
		t.Fatal(templateErr)
	}
	stateData := read(t, "../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json")
	state, stateErr := v1alpha1.ParseSystemIssuedState(stateData)
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	return request, template, state.EffectiveAuthority
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertFailure(t *testing.T, got *v1alpha1.EffectiveAuthority, err *v1alpha1.ValidationError, path string) {
	t.Helper()
	if got != nil || err == nil || err.FieldPath != path {
		t.Fatalf("authority/error = %#v/%#v, want nil and path %q", got, err, path)
	}
}
