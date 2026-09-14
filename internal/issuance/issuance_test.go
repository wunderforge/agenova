// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package issuance

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authority"
	"github.com/wunderforge/agenova/internal/authorization"
	"github.com/wunderforge/agenova/internal/policy"
)

func TestIssueCanonicalTeamAPendingClaim(t *testing.T) {
	request, template, fixture, bundle := teamAFixtures(t)
	admission, decision := admit(t, request, fixture.Principal, bundle)
	resolved := resolve(t, request, template, admission)
	state, err := Issue(request, fixture.Principal, decision, resolved, admission)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if err := v1alpha1.ValidateIssuedState(state); err != nil {
		t.Fatalf("issued state invalid: %v", err)
	}
	if state.Principal != fixture.Principal || state.Action != fixture.Action || state.PolicyRef != decision.PolicyRef || state.Decision != decision {
		t.Fatalf("admission context was not preserved: %+v", state)
	}
	if state.Claim == nil || state.Claim.Phase != v1alpha1.ClaimPhasePending || state.Claim.BackendIdentity != nil {
		t.Fatalf("expected Pending claim with no backend identity: %+v", state.Claim)
	}
	if !strings.HasPrefix(state.Claim.ID, "claim:fix-payment-timeout:issuance:") ||
		!strings.HasPrefix(state.EffectiveAuthority.ID, "authority:fix-payment-timeout:issuance:") ||
		state.Claim.ID == fixture.Claim.ID || state.EffectiveAuthority.ID == fixture.EffectiveAuthority.ID {
		t.Fatalf("identities were not system-derived: claim=%q authority=%q", state.Claim.ID, state.EffectiveAuthority.ID)
	}
	if state.Claim.RequestRef != request.Metadata.Name || state.Claim.TemplateRef != request.Spec.TemplateRef ||
		state.Claim.AuthorityRef != state.EffectiveAuthority.ID || state.Evidence.ClaimID != state.Claim.ID ||
		!reflect.DeepEqual(state.Evidence.DecisionIDs, []string{decision.ID}) {
		t.Fatalf("issued identities do not correlate: %+v", state)
	}
	if state.EffectiveAuthority.Runtime != resolved.Runtime ||
		!reflect.DeepEqual(state.EffectiveAuthority.Tools, resolved.Tools) ||
		!reflect.DeepEqual(state.EffectiveAuthority.ResourceScopes, resolved.ResourceScopes) ||
		!reflect.DeepEqual(state.EffectiveAuthority.MemoryScopes, resolved.MemoryScopes) ||
		state.EffectiveAuthority.ModelProfile != resolved.ModelProfile || resolved.ID != "" {
		t.Fatalf("resolved authority changed during issuance: got=%+v source=%+v", state.EffectiveAuthority, resolved)
	}
	if state.Evidence.RuntimeEvents == nil || state.Evidence.ToolInvocations == nil || state.Evidence.ModelInvocations == nil ||
		len(state.Evidence.RuntimeEvents) != 0 || len(state.Evidence.ToolInvocations) != 0 || len(state.Evidence.ModelInvocations) != 0 {
		t.Fatalf("new claim has prepopulated or nil facts: %+v", state.Evidence)
	}
	encoded := encode(t, state)
	parsed, parseErr := v1alpha1.ParseSystemIssuedState(encoded)
	if parseErr != nil || !reflect.DeepEqual(parsed, state) {
		t.Fatalf("system-issued round trip = %+v/%v", parsed, parseErr)
	}
	if caller, callerErr := v1alpha1.ParseCallerIssuedState(encoded); caller != nil || callerErr == nil || callerErr.Category != v1alpha1.ValidationCategorySystemManagedField {
		t.Fatalf("caller was allowed to self-issue state: %+v/%v", caller, callerErr)
	}
}

