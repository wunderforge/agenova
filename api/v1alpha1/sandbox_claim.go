// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// DecisionResult is the typed outcome vocabulary for one authorization
// decision. It is never represented as a boolean in the public contract.
type DecisionResult string

const (
	DecisionResultAllow            DecisionResult = "Allow"
	DecisionResultDeny             DecisionResult = "Deny"
	DecisionResultApprovalRequired DecisionResult = "ApprovalRequired"
)

// Principal is the authenticated subject carried inside issued state.
type Principal struct {
	Subject               string `json:"subject"`
	Team                  string `json:"team"`
	AuthenticationContext string `json:"authenticationContext"`
}

// Action is the requested/decided operation carried inside issued state.
type Action struct {
	Name        string `json:"name"`
	Project     string `json:"project"`
	TemplateRef string `json:"templateRef"`
}

// PolicyReference identifies the policy version that produced a decision.
type PolicyReference struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// EffectiveAuthority is the system-issued grant distinct from any requested
// access. It is never caller-suppliable.
type EffectiveAuthority struct {
	ID             string                    `json:"id"`
	Tools          []string                  `json:"tools,omitempty"`
	ResourceScopes []string                  `json:"resourceScopes,omitempty"`
	ModelProfile   string                    `json:"modelProfile,omitempty"`
	MemoryScopes   []string                  `json:"memoryScopes,omitempty"`
	Runtime        EffectiveAuthorityRuntime `json:"runtime"`
}

type EffectiveAuthorityRuntime struct {
	ProfileRef string `json:"profileRef"`
	Timeout    string `json:"timeout"`
}

// SandboxClaimBackendIdentity is the system-allocated backend/worker binding.
// It only exists once a claim has been allocated (Bound phase onward); a
// Pending claim has not been assigned one yet.
type SandboxClaimBackendIdentity struct {
	Backend  string `json:"backend"`
	WorkerID string `json:"workerId"`
}

// SandboxClaim is the v0 backend-neutral issued-state claim: one immutable
// snapshot of what was actually granted for one request, distinct from any
// requested access. It is system-issued only.
//
// This is Agenova's single canonical public SandboxClaim. The unrelated
// Kubernetes-facing reference-runtime prototype that previously used this
// name now lives as internal/runtime.BackendClaim.
type SandboxClaim struct {
	ID              string                       `json:"id"`
	RequestRef      string                       `json:"requestRef"`
	TemplateRef     string                       `json:"templateRef"`
	AuthorityRef    string                       `json:"authorityRef"`
	Phase           ClaimPhase                   `json:"phase"`
	BackendIdentity *SandboxClaimBackendIdentity `json:"backendIdentity,omitempty"`
}

// Decision is one typed authorization outcome. Result is never represented
// as a boolean, and ApprovalRequired is never treated as granted authority.
type Decision struct {
	ID           string          `json:"id"`
	PrincipalRef string          `json:"principalRef"`
	Action       string          `json:"action"`
	Result       DecisionResult  `json:"result"`
	PolicyRef    PolicyReference `json:"policyRef"`
	Reason       string          `json:"reason,omitempty"`
}

// EvidenceRuntimeEvent is the lightweight evidence-view of a runtime
// lifecycle event. It is intentionally distinct from the internal
// RuntimeEvent fact struct, which carries a ClaimID/timestamp that the
// evidence envelope already provides at the parent level.
type EvidenceRuntimeEvent struct {
	Kind string `json:"kind"`
}

// EvidenceToolInvocation is a minimal evidence-view placeholder for a tool
// call. Its detailed item schema is owned by #33, not this ticket.
type EvidenceToolInvocation struct {
	ToolName string `json:"toolName,omitempty"`
}

// EvidenceModelInvocation is a minimal evidence-view placeholder for a model
// call. Its detailed item schema is owned by #37, not this ticket.
type EvidenceModelInvocation struct {
	ModelName string `json:"modelName,omitempty"`
}

