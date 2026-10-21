// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package authority resolves requested access into backend-neutral effective
// authority after assignment admission has explicitly allowed the request.
package authority

import (
	"fmt"
	"strings"
	"time"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authorization"
	"github.com/wunderforge/agenova/internal/policy"
)

// Resolve intersects a validated request with a validated AgentTemplate
// ceiling after the authorization Gate admits that exact request context. It
// returns a fresh snapshot and never mutates either input.
func Resolve(request *v1alpha1.ClaimRequest, template *v1alpha1.AgentTemplate, admission authorization.Admission) (*v1alpha1.EffectiveAuthority, *v1alpha1.ValidationError) {
	if err := v1alpha1.ValidateClaimRequest(request); err != nil {
		return nil, err
	}
	if err := v1alpha1.ValidateAgentTemplate(template); err != nil {
		return nil, err
	}
	if request.Spec.TemplateRef != template.Metadata.Name {
		return nil, invalid("spec.templateRef", "must match the resolved AgentTemplate")
	}
	if !admission.Matches(request.Metadata.Name, request.Spec.ProjectRef, request.Spec.TemplateRef) {
		return nil, invalid("admission", "must be issued by the authorization Gate for this request, project, and template")
	}

	ceiling := template.Spec.CapabilityCeiling
	caps := admission.Constraints()
	for _, dimension := range []struct {
		path      string
		requested []string
		cap       *policy.StringSetConstraint
	}{
		{"spec.requestedAccess.tools", request.Spec.RequestedAccess.Tools, capSet(caps, "tools")},
		{"spec.requestedAccess.resourceScopes", request.Spec.RequestedAccess.ResourceScopes, capSet(caps, "resources")},
		{"spec.requestedAccess.memoryScopes", request.Spec.RequestedAccess.MemoryScopes, capSet(caps, "memory")},
		{"spec.requestedAccess.modelProfile", []string{request.Spec.RequestedAccess.ModelProfile}, capSet(caps, "models")},
		{"spec.runtime.profileRef", []string{request.Spec.Runtime.ProfileRef}, capSet(caps, "runtimes")},
	} {
		if dimension.cap == nil {
			continue
		}
		for _, value := range dimension.cap.Values {
			if !contains(dimension.requested, value) {
				return nil, invalid(dimension.path, "policy constraint contains unrequested authority")
			}
		}
	}
	if err := validateScopeCeiling(ceiling.ResourceScopes); err != nil {
		return nil, err
	}

	tools := exactIntersection(request.Spec.RequestedAccess.Tools, ceiling.Tools)
	tools = applySetCap(tools, capSet(caps, "tools"))
	if len(request.Spec.RequestedAccess.Tools) > 0 && len(tools) == 0 {
		return nil, invalid("spec.requestedAccess.tools", "requested tools resolved completely empty")
	}
	resources, err := resourceIntersection(request.Spec.RequestedAccess.ResourceScopes, ceiling.ResourceScopes)
	if err != nil {
		return nil, err
	}
	resources = applySetCap(resources, capSet(caps, "resources"))
	if len(request.Spec.RequestedAccess.ResourceScopes) > 0 && len(resources) == 0 {
		return nil, invalid("spec.requestedAccess.resourceScopes", "requested resource scopes resolved completely empty")
	}
	memory := exactIntersection(request.Spec.RequestedAccess.MemoryScopes, ceiling.MemoryScopes)
	memory = applySetCap(memory, capSet(caps, "memory"))
	if len(request.Spec.RequestedAccess.MemoryScopes) > 0 && len(memory) == 0 {
		return nil, invalid("spec.requestedAccess.memoryScopes", "requested memory scopes resolved completely empty")
	}
	// Network authority requires positive consent from every ceiling. Unlike
	// legacy dimensions, an absent Policy cap is deliberately default-deny.
	// A cap may mention public-web for a reusable Policy while an offline Work
	// omits it; the cap never manufactures an unsolicited grant.
	network := request.Spec.RequestedAccess.NetworkProfiles
	for _, profile := range network {
		if !contains(ceiling.NetworkProfiles, profile) {
			return nil, invalid("spec.requestedAccess.networkProfiles", "requested network profile is outside the template ceiling")
		}
		cap := capSet(caps, "network")
		if cap == nil || !contains(cap.Values, profile) {
			return nil, invalid("spec.requestedAccess.networkProfiles", "requested network profile requires an explicit matching policy constraint")
		}
	}
	network = append([]string(nil), network...)

	model := request.Spec.RequestedAccess.ModelProfile
	if model != "" && !contains(ceiling.ModelProfiles, model) {
		return nil, invalid("spec.requestedAccess.modelProfile", "requested model profile is outside the template ceiling")
	}
	if cap := capSet(caps, "models"); cap != nil && model != "" && !contains(cap.Values, model) {
		return nil, invalid("spec.requestedAccess.modelProfile", "policy constraint excludes requested model profile")
	}
	runtimeProfile := request.Spec.Runtime.ProfileRef
	if !contains(ceiling.RuntimeProfiles, runtimeProfile) {
		return nil, invalid("spec.runtime.profileRef", "requested runtime profile is outside the template ceiling")
	}
	if cap := capSet(caps, "runtimes"); cap != nil && !contains(cap.Values, runtimeProfile) {
		return nil, invalid("spec.runtime.profileRef", "policy constraint excludes requested runtime profile")
	}
	if ceiling.MaxTimeout == nil || time.Duration(*ceiling.MaxTimeout) <= 0 {
		return nil, invalid("spec.capabilityCeiling.maxTimeout", "a positive runtime timeout ceiling is required")
	}
	timeout := *request.Spec.Runtime.Timeout
	if *ceiling.MaxTimeout < timeout {
		timeout = *ceiling.MaxTimeout
	}
	if caps != nil && caps.MaxTimeout != nil && v1alpha1.Duration(*caps.MaxTimeout) < timeout {
		timeout = v1alpha1.Duration(*caps.MaxTimeout)
	}

	return &v1alpha1.EffectiveAuthority{
		Tools:           tools,
		ResourceScopes:  resources,
		ModelProfile:    model,
		MemoryScopes:    memory,
		NetworkProfiles: network,
		Runtime: v1alpha1.EffectiveAuthorityRuntime{
			ProfileRef: runtimeProfile,
			Timeout:    timeout,
		},
	}, nil
}

