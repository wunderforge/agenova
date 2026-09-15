// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authority"
	"github.com/wunderforge/agenova/internal/authorization"
	"github.com/wunderforge/agenova/internal/issuance"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/runtime"
)

// LocalPrincipalEnv selects the reference local principal preset.
// It is operator setup, not a ClaimRequest field or an authority flag.
const LocalPrincipalEnv = "AGENOVA_LOCAL_PRINCIPAL"

// ReferenceRuntimeTemplateRef is the backend-neutral runtime-template key used
// by the canonical reference composition. A concrete backend maps this key to
// its own workload artifact at the composition edge.
const ReferenceRuntimeTemplateRef = "reference-engineer-runtime"

// SubmitResult is the backend-neutral outcome of one run -f submission.
type SubmitResult struct {
	RequestRef string
	Decision   v1alpha1.DecisionResult
	Principal  string
	Allocated  bool
	ClaimID    string
	Phase      v1alpha1.ClaimPhase
}

// PrincipalPresetFromEnv reads the local identity boundary. An empty value
// selects Team A. Unknown values fail closed.
func PrincipalPresetFromEnv() (ReferencePrincipalPreset, error) {
	value := strings.TrimSpace(os.Getenv(LocalPrincipalEnv))
	if value == "" {
		return ReferencePrincipalTeamA, nil
	}
	preset := ReferencePrincipalPreset(value)
	if _, err := NewReferencePrincipalSource(preset); err != nil {
		return "", err
	}
	return preset, nil
}

// SubmitClaimRequestFile parses one ClaimRequest YAML file, admits it with the
// trusted local principal, resolves and issues one claim, then executes that
// claim through the application-owned run service. Denied and invalid requests
// never reach claim issuance or the runtime backend.
func SubmitClaimRequestFile(path string, backend runtime.RuntimeBackend, preset ReferencePrincipalPreset) (SubmitResult, error) {
	if strings.TrimSpace(path) == "" {
		return SubmitResult{}, fmt.Errorf("claim request file is required")
	}
	if backend == nil {
		return SubmitResult{}, fmt.Errorf("runtime backend is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("read claim request: %w", err)
	}

	policies := &policy.Loader{}
	if err := policies.Load(referencePolicyBundle()); err != nil {
		return SubmitResult{}, err
	}
	request, validationErr := v1alpha1.ParseClaimRequestYAML(data)
	if validationErr != nil {
		return SubmitResult{}, validationErr
	}
	service, err := NewReferenceAssignmentService(preset, authorization.Authorizer{Policies: policies})
	if err != nil {
		return SubmitResult{}, err
	}

	var issued *v1alpha1.IssuedState
	result, err := service.AdmitYAML(data, func(admission authorization.Admission) error {
		template, templateErr := referenceAgentTemplate(request.Spec.TemplateRef)
		if templateErr != nil {
			return templateErr
		}
		resolution, resolveErr := authority.ResolveForIssuance(request, template, admission)
		if resolveErr != nil {
			return resolveErr
		}
		issued, validationErr = issuance.Issue(request, service.principals.Principal(), admission.Decision(), resolution, admission)
		if validationErr != nil {
			return validationErr
		}
		return nil
	})
	if err != nil {
		return SubmitResult{}, err
	}
	report := SubmitResult{
		RequestRef: result.RequestRef,
		Decision:   result.Decision.Result,
		Principal:  result.Principal.Subject,
	}
	if result.Decision.Result != v1alpha1.DecisionResultAllow {
		return report, nil
	}
	if issued == nil || issued.EffectiveAuthority == nil {
		return report, fmt.Errorf("allowed submission did not produce an issued claim")
	}

	runner, err := NewRunService(backend, RunServiceOptions{})
	if err != nil {
		return report, err
	}
	final, runErr := runner.Run(issued, ResolvedLaunch{
		ProfileRef:  issued.EffectiveAuthority.Runtime.ProfileRef,
		TemplateRef: ReferenceRuntimeTemplateRef,
	}, func() error { return nil })
	if final != nil && final.Claim != nil {
		report.ClaimID = final.Claim.ID
		report.Phase = final.Claim.Phase
		report.Allocated = final.Claim.BackendIdentity != nil
	}
	return report, runErr
}

// referenceAgentTemplate is the local demo registry entry. It remains a
// backend-neutral role contract; runtime template selection is separate.
func referenceAgentTemplate(templateRef string) (*v1alpha1.AgentTemplate, error) {
	if templateRef != "engineer" {
		return nil, fmt.Errorf("unknown reference AgentTemplate %q", templateRef)
	}
	maxTimeout := v1alpha1.Duration(30 * time.Minute)
	return &v1alpha1.AgentTemplate{
		APIVersion: v1alpha1.AgentTemplateAPIVersion,
		Kind:       v1alpha1.AgentTemplateKind,
		Metadata:   v1alpha1.ObjectMeta{Name: "engineer"},
		Spec: v1alpha1.AgentTemplateSpec{
			Artifact:   &v1alpha1.AgentTemplateArtifact{Image: "ghcr.io/wunderforge/agenova-engineer:v0"},
			Entrypoint: &v1alpha1.AgentTemplateEntrypoint{Command: []string{"/agenova-agent", "run"}},
			Defaults: v1alpha1.AgentTemplateDefaults{
				ModelProfile: "approved-coding-model",
				MemoryScopes: []string{"team-docs"},
			},
			CapabilityCeiling: &v1alpha1.AgentTemplateCapabilityCeiling{
				Tools:           []string{"git.read", "git.write", "github.pull-request"},
				ResourceScopes:  []string{"repo:acme/*"},
				ModelProfiles:   []string{"approved-coding-model"},
				MemoryScopes:    []string{"team-docs"},
				RuntimeProfiles: []string{"standard-isolated"},
				MaxTimeout:      &maxTimeout,
			},
		},
	}, nil
}

func referencePolicyBundle() policy.PolicyBundle {
	return policy.PolicyBundle{
		ID:      "reference-default-deny",
		Version: "1",
		Rules: []policy.Rule{{
			Team:        "team-a",
			Action:      "claim.create",
			Project:     "payments",
			TemplateRef: "engineer",
		}},
	}
}
