// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package runtime defines the RuntimeBackend boundary between Agenova's stable
// runtime contract and any concrete sandbox substrate.
//
// Application-facing Agenova APIs must not change when the selected backend
// changes. The in-memory backend is the reference implementation and the
// contract test oracle.
package runtime

import "github.com/wunderforge/agenova/api/v1alpha1"

// RuntimeBackend is the pluggable isolation boundary for sandbox lifecycle.
// Any implementation satisfying this interface is a valid Agenova runtime
// backend. Swapping backends must be invisible to application agents.
type RuntimeBackend interface {
	AddTemplate(template v1alpha1.AgentSandboxTemplate) error
	AddWarmPool(pool v1alpha1.SandboxWarmPool) error
	AddClaim(claim BackendClaim) error
	BindClaim(name string) error
	StartClaim(name string) error
	SucceedClaim(name string) error
	FailClaim(name string, summary string) error
	ExpireClaim(name string, summary string) error
	Claim(name string) (BackendClaim, bool)
	PoolStatus(name string) (v1alpha1.SandboxWarmPoolStatus, bool)
}

// BackendClaim is the runtime backend's own reference-runtime view of a claim
// assignment. It is distinct from the public, backend-neutral api/v1alpha1
// authority/decision contract (SandboxClaim et al.): this shape is an
// implementation-internal projection used to drive the lifecycle state
// machine across RuntimeBackend implementations, not a public contract.
type BackendClaim struct {
	Metadata v1alpha1.ObjectMeta
	Spec     BackendClaimSpec
	Status   BackendClaimStatus
}

type BackendClaimSpec struct {
	PoolRef string
	Input   map[string]string
}

type BackendClaimStatus struct {
	Phase     v1alpha1.ClaimPhase
	SandboxID string
	Error     string

	// SandboxReplaced records the resource-side fact that the sandbox bound
	// to this claim was destroyed and replaced after the claim reached a
	// terminal phase. In the Kubernetes-facing shape this becomes a status
	// condition (SandboxReplaced=True), not a claim phase: claim phases keep
	// the business outcome (Succeeded/Failed/Expired); sandbox cleanup is a
	// resource fact.
	SandboxReplaced bool
}
