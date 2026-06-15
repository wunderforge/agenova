// Package agentsandbox is the ONLY package permitted to import or reference
// upstream Agent Sandbox CRD types, manifests, or API group names.
//
// It implements a spike RuntimeBackend adapter that drives the upstream
// Agent Sandbox controller (sigs.k8s.io/agent-sandbox) via kubectl for local
// e2e validation. This is a spike backend - explicit, thin, and close to the
// raw upstream API - not a production implementation.
//
// # Semantic gap summary
//
// The upstream Agent Sandbox model differs from Agenova's explicit lifecycle
// contract in three ways discovered during Phase 2:
//
//  1. No phase field. Upstream SandboxClaim uses k8s-standard conditions
//     (status.conditions), not a phase string. The adapter maps conditions to
//     Agenova phases; BindClaim and StartClaim poll for controller-driven
//     transitions rather than triggering them directly.
//
//  2. No SucceedClaim / FailClaim primitives. The upstream controller manages
//     sandbox termination through pod lifecycle and lifecycle.shutdownPolicy.
//     Agenova's explicit terminal transitions are tracked in local adapter
//     state and realized by deleting the upstream claim. The "Succeeded" or
//     "Failed" phase is not durable in the upstream CRD.
//
//  3. Pool status granularity. Upstream SandboxWarmPool status exposes only
//     readyReplicas and replicas; it does not break down IdleSandboxes,
//     BoundClaims, RunningClaims, or ReplacedSandboxes per the Agenova model.
//     The adapter approximates these from local claim tracking state.
//
// # RuntimeBackend boundary
//
// Application-facing Agenova APIs must not import or reference this package.
// The upstream API group strings ("agents.x-k8s.io", "extensions.agents.x-k8s.io")
// and all upstream type definitions are confined here.
package agentsandbox
