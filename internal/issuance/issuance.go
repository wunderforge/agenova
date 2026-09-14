// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package issuance turns an admitted assignment and resolved authority into
// one backend-neutral, system-managed Pending claim snapshot.
package issuance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authorization"
)

// Issue produces a fresh issued snapshot. The caller must pass the trusted
// principal and the authority previously resolved by #28; this pure step
// neither authorizes the assignment nor recomputes its authority ceiling.
// A public Decision alone cannot serve as an admission token.
func Issue(
	request *v1alpha1.ClaimRequest,
	principal v1alpha1.Principal,
	decision v1alpha1.Decision,
	resolved *v1alpha1.EffectiveAuthority,
	admission authorization.Admission,
) (*v1alpha1.IssuedState, *v1alpha1.ValidationError) {
	if err := v1alpha1.ValidateClaimRequest(request); err != nil {
		return nil, err
	}
	action := v1alpha1.Action{
		Name:        "claim.create",
		Project:     request.Spec.ProjectRef,
		TemplateRef: request.Spec.TemplateRef,
	}
	context := authorization.Request{
		RequestRef: request.Metadata.Name,
		Principal:  principal,
		Action:     action,
	}
	if !admission.MatchesContext(context, decision) {
		return nil, invalid("admission", "must be Gate-issued for this request, trusted principal, and decision")
	}
	if resolved == nil {
		return nil, required("effectiveAuthority")
	}
	if resolved.ID != "" {
		return nil, invalid("effectiveAuthority.id", "identity must be assigned by issuance")
	}
	if strings.TrimSpace(resolved.Runtime.ProfileRef) == "" {
		return nil, required("effectiveAuthority.runtime.profileRef")
	}
	if time.Duration(resolved.Runtime.Timeout) <= 0 {
		return nil, invalid("effectiveAuthority.runtime.timeout", "must be a positive duration")
	}
	for _, dimension := range []struct {
		path   string
		values []string
	}{
		{"effectiveAuthority.tools", resolved.Tools},
		{"effectiveAuthority.resourceScopes", resolved.ResourceScopes},
		{"effectiveAuthority.memoryScopes", resolved.MemoryScopes},
	} {
		for index, value := range dimension.values {
			if strings.TrimSpace(value) == "" {
				return nil, invalid(fmt.Sprintf("%s[%d]", dimension.path, index), "value must be non-blank")
			}
		}
	}

	// Copy every slice before assigning IDs. Neither later source mutations nor
	// callers mutating a returned snapshot can change the resolver's output.
	authority := *resolved
	authority.Tools = cloneStrings(resolved.Tools)
	authority.ResourceScopes = cloneStrings(resolved.ResourceScopes)
	authority.MemoryScopes = cloneStrings(resolved.MemoryScopes)

	id, err := issuanceDigest(request, principal, action, decision, authority)
	if err != nil {
		return nil, invalid("metadata.name", "validated request could not be encoded for issuance")
	}
	claimID := fmt.Sprintf("claim:%s:issuance:%x", request.Metadata.Name, id[:16])
	authority.ID = fmt.Sprintf("authority:%s:issuance:%x", request.Metadata.Name, id[:16])
	claim := &v1alpha1.SandboxClaim{
		ID:           claimID,
		RequestRef:   request.Metadata.Name,
		TemplateRef:  request.Spec.TemplateRef,
		AuthorityRef: authority.ID,
		Phase:        v1alpha1.ClaimPhasePending,
	}
	state := &v1alpha1.IssuedState{
		RequestRef:         request.Metadata.Name,
		Principal:          principal,
		Action:             action,
		PolicyRef:          decision.PolicyRef,
		EffectiveAuthority: &authority,
		Claim:              claim,
		Decision:           decision,
		Evidence: v1alpha1.Evidence{
			RequestRef:       request.Metadata.Name,
			ClaimID:          claimID,
			DecisionIDs:      []string{decision.ID},
			RuntimeEvents:    []v1alpha1.EvidenceRuntimeEvent{},
			ToolInvocations:  []v1alpha1.EvidenceToolInvocation{},
			ModelInvocations: []v1alpha1.EvidenceModelInvocation{},
		},
	}
	if err := v1alpha1.ValidateIssuedState(state); err != nil {
		return nil, err
	}
	return state, nil
}

func issuanceDigest(
	request *v1alpha1.ClaimRequest,
	principal v1alpha1.Principal,
	action v1alpha1.Action,
	decision v1alpha1.Decision,
	authority v1alpha1.EffectiveAuthority,
) ([sha256.Size]byte, error) {
	var payload strings.Builder
	for _, value := range []any{request, principal, action, decision, authority} {
		encoded, err := json.Marshal(value)
		if err != nil {
			return [sha256.Size]byte{}, err
		}
		fmt.Fprintf(&payload, "%d:", len(encoded))
		payload.Write(encoded)
	}
	return sha256.Sum256([]byte(payload.String())), nil
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func required(path string) *v1alpha1.ValidationError {
	return &v1alpha1.ValidationError{
		Category:  v1alpha1.ValidationCategoryRequiredField,
		FieldPath: path,
		Detail:    "value is required",
	}
}

func invalid(path, detail string) *v1alpha1.ValidationError {
	return &v1alpha1.ValidationError{
		Category:  v1alpha1.ValidationCategoryInvalidValue,
		FieldPath: path,
		Detail:    detail,
	}
}
