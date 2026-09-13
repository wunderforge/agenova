// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "time"

type ObjectMeta struct {
	Name string `json:"name" yaml:"name"`
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

type ClaimPhase string

const (
	ClaimPhasePending   ClaimPhase = "Pending"
	ClaimPhaseBound     ClaimPhase = "Bound"
	ClaimPhaseRunning   ClaimPhase = "Running"
	ClaimPhaseSucceeded ClaimPhase = "Succeeded"
	ClaimPhaseFailed    ClaimPhase = "Failed"
	ClaimPhaseExpired   ClaimPhase = "Expired"
)

// ToolInvocation records one concrete tool call inside a claim.
// InvocationID is the gateway-assigned correlation identity shared by the
// policy decision, the attempted external call, and this fact. Result reuses
// the issued-state DecisionResult vocabulary so a gateway decision and its
// recorded fact cannot drift apart.
type ToolInvocation struct {
	ClaimID      string
	ToolName     string
	InvocationID string
	Result       DecisionResult
	Timestamp    time.Time
}

// ModelInvocation records one concrete model call inside a claim.
// InvocationID and Result carry the same correlation semantics as
// ToolInvocation.
type ModelInvocation struct {
	ClaimID      string
	ModelName    string
	InvocationID string
	Result       DecisionResult
	Timestamp    time.Time
}

// RuntimeEvent records a lifecycle event for a claim.
type RuntimeEvent struct {
	ClaimID   string
	Kind      string
	Timestamp time.Time
}
