// Package runtime defines the RuntimeBackend boundary between Agenova's stable
// runtime contract and any concrete sandbox substrate.
//
// Application-facing Agenova APIs must not change when the selected backend
// changes. The in-memory backend is the reference implementation and the
// contract test oracle.
package runtime

import "github.com/donozhang1992/agenova/api/v1alpha1"

// RuntimeBackend is the pluggable isolation boundary for sandbox lifecycle.
// Any implementation satisfying this interface is a valid Agenova runtime
// backend. Swapping backends must be invisible to application agents.
type RuntimeBackend interface {
	AddTemplate(template v1alpha1.AgentSandboxTemplate) error
	AddWarmPool(pool v1alpha1.SandboxWarmPool) error
	AddClaim(claim v1alpha1.SandboxClaim) error
	BindClaim(name string) error
	StartClaim(name string) error
	SucceedClaim(name string) error
	FailClaim(name string, summary string) error
	ExpireClaim(name string, summary string) error
	Claim(name string) (v1alpha1.SandboxClaim, bool)
	PoolStatus(name string) (v1alpha1.SandboxWarmPoolStatus, bool)
}