// Evidence is the standalone evidentiary envelope for one request. It
// validates on its own even for a pre-claim denial, with no fabricated claim.
type Evidence struct {
	RequestRef       string                    `json:"requestRef"`
	ClaimID          string                    `json:"claimId,omitempty"`
	DecisionIDs      []string                  `json:"decisionIds,omitempty"`
	RuntimeEvents    []EvidenceRuntimeEvent    `json:"runtimeEvents,omitempty"`
	ToolInvocations  []EvidenceToolInvocation  `json:"toolInvocations,omitempty"`
	ModelInvocations []EvidenceModelInvocation `json:"modelInvocations,omitempty"`
}

// IssuedState is one immutable snapshot of everything decided, and (only on
// Allow) granted, for one request. It is the parse result of both
// ParseCallerIssuedState and ParseSystemIssuedState.
type IssuedState struct {
	RequestRef         string              `json:"requestRef"`
	Principal          Principal           `json:"principal"`
	Action             Action              `json:"action"`
	PolicyRef          PolicyReference     `json:"policyRef"`
	EffectiveAuthority *EffectiveAuthority `json:"effectiveAuthority,omitempty"`
	Claim              *SandboxClaim       `json:"claim,omitempty"`
	Decision           Decision            `json:"decision"`
	Evidence           Evidence            `json:"evidence"`
}

// ParseSystemIssuedState parses a document from a trusted, system-issued
// entrypoint: effectiveAuthority, claim.phase, and claim.backendIdentity are
// accepted. Origin is a property of which function the caller invokes, never
// a field inside data.
func ParseSystemIssuedState(data []byte) (*IssuedState, *ValidationError) {
	state, err := decodeIssuedState(data)
	if err != nil {
		return nil, err
	}
	if err := ValidateIssuedState(state); err != nil {
		return nil, err
	}
	return state, nil
}

// ParseCallerIssuedState parses a document from a caller-facing entrypoint:
// effectiveAuthority, claim.phase, and claim.backendIdentity are rejected
// with category system-managed-field if present, regardless of any
// "source"-style field inside the payload — that field is never read.
func ParseCallerIssuedState(data []byte) (*IssuedState, *ValidationError) {
	if err := rejectCallerManagedFields(data); err != nil {
		return nil, err
	}
	state, err := decodeIssuedState(data)
	if err != nil {
		return nil, err
	}
	if err := ValidateIssuedState(state); err != nil {
		return nil, err
	}
	return state, nil
}