func TestIssueCanonicalTeamBDenialProducesNoClaim(t *testing.T) {
	request, template, _, bundle := teamAFixtures(t)
	denied := issuedFixture(t, "valid-team-b-denial.json")
	loader := loadPolicy(t, bundle)
	gate := authorization.Gate{Evaluator: authorization.Authorizer{Policies: loader}}
	called := false
	decision, err := gate.Admit(authorizationRequest(request, denied.Principal), func(admission authorization.Admission) error {
		called = true
		return nil
	})
	if err != nil || called || decision.Result != v1alpha1.DecisionResultDeny {
		t.Fatalf("Team B admission = %+v/%v, continuation called=%v", decision, err, called)
	}
	if denied.Claim != nil || denied.EffectiveAuthority != nil || denied.Evidence.ClaimID != "" {
		t.Fatalf("denial fixture fabricated a claim: %+v", denied)
	}
	// Even a valid Team A resolution cannot be paired with Team B's public
	// denial decision or an absent admission token.
	teamA := issuedFixture(t, "valid-team-a-engineer.json").Principal
	teamAAdmission, _ := admit(t, request, teamA, bundle)
	resolved := resolve(t, request, template, teamAAdmission)
	state, issueErr := Issue(request, denied.Principal, decision, resolved, authorization.Admission{})
	assertRejected(t, state, issueErr, "admission")
}

func TestIssueRequiresExactGateContext(t *testing.T) {
	request, template, fixture, bundle := teamAFixtures(t)
	admission, decision := admit(t, request, fixture.Principal, bundle)
	resolved := resolve(t, request, template, admission)
	for name, mutate := range map[string]func(*v1alpha1.ClaimRequest, *v1alpha1.Principal, *v1alpha1.Decision){
		"request reference": func(r *v1alpha1.ClaimRequest, _ *v1alpha1.Principal, _ *v1alpha1.Decision) {
			r.Metadata.Name = "other-request"
		},
		"project": func(r *v1alpha1.ClaimRequest, _ *v1alpha1.Principal, _ *v1alpha1.Decision) {
			r.Spec.ProjectRef = "other-project"
		},
		"template": func(r *v1alpha1.ClaimRequest, _ *v1alpha1.Principal, _ *v1alpha1.Decision) {
			r.Spec.TemplateRef = "reviewer"
		},
		"subject": func(_ *v1alpha1.ClaimRequest, p *v1alpha1.Principal, _ *v1alpha1.Decision) { p.Subject = "user:other" },
		"team":    func(_ *v1alpha1.ClaimRequest, p *v1alpha1.Principal, _ *v1alpha1.Decision) { p.Team = "team-b" },
		"auth context": func(_ *v1alpha1.ClaimRequest, p *v1alpha1.Principal, _ *v1alpha1.Decision) {
			p.AuthenticationContext = "upstream:other"
		},
		"decision ID":     func(_ *v1alpha1.ClaimRequest, _ *v1alpha1.Principal, d *v1alpha1.Decision) { d.ID = "decision:forged" },
		"decision policy": func(_ *v1alpha1.ClaimRequest, _ *v1alpha1.Principal, d *v1alpha1.Decision) { d.PolicyRef.Version = "2" },
		"decision result": func(_ *v1alpha1.ClaimRequest, _ *v1alpha1.Principal, d *v1alpha1.Decision) {
			d.Result = v1alpha1.DecisionResultApprovalRequired
		},
		"decision reason": func(_ *v1alpha1.ClaimRequest, _ *v1alpha1.Principal, d *v1alpha1.Decision) { d.Reason = "forged" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := cloneRequest(t, request)
			principal, emitted := fixture.Principal, decision
			mutate(changed, &principal, &emitted)
			state, err := Issue(changed, principal, emitted, resolved, admission)
			assertRejected(t, state, err, "admission")
		})
	}
	state, err := Issue(request, fixture.Principal, decision, resolved, authorization.Admission{})
	assertRejected(t, state, err, "admission")
	state, err = Issue(nil, fixture.Principal, decision, resolved, admission)
	assertRejected(t, state, err, "$")
}

