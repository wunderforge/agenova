// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// Compile-time assertion: SpikeAdapter must satisfy the RuntimeBackend contract.
var _ runtime.RuntimeBackend = (*SpikeAdapter)(nil)

const (
	defaultBindTimeout  = 60 * time.Second
	defaultStartTimeout = 120 * time.Second
	pollInterval        = 1 * time.Second
)

// SpikeAdapter is a spike RuntimeBackend that drives the upstream
// Agent Sandbox controller via kubectl. It is the only place in Agenova
// that knows about upstream Agent Sandbox CRD shapes.
//
// Semantic gaps relative to the in-memory reference backend are documented
// in doc.go and should be reviewed before any production promotion.
type SpikeAdapter struct {
	kube kubectlRunner

	mu       sync.Mutex
	pools    map[string]poolEntry  // Agenova pool name -> upstream resource names
	claims   map[string]claimEntry // claim name -> local tracking state
	poolRefs map[string]string     // Agenova template name -> upstream sandboxtemplate name
}

type poolEntry struct {
	upstreamTemplateName string
	upstreamPoolName     string
	replicas             int
}

// claimEntry tracks Agenova claim state locally. Required because the upstream
// CRD does not expose Succeeded/Failed/Expired phases - only conditions - and
// the adapter must synthesize these from local knowledge.
type claimEntry struct {
	phase     v1alpha1.ClaimPhase
	sandboxID string // upstream sandbox name assigned by controller
	errMsg    string
	replaced  bool
}

// New returns a SpikeAdapter configured for the given kubectl context and
// Kubernetes namespace.
func New(kubeContext, namespace string) *SpikeAdapter {
	return &SpikeAdapter{
		kube:     kubectlRunner{context: kubeContext, namespace: namespace},
		pools:    make(map[string]poolEntry),
		claims:   make(map[string]claimEntry),
		poolRefs: make(map[string]string),
	}
}

// AddTemplate creates an upstream SandboxTemplate CRD from an Agenova template.
// The container is named "agent" as a convention for the spike.
func (a *SpikeAdapter) AddTemplate(template v1alpha1.AgentSandboxTemplate) error {
	if template.Metadata.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if template.Spec.Image == "" {
		return fmt.Errorf("template image is required")
	}

	upstreamName := resourceName("tmpl", template.Metadata.Name)
	obj := upstreamSandboxTemplate{
		APIVersion: apiVersionExtensions,
		Kind:       kindSandboxTemplate,
		Metadata:   upstreamMeta{Name: upstreamName, Namespace: a.kube.namespace},
		Spec: upstreamSandboxTemplateSpec{
			PodTemplate: upstreamPodTemplate{
				Spec: upstreamPodSpec{
					Containers: []upstreamContainer{
						{
							Name:    "agent",
							Image:   template.Spec.Image,
							Command: template.Spec.Command,
						},
					},
				},
			},
		},
	}
	manifest, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal sandbox template: %w", err)
	}
	if err := a.kube.applyBytes(manifest); err != nil {
		return fmt.Errorf("apply sandbox template %q: %w", upstreamName, err)
	}

	a.mu.Lock()
	a.poolRefs[template.Metadata.Name] = upstreamName
	a.mu.Unlock()
	return nil
}

// AddWarmPool creates an upstream SandboxWarmPool CRD from an Agenova pool.
// The upstream pool is linked to the SandboxTemplate created by AddTemplate.
func (a *SpikeAdapter) AddWarmPool(pool v1alpha1.SandboxWarmPool) error {
	if pool.Metadata.Name == "" {
		return fmt.Errorf("pool name is required")
	}
	if pool.Spec.TemplateRef == "" {
		return fmt.Errorf("pool template ref is required")
	}
	if pool.Spec.Replicas < 1 {
		return fmt.Errorf("pool replicas must be at least 1")
	}

	a.mu.Lock()
	upstreamTemplateName, ok := a.poolRefs[pool.Spec.TemplateRef]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("template not found: %s (call AddTemplate first)", pool.Spec.TemplateRef)
	}

	upstreamPoolName := resourceName("pool", pool.Metadata.Name)
	obj := upstreamSandboxWarmPool{
		APIVersion: apiVersionExtensions,
		Kind:       kindSandboxWarmPool,
		Metadata:   upstreamMeta{Name: upstreamPoolName, Namespace: a.kube.namespace},
		Spec: upstreamSandboxWarmPoolSpec{
			Replicas:           pool.Spec.Replicas,
			SandboxTemplateRef: upstreamNameRef{Name: upstreamTemplateName},
		},
	}
	manifest, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal sandbox warm pool: %w", err)
	}
	if err := a.kube.applyBytes(manifest); err != nil {
		return fmt.Errorf("apply sandbox warm pool %q: %w", upstreamPoolName, err)
	}

	a.mu.Lock()
	a.pools[pool.Metadata.Name] = poolEntry{
		upstreamTemplateName: upstreamTemplateName,
		upstreamPoolName:     upstreamPoolName,
		replicas:             pool.Spec.Replicas,
	}
	a.mu.Unlock()
	return nil
}

