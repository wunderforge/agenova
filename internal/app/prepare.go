// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authority"
	"github.com/wunderforge/agenova/internal/authorization"
	"github.com/wunderforge/agenova/internal/issuance"
	"github.com/wunderforge/agenova/internal/policy"
)

// PreparedAssignment separates the existing reference admission and issuance
// from execution, so CLI and the approved internal console share one path.
type PreparedAssignment struct {
	Request   *v0.ClaimRequest
	Template  *v0.AgentTemplate
	Admission ReferenceAdmissionResult
	Issued    *v0.IssuedState
	Changes   []authority.Change
}

// TemplateSource resolves a registered backend-neutral role. It must return
// an independent snapshot; later registry writes cannot expand this run.
type TemplateSource interface {
	Lookup(name string) (*v0.AgentTemplate, error)
}

type templateSourceFunc func(string) (*v0.AgentTemplate, error)

func (f templateSourceFunc) Lookup(name string) (*v0.AgentTemplate, error) { return f(name) }

func PrepareReferenceAssignment(data []byte, preset ReferencePrincipalPreset) (PreparedAssignment, error) {
	policies := &policy.Loader{}
	if err := policies.Load(referencePolicyBundle()); err != nil {
		return PreparedAssignment{}, err
	}
	principals, err := NewReferencePrincipalSource(preset)
	if err != nil {
		return PreparedAssignment{}, err
	}
	return PrepareAssignment(data, principals, policies, templateSourceFunc(referenceAgentTemplate))
}

// PrepareAssignment is shared by connected and reference compositions. The
// caller supplies trusted identity, active policy and registered templates;
// the request can supply none of those authorities.
func PrepareAssignment(data []byte, principals PrincipalSource, policies authorization.BundleSource, templates TemplateSource) (PreparedAssignment, error) {
	request, validationErr := v0.ParseClaimRequestYAML(data)
	if validationErr != nil {
		return PreparedAssignment{}, validationErr
	}
	if policies == nil || templates == nil {
		return PreparedAssignment{}, fmt.Errorf("policy and template sources are required")
	}
	service, err := NewAssignmentService(principals, authorization.Authorizer{Policies: policies})
	if err != nil {
		return PreparedAssignment{}, err
	}
	prepared := PreparedAssignment{Request: request}
	prepared.Admission, err = service.AdmitYAML(data, func(admission authorization.Admission) error {
		template, err := templates.Lookup(request.Spec.TemplateRef)
		if err != nil {
			return err
		}
		if template == nil || v0.ValidateAgentTemplate(template) != nil || template.Metadata.Name != request.Spec.TemplateRef {
			return fmt.Errorf("registered AgentTemplate %q is invalid", request.Spec.TemplateRef)
		}
		// The registry contract returns a snapshot, not an editable pointer.
		prepared.Template = template
		resolution, resolveErr := authority.ResolveForIssuance(request, template, admission)
		if resolveErr != nil {
			return resolveErr
		}
		issued, validationErr := issuance.Issue(request, service.principals.Principal(), admission.Decision(), resolution, admission)
		if validationErr != nil {
			return validationErr
		}
		prepared.Issued = issued
		prepared.Changes, _ = resolution.ChangesFor(request)
		return nil
	})
	if err != nil {
		return prepared, err
	}
	if prepared.Admission.Decision.Result == v0.DecisionResultDeny {
		prepared.Issued = prepared.Admission.DeniedState
	} else if prepared.Admission.Decision.Result == v0.DecisionResultAllow && prepared.Issued == nil {
		return prepared, fmt.Errorf("allowed submission did not produce an issued claim")
	}
	return prepared, nil
}

// ReferenceTemplate returns a fresh registry view, not an editable console object.
func ReferenceTemplate() *v0.AgentTemplate {
	template, _ := referenceAgentTemplate("engineer")
	return template
}

func ReferencePolicy() policy.PolicyBundle { return referencePolicyBundle() }
