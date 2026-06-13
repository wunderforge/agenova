package v1alpha1

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