// AddClaim creates an upstream SandboxClaim CRD and registers it as Pending.
// The claim is linked to the SandboxTemplate derived from the Agenova pool.
// The upstream controller assigns a sandbox asynchronously - call BindClaim to
// wait for that assignment.
func (a *SpikeAdapter) AddClaim(claim runtime.BackendClaim) error {
	if claim.Metadata.Name == "" {
		return fmt.Errorf("claim name is required")
	}
	if claim.Spec.PoolRef == "" {
		return fmt.Errorf("claim pool ref is required")
	}

	a.mu.Lock()
	if _, exists := a.claims[claim.Metadata.Name]; exists {
		a.mu.Unlock()
		return fmt.Errorf("claim already exists: %s", claim.Metadata.Name)
	}
	pool, ok := a.pools[claim.Spec.PoolRef]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("pool not found: %s", claim.Spec.PoolRef)
	}

	upstreamClaimName := resourceName("claim", claim.Metadata.Name)
	ttl := int32(300) // 5-minute spike TTL; real production would read from claim input.
	obj := upstreamSandboxClaim{
		APIVersion: apiVersionExtensions,
		Kind:       kindSandboxClaim,
		Metadata:   upstreamMeta{Name: upstreamClaimName, Namespace: a.kube.namespace},
		Spec: upstreamSandboxClaimSpec{
			SandboxTemplateRef: upstreamNameRef{Name: pool.upstreamTemplateName},
			Warmpool:           pool.upstreamPoolName,
			Lifecycle: &upstreamSandboxClaimLifecycle{
				ShutdownPolicy:          "Delete",
				TTLSecondsAfterFinished: &ttl,
			},
		},
	}
	manifest, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal sandbox claim: %w", err)
	}
	if err := a.kube.applyBytes(manifest); err != nil {
		return fmt.Errorf("apply sandbox claim %q: %w", upstreamClaimName, err)
	}

	a.mu.Lock()
	a.claims[claim.Metadata.Name] = claimEntry{phase: v1alpha1.ClaimPhasePending}
	a.mu.Unlock()
	return nil
}