func capSet(caps *policy.AuthorityConstraints, dimension string) *policy.StringSetConstraint {
	if caps == nil {
		return nil
	}
	switch dimension {
	case "tools":
		return caps.Tools
	case "resources":
		return caps.ResourceScopes
	case "memory":
		return caps.MemoryScopes
	case "models":
		return caps.ModelProfiles
	case "runtimes":
		return caps.RuntimeProfiles
	case "network":
		return caps.NetworkProfiles
	}
	return nil
}

func applySetCap(values []string, cap *policy.StringSetConstraint) []string {
	if cap == nil {
		return values
	}
	return exactIntersection(values, cap.Values)
}

func exactIntersection(requested, allowed []string) []string {
	if len(requested) == 0 {
		return nil
	}
	result := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, value := range requested {
		if !contains(allowed, value) {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func resourceIntersection(requested, ceilings []string) ([]string, *v1alpha1.ValidationError) {
	if len(requested) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for index, scope := range requested {
		if strings.Contains(scope, "*") {
			return nil, invalid(fmt.Sprintf("spec.requestedAccess.resourceScopes[%d]", index), "requested resource scope must be concrete")
		}
		allowed := false
		for _, ceiling := range ceilings {
			if ceiling == scope || strings.HasSuffix(ceiling, "*") && strings.HasPrefix(scope, strings.TrimSuffix(ceiling, "*")) {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		if _, duplicate := seen[scope]; duplicate {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	return result, nil
}

func validateScopeCeiling(scopes []string) *v1alpha1.ValidationError {
	for index, scope := range scopes {
		stars := strings.Count(scope, "*")
		if stars == 0 {
			continue
		}
		if stars != 1 || !strings.HasSuffix(scope, "*") || scope == "*" {
			return invalid(fmt.Sprintf("spec.capabilityCeiling.resourceScopes[%d]", index), "only one terminal wildcard is supported")
		}
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func invalid(path, detail string) *v1alpha1.ValidationError {
	return &v1alpha1.ValidationError{
		Category:  v1alpha1.ValidationCategoryInvalidValue,
		FieldPath: path,
		Detail:    detail,
	}
}