func TestIssueIsDeterministicAndIncludesFullTask(t *testing.T) {
	request, template, fixture, bundle := teamAFixtures(t)
	admission, decision := admit(t, request, fixture.Principal, bundle)
	resolved := resolve(t, request, template, admission)
	first, err := Issue(request, fixture.Principal, decision, resolved, admission)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Issue(request, fixture.Principal, decision, resolved, admission)
	if err != nil || !bytes.Equal(encode(t, first), encode(t, second)) {
		t.Fatalf("identical admitted inputs produced different snapshots: %v", err)
	}
	changedTask := cloneRequest(t, request)
	changedTask.Spec.Task.Input["objective"] = "Fix a different timeout"
	changed, err := Issue(changedTask, fixture.Principal, decision, resolved, admission)
	if err != nil || changed.Claim.ID == first.Claim.ID {
		t.Fatalf("same-reference/different-task did not change identity: %+v/%v", changed, err)
	}
	changedTask.Spec.Task.Input["objective"] = request.Spec.Task.Input["objective"]
	changedTask.Spec.Task.Input["baseBranch"] = "release"
	changed, err = Issue(changedTask, fixture.Principal, decision, resolved, admission)
	if err != nil || changed.Claim.ID == first.Claim.ID {
		t.Fatalf("same-reference/different-base-branch did not change identity: %+v/%v", changed, err)
	}
	otherAuthority := *resolved
	otherAuthority.Tools = cloneStrings(resolved.Tools[1:])
	changed, err = Issue(request, fixture.Principal, decision, &otherAuthority, admission)
	if err != nil || changed.Claim.ID == first.Claim.ID {
		t.Fatalf("changed authority did not change identity: %+v/%v", changed, err)
	}
}

func TestIssueRejectsInvalidAuthorityWithoutPartialState(t *testing.T) {
	request, template, fixture, bundle := teamAFixtures(t)
	admission, decision := admit(t, request, fixture.Principal, bundle)
	resolved := resolve(t, request, template, admission)
	for name, test := range map[string]struct {
		path   string
		mutate func(*v1alpha1.EffectiveAuthority)
	}{
		"preissued ID":    {"effectiveAuthority.id", func(a *v1alpha1.EffectiveAuthority) { a.ID = "caller-id" }},
		"runtime profile": {"effectiveAuthority.runtime.profileRef", func(a *v1alpha1.EffectiveAuthority) { a.Runtime.ProfileRef = "" }},
		"runtime timeout": {"effectiveAuthority.runtime.timeout", func(a *v1alpha1.EffectiveAuthority) { a.Runtime.Timeout = 0 }},
		"blank tool":      {"effectiveAuthority.tools[0]", func(a *v1alpha1.EffectiveAuthority) { a.Tools = []string{" "} }},
	} {
		t.Run(name, func(t *testing.T) {
			changed := *resolved
			test.mutate(&changed)
			state, err := Issue(request, fixture.Principal, decision, &changed, admission)
			assertRejected(t, state, err, test.path)
		})
	}
	state, err := Issue(request, fixture.Principal, decision, nil, admission)
	assertRejected(t, state, err, "effectiveAuthority")
}

func TestIssueSnapshotDoesNotAliasInputs(t *testing.T) {
	request, template, fixture, bundle := teamAFixtures(t)
	admission, decision := admit(t, request, fixture.Principal, bundle)
	resolved := resolve(t, request, template, admission)
	state, err := Issue(request, fixture.Principal, decision, resolved, admission)
	if err != nil {
		t.Fatal(err)
	}
	before := encode(t, state)
	request.Metadata.Name = "mutated-request"
	request.Spec.Task.Input["repository"] = "other/repo"
	request.Spec.RequestedAccess.Tools[0] = "mutated-request-tool"
	template.Spec.CapabilityCeiling.Tools[0] = "mutated-template-tool"
	bundle.Rules[0].Team = "mutated-policy"
	resolved.Tools[0] = "mutated-resolved-tool"
	resolved.ResourceScopes[0] = "mutated-resolved-resource"
	resolved.MemoryScopes[0] = "mutated-resolved-memory"
	if !bytes.Equal(before, encode(t, state)) {
		t.Fatal("source mutation changed issued snapshot")
	}
}