// BindClaim polls the upstream controller until it assigns a sandbox to the
// claim, then transitions local state to Bound.
//
// Semantic drift: the in-memory backend binds synchronously. This adapter
// polls the upstream controller and blocks for up to defaultBindTimeout. This
// is intentional for the spike - a production adapter would use a watch.
func (a *SpikeAdapter) BindClaim(name string) error {
	if err := a.requirePhase(name, v1alpha1.ClaimPhasePending); err != nil {
		return err
	}

	upstreamName := resourceName("claim", name)
	deadline := time.Now().Add(defaultBindTimeout)
	for time.Now().Before(deadline) {
		var sc upstreamSandboxClaim
		if err := a.kube.get("sandboxclaims", upstreamName, &sc); err != nil {
			return fmt.Errorf("get claim status: %w", err)
		}
		if sc.Status.Sandbox != nil && sc.Status.Sandbox.Name != "" {
			a.mu.Lock()
			entry := a.claims[name]
			entry.phase = v1alpha1.ClaimPhaseBound
			entry.sandboxID = sc.Status.Sandbox.Name
			a.claims[name] = entry
			a.mu.Unlock()
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("bind timeout after %s: claim %s not assigned a sandbox", defaultBindTimeout, name)
}

// StartClaim polls the upstream controller until the sandbox pod is running,
// then transitions local state to Running.
//
// The upstream controller drives pod startup; this call only observes it.
func (a *SpikeAdapter) StartClaim(name string) error {
	if err := a.requirePhase(name, v1alpha1.ClaimPhaseBound); err != nil {
		return err
	}

	a.mu.Lock()
	entry := a.claims[name]
	sandboxName := entry.sandboxID
	a.mu.Unlock()

	if sandboxName == "" {
		return fmt.Errorf("claim %s is Bound but has no sandboxID; call BindClaim first", name)
	}

	deadline := time.Now().Add(defaultStartTimeout)
	for time.Now().Before(deadline) {
		var sc upstreamSandboxClaim
		if err := a.kube.get("sandboxclaims", resourceName("claim", name), &sc); err != nil {
			return fmt.Errorf("get claim status: %w", err)
		}
		if hasCondition(sc.Status.Conditions, conditionTypeReady, conditionStatusTrue) {
			a.mu.Lock()
			entry := a.claims[name]
			entry.phase = v1alpha1.ClaimPhaseRunning
			a.claims[name] = entry
			a.mu.Unlock()
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("start timeout after %s: claim %s sandbox not ready", defaultStartTimeout, name)
}

// SucceedClaim transitions local state to Succeeded and deletes the upstream
// SandboxClaim, triggering sandbox cleanup per the claim's shutdownPolicy.
//
// Gap: the upstream CRD has no "Succeeded" phase. Terminal state is kept in
// local adapter memory only; the upstream resource is deleted.
func (a *SpikeAdapter) SucceedClaim(name string) error {
	if err := a.requirePhase(name, v1alpha1.ClaimPhaseRunning); err != nil {
		return err
	}
	if err := a.kube.delete("sandboxclaims", resourceName("claim", name)); err != nil {
		return fmt.Errorf("delete claim on succeed: %w", err)
	}
	a.mu.Lock()
	entry := a.claims[name]
	entry.phase = v1alpha1.ClaimPhaseSucceeded
	entry.replaced = true
	a.claims[name] = entry
	a.mu.Unlock()
	return nil
}

// FailClaim transitions local state to Failed and deletes the upstream
// SandboxClaim, triggering sandbox cleanup per the claim's shutdownPolicy.
//
// Gap: same as SucceedClaim - no upstream "Failed" phase exists.
func (a *SpikeAdapter) FailClaim(name string, summary string) error {
	if err := a.requirePhase(name, v1alpha1.ClaimPhaseBound, v1alpha1.ClaimPhaseRunning); err != nil {
		return err
	}
	if err := a.kube.delete("sandboxclaims", resourceName("claim", name)); err != nil {
		return fmt.Errorf("delete claim on fail: %w", err)
	}
	a.mu.Lock()
	entry := a.claims[name]
	entry.phase = v1alpha1.ClaimPhaseFailed
	entry.errMsg = summary
	entry.replaced = true
	a.claims[name] = entry
	a.mu.Unlock()
	return nil
}

// ExpireClaim transitions a pending claim to Expired and deletes the upstream
// SandboxClaim without binding a sandbox.
//
// Gap: no upstream "Expired" phase; the resource is deleted immediately.
func (a *SpikeAdapter) ExpireClaim(name string, summary string) error {
	if err := a.requirePhase(name, v1alpha1.ClaimPhasePending); err != nil {
		return err
	}
	if err := a.kube.delete("sandboxclaims", resourceName("claim", name)); err != nil {
		return fmt.Errorf("delete claim on expire: %w", err)
	}
	a.mu.Lock()
	entry := a.claims[name]
	entry.phase = v1alpha1.ClaimPhaseExpired
	entry.errMsg = summary
	a.claims[name] = entry
	a.mu.Unlock()
	return nil
}

// Claim returns the Agenova view of a claim. For non-terminal phases,
// it reconciles with the upstream CRD status to keep the phase current.
func (a *SpikeAdapter) Claim(name string) (runtime.BackendClaim, bool) {
	a.mu.Lock()
	entry, ok := a.claims[name]
	a.mu.Unlock()
	if !ok {
		return runtime.BackendClaim{}, false
	}
	return runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: name},
		Spec:     runtime.BackendClaimSpec{},
		Status: runtime.BackendClaimStatus{
			Phase:           entry.phase,
			SandboxID:       entry.sandboxID,
			Error:           entry.errMsg,
			SandboxReplaced: entry.replaced,
		},
	}, true
}

// PoolStatus returns an approximated pool status derived from local claim
// tracking. The upstream SandboxWarmPool status only exposes readyReplicas and
// replicas; the Agenova breakdown is synthesized locally.
func (a *SpikeAdapter) PoolStatus(name string) (v1alpha1.SandboxWarmPoolStatus, bool) {
	a.mu.Lock()
	pool, ok := a.pools[name]
	if !ok {
		a.mu.Unlock()
		return v1alpha1.SandboxWarmPoolStatus{}, false
	}

	var bound, running, replaced int
	for _, c := range a.claims {
		switch c.phase {
		case v1alpha1.ClaimPhaseBound:
			bound++
		case v1alpha1.ClaimPhaseRunning:
			running++
		}
		if c.replaced {
			replaced++
		}
	}
	a.mu.Unlock()

	// Idle = pool capacity minus active claims. Query upstream for readyReplicas
	// as a cross-check; fall back to local estimate on error.
	var upstream upstreamSandboxWarmPool
	idle := pool.replicas - bound - running
	if err := a.kube.get("sandboxwarmpools", pool.upstreamPoolName, &upstream); err == nil {
		if upstream.Status.ReadyReplicas > 0 {
			idle = int(upstream.Status.ReadyReplicas)
		}
	}
	if idle < 0 {
		idle = 0
	}

	return v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     idle,
		BoundClaims:       bound,
		RunningClaims:     running,
		ReplacedSandboxes: replaced,
	}, true
}

// --- helpers ---

func (a *SpikeAdapter) requirePhase(name string, from ...v1alpha1.ClaimPhase) error {
	a.mu.Lock()
	entry, ok := a.claims[name]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("claim not found: %s", name)
	}
	for _, p := range from {
		if entry.phase == p {
			return nil
		}
	}
	return fmt.Errorf("invalid claim transition from %s; expected one of %v", entry.phase, from)
}

func hasCondition(conditions []upstreamCondition, condType, condStatus string) bool {
	for _, c := range conditions {
		if c.Type == condType && c.Status == condStatus {
			return true
		}
	}
	return false
}

// resourceName generates a deterministic Kubernetes resource name from an
// Agenova concept and name. Kubernetes names must be DNS labels; this
// conversion is safe for the spike's test names.
func resourceName(kind, agenovaName string) string {
	return fmt.Sprintf("agenova-%s-%s", kind, agenovaName)
}
