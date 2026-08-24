// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "time"

type ObjectMeta struct {
	Name string
}

type AgentSandboxTemplate struct {
	Metadata ObjectMeta
	Spec     AgentSandboxTemplateSpec
}

type AgentSandboxTemplateSpec struct {
	Image   string
	Command []string
}

type SandboxWarmPool struct {
	Metadata ObjectMeta
	Spec     SandboxWarmPoolSpec
	Status   SandboxWarmPoolStatus
}

type SandboxWarmPoolSpec struct {
	TemplateRef string
	Replicas    int
}

type SandboxWarmPoolStatus struct {
	IdleSandboxes int
	BoundClaims   int
	RunningClaims int

	// ReplacedSandboxes is a cumulative counter of sandboxes destroyed and
	// replaced after their claim reached a terminal phase. It is not derived
	// from the active sandbox list.
	ReplacedSandboxes int
}

type SandboxClaim struct {
	Metadata ObjectMeta
	Spec     SandboxClaimSpec
	Status   SandboxClaimStatus
}

type SandboxClaimSpec struct {
	PoolRef string
	Input   map[string]string
}

type ClaimPhase string

const (
	ClaimPhasePending   ClaimPhase = "Pending"
	ClaimPhaseBound     ClaimPhase = "Bound"
	ClaimPhaseRunning   ClaimPhase = "Running"
	ClaimPhaseSucceeded ClaimPhase = "Succeeded"
	ClaimPhaseFailed    ClaimPhase = "Failed"
	ClaimPhaseExpired   ClaimPhase = "Expired"
)

type SandboxClaimStatus struct {
	Phase     ClaimPhase
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

// ToolInvocation records one concrete tool call inside a claim.
type ToolInvocation struct {
	ClaimID   string
	ToolName  string
	Timestamp time.Time
}

// ModelInvocation records one concrete model call inside a claim.
type ModelInvocation struct {
	ClaimID   string
	ModelName string
	Timestamp time.Time
}

// RuntimeEvent records a lifecycle event for a claim.
type RuntimeEvent struct {
	ClaimID   string
	Kind      string
	Timestamp time.Time
}