// ValidateIssuedState checks the cross-object invariants that keep one
// snapshot from contradicting itself: reference correlation across request,
// principal, action, policy, authority, claim, decision, and evidence, plus
// the Allow-only-carries-authority and Pending-has-no-backendIdentity rules.
func ValidateIssuedState(state *IssuedState) *ValidationError {
	if state == nil {
		return validationError(ValidationCategoryRequiredField, "$", "issued state is required")
	}
	if strings.TrimSpace(state.RequestRef) == "" {
		return validationError(ValidationCategoryRequiredField, "requestRef", "value is required")
	}
	if strings.TrimSpace(state.Principal.Subject) == "" {
		return validationError(ValidationCategoryRequiredField, "principal.subject", "value is required")
	}
	if strings.TrimSpace(state.Action.Name) == "" {
		return validationError(ValidationCategoryRequiredField, "action.name", "value is required")
	}
	if strings.TrimSpace(state.Decision.ID) == "" {
		return validationError(ValidationCategoryRequiredField, "decision.id", "value is required")
	}
	switch state.Decision.Result {
	case DecisionResultAllow, DecisionResultDeny, DecisionResultApprovalRequired:
	default:
		return validationError(ValidationCategoryInvalidValue, "decision.result", "must be one of Allow, Deny, ApprovalRequired")
	}
	if state.Decision.PrincipalRef != state.Principal.Subject {
		return validationError(ValidationCategoryInvalidValue, "decision.principalRef", "must match principal.subject")
	}
	if state.Decision.Action != state.Action.Name {
		return validationError(ValidationCategoryInvalidValue, "decision.action", "must match action.name")
	}
	if state.Decision.PolicyRef != state.PolicyRef {
		return validationError(ValidationCategoryInvalidValue, "decision.policyRef", "must match the issued policyRef")
	}

	if state.Decision.Result == DecisionResultAllow {
		if state.Claim == nil {
			return validationError(ValidationCategoryInvalidValue, "claim", "required when decision.result is Allow")
		}
		if state.EffectiveAuthority == nil {
			return validationError(ValidationCategoryInvalidValue, "effectiveAuthority", "required when decision.result is Allow")
		}
	} else {
		if state.Claim != nil {
			return validationError(ValidationCategoryInvalidValue, "claim", "must be absent unless decision.result is Allow")
		}
		if state.EffectiveAuthority != nil {
			return validationError(ValidationCategoryInvalidValue, "effectiveAuthority", "must be absent unless decision.result is Allow")
		}
	}

	if state.Claim != nil {
		if state.Claim.RequestRef != state.RequestRef {
			return validationError(ValidationCategoryInvalidValue, "claim.requestRef", "must match the top-level requestRef")
		}
		if state.EffectiveAuthority != nil && state.Claim.AuthorityRef != state.EffectiveAuthority.ID {
			return validationError(ValidationCategoryInvalidValue, "claim.authorityRef", "must match effectiveAuthority.id")
		}
		if state.Claim.Phase == ClaimPhasePending && state.Claim.BackendIdentity != nil {
			return validationError(ValidationCategoryInvalidValue, "claim.backendIdentity", "must be absent while the claim is Pending")
		}
	}

	if strings.TrimSpace(state.Evidence.RequestRef) == "" {
		return validationError(ValidationCategoryRequiredField, "evidence.requestRef", "value is required")
	}
	if state.Evidence.RequestRef != state.RequestRef {
		return validationError(ValidationCategoryInvalidValue, "evidence.requestRef", "must match the top-level requestRef")
	}
	if !contains(state.Evidence.DecisionIDs, state.Decision.ID) {
		return validationError(ValidationCategoryInvalidValue, "evidence.decisionIds", "must contain the emitted decision id")
	}
	if state.Evidence.ClaimID != "" {
		if state.Claim == nil {
			return validationError(ValidationCategoryInvalidValue, "evidence.claimId", "must be absent unless decision.result is Allow")
		}
		if state.Evidence.ClaimID != state.Claim.ID {
			return validationError(ValidationCategoryInvalidValue, "evidence.claimId", "must match claim.id")
		}
	}

	return nil
}

func decodeIssuedState(data []byte) (*IssuedState, *ValidationError) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var state IssuedState
	if err := decoder.Decode(&state); err != nil {
		return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return nil, validationError(ValidationCategoryInvalidDocument, "$", err.Error())
		}
		return nil, validationError(ValidationCategoryInvalidDocument, "$", "multiple JSON documents are not allowed")
	}
	return &state, nil
}

// rejectCallerManagedFields checks for system-managed keys by exact path in
// the raw document, independent of and prior to typed decoding, so it works
// on the partial documents a caller may submit. It never reads any
// "source"-style field — origin is established by which parse function the
// caller invoked, not by payload content.
func rejectCallerManagedFields(data []byte) *ValidationError {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return validationError(ValidationCategoryInvalidDocument, "$", err.Error())
	}
	if _, present := raw["effectiveAuthority"]; present {
		return validationError(ValidationCategorySystemManagedField, "effectiveAuthority", "field is system-managed and cannot be caller-supplied")
	}
	if claimRaw, present := raw["claim"]; present {
		var claim map[string]json.RawMessage
		if err := json.Unmarshal(claimRaw, &claim); err != nil {
			return validationError(ValidationCategoryInvalidDocument, "claim", err.Error())
		}
		if _, present := claim["phase"]; present {
			return validationError(ValidationCategorySystemManagedField, "claim.phase", "field is system-managed and cannot be caller-supplied")
		}
		if _, present := claim["backendIdentity"]; present {
			return validationError(ValidationCategorySystemManagedField, "claim.backendIdentity", "field is system-managed and cannot be caller-supplied")
		}
	}
	return nil
}
