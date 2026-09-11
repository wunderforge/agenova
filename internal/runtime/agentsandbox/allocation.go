// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// BackendName is the identity Backend value reported by this adapter.
const BackendName = "agent-sandbox"

var (
	// errCleanupUnconfirmed marks a cleanup or compensation whose resource
	// release could not be confirmed. It is retryable.
	errCleanupUnconfirmed = errors.New("agentsandbox: resource release not confirmed")
	// errWorkerUnknown marks an attempt whose assigned worker was never
	// observed. Absence of the upstream claim does not prove that no worker
	// exists, so release cannot be confirmed for such an attempt.
	errWorkerUnknown = errors.New("agentsandbox: worker identity never observed, release cannot be confirmed")
	// errIdentityMismatch marks an upstream claim whose current worker no
	// longer matches the recorded identity.
	errIdentityMismatch = errors.New("agentsandbox: upstream worker does not match the recorded identity")
	// errIdentityConflict marks an attempt whose assigned worker already
	// belongs to another claim. Such an attempt is never compensated
	// destructively, because deleting its claim can destroy the other
	// claim's worker.
	errIdentityConflict = errors.New("agentsandbox: upstream worker already associated with another claim")
	// errRecoveryInProgress marks a concurrent recovery of the same ClaimID.
	errRecoveryInProgress = errors.New("agentsandbox: recovery already in progress for this claim")
)

type allocationState int

const (
	// allocPending: the attempt is recorded and the upstream claim may exist.
	allocPending allocationState = iota
	// allocBound: the controller assigned a sandbox; WorkerID is known.
	allocBound
	// allocReleased: Cleanup confirmed claim and sandbox are gone.
	allocReleased
	// allocRecoveryPending: a failed allocation could not be compensated
	// with confirmed evidence. The record is kept; a retry with the same
	// ClaimID continues recovery instead of creating a new claim.
	allocRecoveryPending
	// allocConflicted: the controller assigned a worker that already belongs
	// to another claim. The attempt's upstream claim is deliberately left in
	// place, because deleting it may destroy the other claim's worker. This
	// state is terminal for the ClaimID and needs operator intervention.
	allocConflicted
)

// allocationEntry is this adapter's in-process bookkeeping for one reduced
// contract allocation attempt. It carries no application phase.
// All mutable fields are read and written under SpikeAdapter.mu. claimID and
// upstreamClaimName are immutable after construction.
type allocationEntry struct {
	claimID           string
	upstreamClaimName string

	// workerID is set only once the upstream status was actually read and
	// carried a sandbox name. An empty workerID means "never observed", not
	// "no worker exists": the controller may bind at any time, so absence of
	// an observation is never evidence that nothing was allocated.
	workerID string
	state    allocationState
	// applyConfirmed records that the create request returned success.
	applyConfirmed bool
	// conflictWorkerID is kept for diagnostics when the assigned worker
	// belongs to another claim. It is never used to delete anything.
	conflictWorkerID string
	// recovering guards one in-flight recovery per ClaimID.
	recovering bool
	detail     string
}

func (e *allocationEntry) identity() v1alpha1.SandboxClaimBackendIdentity {
	return v1alpha1.SandboxClaimBackendIdentity{Backend: BackendName, WorkerID: e.workerID}
}

// Allocate implements runtime.RuntimeBackend.
//
// It creates the upstream SandboxClaim and waits for the controller to assign
// a sandbox. Failed attempts are recovered only after their worker is known
// and reserved for this claim. Unknown or conflicting identities never cause
// a destructive delete.
//
// Recovery only drops the record when the worker was actually observed and
// then confirmed absent. An attempt whose worker was never observed stays
// recovery-pending, because this adapter cannot prove through the upstream
// API that no worker was assigned: the controller may bind between a status
// read and the delete, and a missing claim may have been created, bound and
// removed without ever being seen. Such a record is retried, never silently
// reused.
func (a *SpikeAdapter) Allocate(req runtime.AllocateRequest) (runtime.Allocation, error) {
	if req.ClaimID == "" {
		return runtime.Allocation{}, fmt.Errorf("claim id is required")
	}
	if req.TemplateRef == "" {
		return runtime.Allocation{}, fmt.Errorf("template ref is required")
	}

	entry, pool, err := a.beginAttempt(req)
	if err != nil {
		return runtime.Allocation{}, err
	}

	manifest, err := a.claimManifest(entry.upstreamClaimName, pool)
	if err != nil {
		a.mu.Lock()
		delete(a.allocations, req.ClaimID)
		a.mu.Unlock()
		return runtime.Allocation{}, err
	}
	if err := a.kube.applyBytes(manifest); err != nil {
		// The server may have accepted the create despite the error.
		return runtime.Allocation{}, a.compensate(entry, fmt.Errorf("apply sandbox claim %q: %w", entry.upstreamClaimName, err))
	}
	a.mu.Lock()
	entry.applyConfirmed = true
	a.mu.Unlock()

	workerID, err := a.waitForSandbox(entry.upstreamClaimName)
	if err != nil {
		return runtime.Allocation{}, a.compensate(entry, err)
	}

	a.mu.Lock()
	if err := a.reserveWorkerLocked(entry, workerID); err != nil {
		a.mu.Unlock()
		return runtime.Allocation{}, fmt.Errorf("allocation %s failed: %w", req.ClaimID, err)
	}
	entry.state = allocBound
	identity := entry.identity()
	a.mu.Unlock()

	return runtime.Allocation{
		ClaimID:  req.ClaimID,
		Identity: identity,
		Filesystem: runtime.FilesystemBoundary{
			EvidenceLevel: runtime.FilesystemEvidenceUnsupported,
		},
	}, nil
}