func teamAFixtures(t *testing.T) (*v1alpha1.ClaimRequest, *v1alpha1.AgentTemplate, *v1alpha1.IssuedState, policy.PolicyBundle) {
	t.Helper()
	requestData := readFixture(t, "../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml")
	request, requestErr := v1alpha1.ParseClaimRequestYAML(requestData)
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	templateData := readFixture(t, "../../harness/fixtures/contract/v0/inputs/agent-template/valid-engineer.yaml")
	template, templateErr := v1alpha1.ParseAgentTemplateYAML(templateData)
	if templateErr != nil {
		t.Fatal(templateErr)
	}
	fixture := issuedFixture(t, "valid-team-a-engineer.json")
	bundle := policy.PolicyBundle{
		ID: "reference-default-deny", Version: "1",
		Rules: []policy.Rule{{Team: fixture.Principal.Team, Action: "claim.create", Project: request.Spec.ProjectRef, TemplateRef: request.Spec.TemplateRef}},
	}
	return request, template, fixture, bundle
}

func issuedFixture(t *testing.T, name string) *v1alpha1.IssuedState {
	t.Helper()
	state, err := v1alpha1.ParseSystemIssuedState(readFixture(t, "../../harness/fixtures/contract/v0/inputs/issued-state/"+name))
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func loadPolicy(t *testing.T, bundle policy.PolicyBundle) *policy.Loader {
	t.Helper()
	loader := &policy.Loader{}
	if err := loader.Load(bundle); err != nil {
		t.Fatal(err)
	}
	return loader
}

func authorizationRequest(request *v1alpha1.ClaimRequest, principal v1alpha1.Principal) authorization.Request {
	return authorization.Request{
		RequestRef: request.Metadata.Name,
		Principal:  principal,
		Action: v1alpha1.Action{
			Name: "claim.create", Project: request.Spec.ProjectRef, TemplateRef: request.Spec.TemplateRef,
		},
	}
}

func admit(t *testing.T, request *v1alpha1.ClaimRequest, principal v1alpha1.Principal, bundle policy.PolicyBundle) (authorization.Admission, v1alpha1.Decision) {
	t.Helper()
	var admission authorization.Admission
	decision, err := (authorization.Gate{Evaluator: authorization.Authorizer{Policies: loadPolicy(t, bundle)}}).Admit(
		authorizationRequest(request, principal), func(value authorization.Admission) error { admission = value; return nil },
	)
	if err != nil || decision.Result != v1alpha1.DecisionResultAllow {
		t.Fatalf("admission = %+v/%v", decision, err)
	}
	return admission, decision
}

func resolve(t *testing.T, request *v1alpha1.ClaimRequest, template *v1alpha1.AgentTemplate, admission authorization.Admission) *v1alpha1.EffectiveAuthority {
	t.Helper()
	resolved, err := authority.Resolve(request, template, admission)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func cloneRequest(t *testing.T, request *v1alpha1.ClaimRequest) *v1alpha1.ClaimRequest {
	t.Helper()
	var clone v1alpha1.ClaimRequest
	if err := json.Unmarshal(encode(t, request), &clone); err != nil {
		t.Fatal(err)
	}
	return &clone
}

func encode(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertRejected(t *testing.T, state *v1alpha1.IssuedState, err *v1alpha1.ValidationError, path string) {
	t.Helper()
	if state != nil || err == nil || err.FieldPath != path {
		t.Fatalf("issued state/error = %+v/%+v, want nil and %q", state, err, path)
	}
}

func TestIssueRejectsNonPositiveTimeout(t *testing.T) {
	request, template, fixture, bundle := teamAFixtures(t)
	admission, decision := admit(t, request, fixture.Principal, bundle)
	resolved := resolve(t, request, template, admission)
	resolved.Runtime.Timeout = v1alpha1.Duration(-time.Second)
	state, err := Issue(request, fixture.Principal, decision, resolved, admission)
	assertRejected(t, state, err, "effectiveAuthority.runtime.timeout")
}
