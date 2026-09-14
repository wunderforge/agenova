// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"os"
	"regexp"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authority"
	"github.com/wunderforge/agenova/internal/authorization"
	"github.com/wunderforge/agenova/internal/issuance"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/runtime"
)

// This crosses the issuer/adapter boundary with a claim ID actually generated
// by issuance. The kube seam is fake; this does not claim a kind-level pass.
func TestIssuedClaimIDAllocatesWithKubernetesSafeResourceName(t *testing.T) {
	requestData, err := os.ReadFile("../../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml")
	if err != nil {
		t.Fatal(err)
	}
	request, parseErr := v1alpha1.ParseClaimRequestYAML(requestData)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	templateData, err := os.ReadFile("../../../harness/fixtures/contract/v0/inputs/agent-template/valid-engineer.yaml")
	if err != nil {
		t.Fatal(err)
	}
	template, parseErr := v1alpha1.ParseAgentTemplateYAML(templateData)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	principal := v1alpha1.Principal{Subject: "user:team-a-engineer", Team: "team-a", AuthenticationContext: "upstream:test"}
	loader := &policy.Loader{}
	if err := loader.Load(policy.PolicyBundle{ID: "reference-default-deny", Version: "1", Rules: []policy.Rule{{
		Team: principal.Team, Action: "claim.create", Project: request.Spec.ProjectRef, TemplateRef: request.Spec.TemplateRef,
	}}}); err != nil {
		t.Fatal(err)
	}
	var admission authorization.Admission
	decision, err := (authorization.Gate{Evaluator: authorization.Authorizer{Policies: loader}}).Admit(authorization.Request{
		RequestRef: request.Metadata.Name, Principal: principal,
		Action: v1alpha1.Action{Name: "claim.create", Project: request.Spec.ProjectRef, TemplateRef: request.Spec.TemplateRef},
	}, func(value authorization.Admission) error { admission = value; return nil })
	if err != nil || decision.Result != v1alpha1.DecisionResultAllow {
		t.Fatalf("admission = %+v/%v", decision, err)
	}
	resolution, resolveErr := authority.ResolveForIssuance(request, template, admission)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	state, issueErr := issuance.Issue(request, principal, decision, resolution, admission)
	if issueErr != nil {
		t.Fatal(issueErr)
	}
	k := newFakeKube()
	k.bindOnApply = "issued-worker"
	a := newTestAdapter(k)
	a.poolRefs[template.Metadata.Name] = "upstream-engineer-template"
	a.pools["engineer-pool"] = poolEntry{upstreamTemplateName: "upstream-engineer-template", upstreamPoolName: "upstream-engineer-pool", replicas: 1}
	allocated, err := a.Allocate(runtime.AllocateRequest{ClaimID: state.Claim.ID, TemplateRef: template.Metadata.Name})
	if err != nil || allocated.ClaimID != state.Claim.ID {
		t.Fatalf("issued claim allocation = %+v/%v", allocated, err)
	}
	upstreamName := resourceName("claim", state.Claim.ID)
	if len(upstreamName) > 63 || !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(upstreamName) {
		t.Fatalf("generated claim ID %q mapped to invalid Kubernetes name %q", state.Claim.ID, upstreamName)
	}
	if _, ok := k.claims[upstreamName]; !ok {
		t.Fatalf("allocation did not create mapped upstream claim %q", upstreamName)
	}
}