// beginAttempt validates the ClaimID against both allocation paths, resumes a
// pending recovery when one exists, and registers a fresh attempt record.
// Claiming a recovery is atomic with the state check so only one caller may
// recover a given ClaimID at a time.
func (a *SpikeAdapter) beginAttempt(req runtime.AllocateRequest) (*allocationEntry, poolEntry, error) {
	a.mu.Lock()
	if _, exists := a.claims[req.ClaimID]; exists {
		a.mu.Unlock()
		return nil, poolEntry{}, fmt.Errorf("claim already exists through the legacy claim path: %s", req.ClaimID)
	}
	if existing, ok := a.allocations[req.ClaimID]; ok {
		switch {
		case existing.state == allocConflicted:
			detail := existing.detail
			a.mu.Unlock()
			return nil, poolEntry{}, fmt.Errorf("allocation %s: %w: %s", req.ClaimID, errIdentityConflict, detail)
		case existing.state != allocRecoveryPending:
			a.mu.Unlock()
			return nil, poolEntry{}, fmt.Errorf("claim already allocated: %s", req.ClaimID)
		case existing.recovering:
			a.mu.Unlock()
			return nil, poolEntry{}, fmt.Errorf("allocation %s: %w", req.ClaimID, errRecoveryInProgress)
		}
		existing.recovering = true
		a.mu.Unlock()

		// Resume recovery of the earlier attempt; never re-apply first.
		err := a.recoverAttempt(existing)

		a.mu.Lock()
		existing.recovering = false
		a.mu.Unlock()
		if err != nil {
			return nil, poolEntry{}, fmt.Errorf("allocation %s retry: %w", req.ClaimID, err)
		}
		a.mu.Lock()
		if _, raced := a.allocations[req.ClaimID]; raced {
			a.mu.Unlock()
			return nil, poolEntry{}, fmt.Errorf("claim already allocated: %s", req.ClaimID)
		}
		if _, raced := a.claims[req.ClaimID]; raced {
			a.mu.Unlock()
			return nil, poolEntry{}, fmt.Errorf("claim already exists through the legacy claim path: %s", req.ClaimID)
		}
	}
	pool, err := a.poolForTemplateLocked(req.TemplateRef)
	if err != nil {
		a.mu.Unlock()
		return nil, poolEntry{}, err
	}
	entry := &allocationEntry{
		claimID:           req.ClaimID,
		upstreamClaimName: resourceName("claim", req.ClaimID),
		state:             allocPending,
	}
	a.allocations[req.ClaimID] = entry
	a.mu.Unlock()
	return entry, pool, nil
}

