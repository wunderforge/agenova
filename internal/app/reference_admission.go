// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authorization"
)

// ReferenceAdmissionResult exposes the local admission outcome without
// pretending that an allowed continuation has already issued a claim.
type ReferenceAdmissionResult struct {
	RequestRef  string
	Principal   v1alpha1.Principal
	Action      v1alpha1.Action
	Decision    v1alpha1.Decision
	DeniedState *v1alpha1.IssuedState
}

// ReferenceAssignmentService composes the local principal boundary with the
// existing assignment Gate. It is intentionally local/demo-only.
type ReferenceAssignmentService struct {
	principals ReferencePrincipalSource
	gate       authorization.Gate
}

// NewReferenceAssignmentService binds one out-of-band local principal to the
// existing authorization evaluator.
func NewReferenceAssignmentService(preset ReferencePrincipalPreset, evaluator authorization.Evaluator) (*ReferenceAssignmentService, error) {
	principals, err := NewReferencePrincipalSource(preset)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, fmt.Errorf("authorization evaluator is required")
	}
	return &ReferenceAssignmentService{
		principals: principals,
		gate:       authorization.Gate{Evaluator: evaluator},
	}, nil
}

// AdmitYAML parses one canonical ClaimRequest, derives the action only from
// validated request references, and presents it to the existing Gate with the
// separately configured trusted principal.
func (s *ReferenceAssignmentService) AdmitYAML(data []byte, onAllowed func(authorization.Admission) error) (ReferenceAdmissionResult, error) {
	if s == nil {
		return ReferenceAdmissionResult{}, fmt.Errorf("reference assignment service is required")
	}
	request, validationErr := v1alpha1.ParseClaimRequestYAML(data)
	if validationErr != nil {
		return ReferenceAdmissionResult{}, validationErr
	}

	principal := s.principals.Principal()
	action := v1alpha1.Action{
		Name:        "claim.create",
		Project:     request.Spec.ProjectRef,
		TemplateRef: request.Spec.TemplateRef,
	}
	authorizationRequest := authorization.Request{
		RequestRef: request.Metadata.Name,
		Principal:  principal,
		Action:     action,
	}
	decision, err := s.gate.Admit(authorizationRequest, onAllowed)
	result := ReferenceAdmissionResult{
		RequestRef: request.Metadata.Name,
		Principal:  principal,
		Action:     action,
		Decision:   decision,
	}
	if err != nil {
		return result, err
	}
	if decision.Result != v1alpha1.DecisionResultDeny {
		return result, nil
	}

	denied := &v1alpha1.IssuedState{
		RequestRef: request.Metadata.Name,
		Principal:  principal,
		Action:     action,
		PolicyRef:  decision.PolicyRef,
		Decision:   decision,
		Evidence: v1alpha1.Evidence{
			RequestRef:       request.Metadata.Name,
			DecisionIDs:      []string{decision.ID},
			RuntimeEvents:    []v1alpha1.EvidenceRuntimeEvent{},
			ToolInvocations:  []v1alpha1.EvidenceToolInvocation{},
			ModelInvocations: []v1alpha1.EvidenceModelInvocation{},
		},
	}
	if validationErr := v1alpha1.ValidateIssuedState(denied); validationErr != nil {
		return result, fmt.Errorf("build denied issued state: %w", validationErr)
	}
	result.DeniedState = denied
	return result, nil
}
