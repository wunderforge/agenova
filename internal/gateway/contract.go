// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"fmt"

	"github.com/wunderforge/agenova/api/v1alpha1"
)

// DecisionResult is the typed outcome of one governed invocation attempt.
// It is never a boolean: ApprovalRequired is a first-class result that does
// not itself grant authority. It is the issued-state DecisionResult from the
// SandboxClaim v0 contract, not a gateway-local duplicate, so a decision and
// its recorded fact share one vocabulary.
type DecisionResult = v1alpha1.DecisionResult

const (
	ResultAllow            = v1alpha1.DecisionResultAllow
	ResultDeny             = v1alpha1.DecisionResultDeny
	ResultApprovalRequired = v1alpha1.DecisionResultApprovalRequired
)

// Stable rejection categories. Values follow the contract-fixture convention
// (kebab-case, machine-readable); "secret-value" matches the category already
// frozen in harness/fixtures/contract/v0.
const (
	CategoryMissingClaimIdentity   = "missing-claim-identity"
	CategoryIncompleteOperation    = "incomplete-operation"
	CategoryAmbiguousResourceScope = "ambiguous-resource-scope"
	CategorySecretValue            = "secret-value"
	CategoryUnknownClaim           = "unknown-claim"
	CategoryClaimNotActive         = "claim-not-active"
	CategoryOutOfParentScope       = "out-of-parent-scope"
	CategoryInvalidPolicyOutcome   = "invalid-policy-outcome"
)

// Decision correlates one governed attempt: the same InvocationID identifies
// the policy decision, any attempted external call, its result, and recorded
// evidence. The gateway issues the ID before policy evaluation; a
// caller-supplied identifier is never adopted as this trusted identity.
type Decision struct {
	InvocationID string
	Result       DecisionResult
	// Category is a stable machine-readable reason class for Deny and
	// ApprovalRequired decisions; empty for Allow.
	Category string
	Reason   string
}

// Outcome is a policy evaluation verdict, before it is bound to an
// invocation identity.
type Outcome struct {
	Result   DecisionResult
	Category string
	Reason   string
}

// Allowed is the neutral policy verdict.
func Allowed() Outcome { return Outcome{Result: ResultAllow} }

// Normalize keeps an outcome whose Result is one of the three contract values
// and fails closed otherwise. A policy that leaves Result unset must never
// reach a caller or the evidence store as an untyped decision.
func (o Outcome) Normalize() Outcome {
	switch o.Result {
	case ResultAllow, ResultDeny, ResultApprovalRequired:
		return o
	default:
		return Outcome{
			Result:   ResultDeny,
			Category: CategoryInvalidPolicyOutcome,
			Reason:   fmt.Sprintf("policy returned unsupported result %q; denying", o.Result),
		}
	}
}