// Observe implements runtime.RuntimeBackend. Readiness is the upstream Ready
// condition and is Bound-level evidence only. Real query failures are
// returned as errors rather than folded into a false observation, and
// readiness is accepted only when the upstream claim still carries the worker
// this identity was issued for.
func (a *SpikeAdapter) Observe(id v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	entry, err := a.entryFor(id)
	if err != nil {
		return runtime.Observation{}, err
	}
	a.mu.Lock()
	claimName, workerID := entry.upstreamClaimName, entry.workerID
	released := entry.state == allocReleased
	detail := entry.detail
	a.mu.Unlock()

	obs := runtime.Observation{
		ClaimID:  entry.claimID,
		Identity: entry.identity(),
		Filesystem: runtime.FilesystemBoundary{
			EvidenceLevel: runtime.FilesystemEvidenceUnsupported,
		},
	}
	if released {
		obs.Released = true
		obs.Detail = detail
		return obs, nil
	}
	present, err := a.kube.exists("sandboxclaims", claimName)
	if err != nil {
		return runtime.Observation{}, fmt.Errorf("observe %s: %w", entry.claimID, err)
	}
	if !present {
		obs.Detail = "upstream sandbox claim is missing without a recorded release"
		return obs, nil
	}
	var sc upstreamSandboxClaim
	if err := a.kube.get("sandboxclaims", claimName, &sc); err != nil {
		return runtime.Observation{}, fmt.Errorf("observe %s: %w", entry.claimID, err)
	}
	// Readiness belongs to whichever worker the claim currently carries. If
	// that is not the recorded worker, the identity is stale and its evidence
	// must not be reported as this allocation's readiness.
	switch {
	case sc.Status.Sandbox == nil || sc.Status.Sandbox.Name == "":
		return runtime.Observation{}, fmt.Errorf("observe %s: %w: upstream claim %s carries no worker, recorded %s", entry.claimID, errIdentityMismatch, claimName, workerID)
	case sc.Status.Sandbox.Name != workerID:
		return runtime.Observation{}, fmt.Errorf("observe %s: %w: upstream claim %s carries worker %s, recorded %s", entry.claimID, errIdentityMismatch, claimName, sc.Status.Sandbox.Name, workerID)
	}
	obs.Ready = hasCondition(sc.Status.Conditions, conditionTypeReady, conditionStatusTrue)
	obs.Detail = "readiness is upstream Ready condition for the recorded worker; warm-pool replacement is not observable"
	return obs, nil
}

// Start implements runtime.RuntimeBackend. The upstream controller starts the
// pod on its own and exposes only readiness; the adapter has no mechanism to
// establish or acknowledge actual work start, so it reports the gap.
func (a *SpikeAdapter) Start(id v1alpha1.SandboxClaimBackendIdentity) error {
	entry, err := a.entryFor(id)
	if err != nil {
		return err
	}
	if a.isReleased(entry) {
		return fmt.Errorf("start %s: %w", entry.claimID, runtime.ErrReleased)
	}
	return fmt.Errorf("start %s: %w: upstream readiness is not work start and no worker start channel exists", entry.claimID, runtime.ErrUnsupported)
}

// Terminate implements runtime.RuntimeBackend. Deleting the claim releases
// resources but yields no evidence of worker stop distinct from deletion, so
// termination is reported as unsupported rather than inferred.
func (a *SpikeAdapter) Terminate(id v1alpha1.SandboxClaimBackendIdentity) error {
	entry, err := a.entryFor(id)
	if err != nil {
		return err
	}
	if a.isReleased(entry) {
		return fmt.Errorf("terminate %s: %w", entry.claimID, runtime.ErrReleased)
	}
	return fmt.Errorf("terminate %s: %w: no worker-stop evidence channel distinct from resource deletion", entry.claimID, runtime.ErrUnsupported)
}

// Cleanup implements runtime.RuntimeBackend. Released is reported only after
// both the upstream claim and the assigned sandbox are confirmed absent.
// Replacement by the upstream warm pool is not observable and stays false.
func (a *SpikeAdapter) Cleanup(id v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	entry, err := a.entryFor(id)
	if err != nil {
		return runtime.CleanupResult{}, err
	}
	result := runtime.CleanupResult{Identity: entry.identity()}
	if a.isReleased(entry) {
		result.Released = true
		return result, nil
	}
	// A stale identity must not delete a claim now bound to another worker:
	// shutdownPolicy Delete could destroy that worker even though our later
	// release check still uses the original worker. Recheck before deletion.
	present, err := a.kube.exists("sandboxclaims", entry.upstreamClaimName)
	if err != nil {
		return result, fmt.Errorf("cleanup %s: check claim: %w", entry.claimID, err)
	}
	if present {
		var sc upstreamSandboxClaim
		if err := a.kube.get("sandboxclaims", entry.upstreamClaimName, &sc); err != nil {
			return result, fmt.Errorf("cleanup %s: read claim binding: %w", entry.claimID, err)
		}
		if sc.Status.Sandbox == nil || sc.Status.Sandbox.Name != id.WorkerID {
			return result, fmt.Errorf("cleanup %s: %w: refusing to delete claim %s without its recorded worker %s", entry.claimID, errIdentityMismatch, entry.upstreamClaimName, id.WorkerID)
		}
		if err := a.kube.delete("sandboxclaims", entry.upstreamClaimName); err != nil {
			return result, fmt.Errorf("cleanup %s: delete claim: %w", entry.claimID, err)
		}
	}
	// entry is allocBound here, so its worker was observed and its release is
	// confirmable.
	if err := a.confirmReleased(entry); err != nil {
		return result, fmt.Errorf("cleanup %s: %w", entry.claimID, err)
	}
	a.mu.Lock()
	entry.state = allocReleased
	entry.detail = "upstream claim and sandbox confirmed absent"
	a.mu.Unlock()
	result.Released = true
	return result, nil
}

