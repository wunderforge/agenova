// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// API wire shapes are adapted from Kubernetes SIGs Agent Sandbox v0.4.6.
// Copyright 2025 The Kubernetes Authors. See THIRD_PARTY_NOTICES.md.

package agentsandbox

// Local minimal type definitions for upstream Agent Sandbox CRD JSON shapes.
// These match extensions.agents.x-k8s.io/v1alpha1 as observed from the
// installed CRD schemas on 2026-06-15. Do not export or use outside this
// package.

const (
	apiVersionExtensions = "extensions.agents.x-k8s.io/v1alpha1"
	kindSandboxTemplate  = "SandboxTemplate"
	kindSandboxWarmPool  = "SandboxWarmPool"
	kindSandboxClaim     = "SandboxClaim"

	// conditionTypeReady is used by the agent-sandbox-controller to signal
	// that a sandbox is assigned and ready for a claim.
	conditionTypeReady = "Ready"
	// conditionStatusTrue matches "True" in a k8s condition status field.
	conditionStatusTrue = "True"
)

type upstreamMeta struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// upstreamSandboxTemplate matches extensions.agents.x-k8s.io/v1alpha1 SandboxTemplate.
type upstreamSandboxTemplate struct {
	APIVersion string                      `json:"apiVersion"`
	Kind       string                      `json:"kind"`
	Metadata   upstreamMeta                `json:"metadata"`
	Spec       upstreamSandboxTemplateSpec `json:"spec"`
}

type upstreamSandboxTemplateSpec struct {
	PodTemplate upstreamPodTemplate `json:"podTemplate"`
}

type upstreamPodTemplate struct {
	Spec upstreamPodSpec `json:"spec"`
}

type upstreamPodSpec struct {
	Containers []upstreamContainer `json:"containers"`
}

type upstreamContainer struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command,omitempty"`
}

// upstreamSandboxWarmPool matches extensions.agents.x-k8s.io/v1alpha1 SandboxWarmPool.
type upstreamSandboxWarmPool struct {
	APIVersion string                        `json:"apiVersion"`
	Kind       string                        `json:"kind"`
	Metadata   upstreamMeta                  `json:"metadata"`
	Spec       upstreamSandboxWarmPoolSpec   `json:"spec"`
	Status     upstreamSandboxWarmPoolStatus `json:"status,omitempty"`
}

type upstreamSandboxWarmPoolSpec struct {
	Replicas           int             `json:"replicas"`
	SandboxTemplateRef upstreamNameRef `json:"sandboxTemplateRef"`
}

type upstreamSandboxWarmPoolStatus struct {
	ReadyReplicas int `json:"readyReplicas"`
	Replicas      int `json:"replicas"`
}

// upstreamSandboxClaim matches extensions.agents.x-k8s.io/v1alpha1 SandboxClaim.
type upstreamSandboxClaim struct {
	APIVersion string                     `json:"apiVersion"`
	Kind       string                     `json:"kind"`
	Metadata   upstreamMeta               `json:"metadata"`
	Spec       upstreamSandboxClaimSpec   `json:"spec"`
	Status     upstreamSandboxClaimStatus `json:"status,omitempty"`
}

type upstreamSandboxClaimSpec struct {
	SandboxTemplateRef upstreamNameRef                `json:"sandboxTemplateRef"`
	Warmpool           string                         `json:"warmpool,omitempty"`
	Lifecycle          *upstreamSandboxClaimLifecycle `json:"lifecycle,omitempty"`
}

type upstreamSandboxClaimLifecycle struct {
	ShutdownPolicy          string `json:"shutdownPolicy,omitempty"`
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

type upstreamSandboxClaimStatus struct {
	Conditions []upstreamCondition `json:"conditions,omitempty"`
	Sandbox    *upstreamSandboxRef `json:"sandbox,omitempty"`
}

type upstreamSandboxRef struct {
	Name   string   `json:"name"`
	PodIPs []string `json:"podIPs,omitempty"`
}

type upstreamCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type upstreamNameRef struct {
	Name string `json:"name"`
}
