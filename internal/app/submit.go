// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"
	"strings"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authorization"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/runtime"
)

// LocalPrincipalEnv selects the reference local principal preset.
// It is operator setup, not a ClaimRequest field or an authority flag.
const LocalPrincipalEnv = "AGENOVA_LOCAL_PRINCIPAL"

// SubmitResult is the application-resolution outcome of one run -f submission.
// Allocated is always false until the #31 run service owns allocation.
type SubmitResult struct {
	RequestRef string
	Decision   v1alpha1.DecisionResult
	Principal  string
	Allocated  bool
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
// trusted local principal, and never allocates a runtime backend.
func SubmitClaimRequestFile(path string, backend runtime.RuntimeBackend, preset ReferencePrincipalPreset) (SubmitResult, error) {
	if strings.TrimSpace(path) == "" {
		return SubmitResult{}, fmt.Errorf("claim request file is required")
	}
	if backend == nil {
		return SubmitResult{}, fmt.Errorf("runtime backend is required")
	}
	// Host the backend for composition and fail-closed configuration, but do
	// not allocate. Issuance and lifecycle belong to later tickets.
	data, err := os.ReadFile(path)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("read claim request: %w", err)
	}

	policies := &policy.Loader{}
	if err := policies.Load(referencePolicyBundle()); err != nil {
		return SubmitResult{}, err
	}
	service, err := NewReferenceAssignmentService(preset, authorization.Authorizer{Policies: policies})
	if err != nil {
		return SubmitResult{}, err
	}

	result, err := service.AdmitYAML(data, func(authorization.Admission) error {
		return nil
	})
	if err != nil {
		return SubmitResult{}, err
	}
	return SubmitResult{
		RequestRef: result.RequestRef,
		Decision:   result.Decision.Result,
		Principal:  result.Principal.Subject,
		Allocated:  false,
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