// --- compensation and confirmation ---

// compensate undoes a failed allocation attempt and always returns an error
// that names the cause and whether the compensation was confirmed.
func (a *SpikeAdapter) compensate(entry *allocationEntry, cause error) error {
	if err := a.recoverAttempt(entry); err != nil {
		return fmt.Errorf("allocation %s failed: %v; %w", entry.claimID, cause, err)
	}
	return fmt.Errorf("allocation %s failed and was compensated: %w", entry.claimID, cause)
}

// recoverAttempt removes the upstream claim of a failed attempt and confirms
// that the claim and its assigned worker are gone. On confirmed success the
// record is dropped and nil is returned; otherwise the record is kept in
// recovery-pending state and the returned error names the unconfirmed step.
// Absence is never inferred from a failed query, and an unobserved worker is
// never treated as no worker.
func (a *SpikeAdapter) recoverAttempt(entry *allocationEntry) error {
	a.mu.Lock()
	claimName, workerID := entry.upstreamClaimName, entry.workerID
	conflicted := entry.state == allocConflicted
	a.mu.Unlock()

	if conflicted {
		// Never delete resources of an attempt whose worker belongs to
		// another claim.
		return fmt.Errorf("%w: refusing destructive recovery for %s", errIdentityConflict, entry.claimID)
	}

	fail := func(step string, err error) error {
		a.mu.Lock()
		entry.state = allocRecoveryPending
		entry.detail = fmt.Sprintf("%s: %v", step, err)
		a.mu.Unlock()
		return fmt.Errorf("recovery unconfirmed (%s): %w", step, err)
	}

	// Learn whether a worker was assigned before deleting anything: once the
	// claim is gone its status can no longer be read.
	present, err := a.kube.exists("sandboxclaims", claimName)
	if err != nil {
		return fail("check upstream claim", err)
	}
	if present {
		var sc upstreamSandboxClaim
		if err := a.kube.get("sandboxclaims", claimName, &sc); err != nil {
			return fail("read upstream claim for worker identity", err)
		}
		if sc.Status.Sandbox == nil || sc.Status.Sandbox.Name == "" {
			// Keep the claim so a later retry can learn its binding. Deleting
			// now could destroy a worker assigned after this empty snapshot.
			return fail("read upstream claim for worker identity", errWorkerUnknown)
		}
		if workerID != "" && workerID != sc.Status.Sandbox.Name {
			return fail("upstream binding changed during recovery", errIdentityMismatch)
		}
		workerID = sc.Status.Sandbox.Name
	}
	if workerID == "" {
		return fail("confirm worker identity", errWorkerUnknown)
	}
	// The same check protects normal binding, first compensation, and retries
	// with a previously observed worker. Keep the reservation through failed
	// deletes and confirmation, so another allocation cannot claim the worker
	// while this attempt is still removing it.
	a.mu.Lock()
	err = a.reserveWorkerLocked(entry, workerID)
	a.mu.Unlock()
	if err != nil {
		// Preserve allocConflicted; fail would turn it back into retryable state.
		return fmt.Errorf("recovery refused for %s: %w", entry.claimID, err)
	}
	if present {
		if err := a.kube.delete("sandboxclaims", claimName); err != nil {
			return fail("delete upstream claim", err)
		}
	}
	if err := a.confirmReleased(entry); err != nil {
		return fail("confirm release", err)
	}

	a.mu.Lock()
	delete(a.allocations, entry.claimID)
	if entry.workerID != "" && a.byWorker[entry.workerID] == entry.claimID {
		delete(a.byWorker, entry.workerID)
	}
	a.mu.Unlock()
	return nil
}

// confirmReleased polls until the upstream claim and the recorded worker are
// both confirmed absent. Query failures and timeouts are errors; absence is
// never assumed.
//
// An attempt whose worker was never observed cannot be confirmed at all: the
// upstream API used here reads a worker only through the claim's status, so
// once the claim is gone there is no way to prove that no worker was left
// behind. Those attempts fail with errWorkerUnknown and keep their record.
// Enumerating workers by owner reference would make this provable and is a
// promotion requirement recorded in docs/backends/agent-sandbox.md.
func (a *SpikeAdapter) confirmReleased(entry *allocationEntry) error {
	a.mu.Lock()
	claimName, workerID := entry.upstreamClaimName, entry.workerID
	a.mu.Unlock()

	deadline := time.Now().Add(a.cleanupTimeout)
	for {
		claimPresent, err := a.kube.exists("sandboxclaims", claimName)
		if err != nil {
			return err
		}
		if workerID == "" {
			// Report the claim state we do have, then refuse to call this a
			// release. Reached only from recovery: a bound allocation always
			// carries a worker.
			return fmt.Errorf("%w (upstream claim present=%t)", errWorkerUnknown, claimPresent)
		}
		sandboxPresent, err := a.kube.exists("sandboxes", workerID)
		if err != nil {
			return err
		}
		if !claimPresent && !sandboxPresent {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("%w after %s: claim present=%t sandbox present=%t", errCleanupUnconfirmed, a.cleanupTimeout, claimPresent, sandboxPresent)
		}
		time.Sleep(a.pollInterval)
	}
}

// waitForSandbox polls the upstream claim until the controller assigns a
// sandbox name or the bind budget expires.
func (a *SpikeAdapter) waitForSandbox(claimName string) (string, error) {
	deadline := time.Now().Add(a.bindTimeout)
	for {
		var sc upstreamSandboxClaim
		if err := a.kube.get("sandboxclaims", claimName, &sc); err != nil {
			return "", fmt.Errorf("get claim status: %w", err)
		}
		if sc.Status.Sandbox != nil && sc.Status.Sandbox.Name != "" {
			return sc.Status.Sandbox.Name, nil
		}
		if !time.Now().Before(deadline) {
			return "", fmt.Errorf("bind timeout after %s: claim %s not assigned a sandbox", a.bindTimeout, claimName)
		}
		time.Sleep(a.pollInterval)
	}
}

// --- helpers ---

// reserveWorkerLocked checks and reserves a known worker for either a bound
// allocation or an attempt being recovered. Caller holds a.mu. Reservations
// for failed attempts do not expose a usable identity through entryFor.
func (a *SpikeAdapter) reserveWorkerLocked(entry *allocationEntry, workerID string) error {
	if other, taken := a.byWorker[workerID]; taken && other != entry.claimID {
		entry.state = allocConflicted
		entry.conflictWorkerID = workerID
		entry.detail = fmt.Sprintf("upstream worker %s is already associated with claim %s; upstream claim %s was left in place to protect it", workerID, other, entry.upstreamClaimName)
		return fmt.Errorf("%w: %s", errIdentityConflict, entry.detail)
	}
	entry.workerID = workerID
	a.byWorker[workerID] = entry.claimID
	return nil
}

func (a *SpikeAdapter) entryFor(id v1alpha1.SandboxClaimBackendIdentity) (*allocationEntry, error) {
	if id.Backend != BackendName || id.WorkerID == "" {
		return nil, fmt.Errorf("%w: %s/%s", runtime.ErrUnknownIdentity, id.Backend, id.WorkerID)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	claimID, ok := a.byWorker[id.WorkerID]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", runtime.ErrUnknownIdentity, id.Backend, id.WorkerID)
	}
	entry, ok := a.allocations[claimID]
	if !ok || entry == nil || entry.workerID != id.WorkerID || (entry.state != allocBound && entry.state != allocReleased) {
		return nil, fmt.Errorf("%w: %s/%s", runtime.ErrUnknownIdentity, id.Backend, id.WorkerID)
	}
	return entry, nil
}

func (a *SpikeAdapter) isReleased(entry *allocationEntry) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return entry.state == allocReleased
}

// poolForTemplateLocked selects the single registered pool for a template.
// Caller holds a.mu.
func (a *SpikeAdapter) poolForTemplateLocked(templateRef string) (poolEntry, error) {
	upstreamTemplate, ok := a.poolRefs[templateRef]
	if !ok {
		return poolEntry{}, fmt.Errorf("template not found: %s (call AddTemplate first)", templateRef)
	}
	names := make([]string, 0, len(a.pools))
	for name, p := range a.pools {
		if p.upstreamTemplateName == upstreamTemplate {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	switch len(names) {
	case 0:
		return poolEntry{}, fmt.Errorf("no pool registered for template %s", templateRef)
	case 1:
		return a.pools[names[0]], nil
	default:
		return poolEntry{}, fmt.Errorf("ambiguous pool selection for template %s: %v", templateRef, names)
	}
}
