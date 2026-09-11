// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"errors"
	"fmt"
	"sort"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/sandbox"
)

// BackendName is the identity Backend value reported by the reference runtime.
const BackendName = "memory"

// workerPool is the resource operation seam for an allocated worker. Keeping
// it with the allocation also pins operations to the pool that issued it.
type workerPool interface {
	MarkRunning(workerID, claimID string) error
	MarkFailed(workerID, claimID string) error
	Replace(workerID, claimID string) (sandbox.Sandbox, error)
}

// allocation is the reference runtime's resource bookkeeping for one reduced
// contract allocation. It never records application phase or authority.
type allocation struct {
	claimID    string
	poolName   string
	pool       workerPool
	workerID   string
	ready      bool
	started    bool
	terminated bool
	released   bool
	replaced   bool
	detail     string
	filesystem runtime.FilesystemBoundary
	taskFiles  map[string][]byte
}

func (a *allocation) identity() v1alpha1.SandboxClaimBackendIdentity {
	return v1alpha1.SandboxClaimBackendIdentity{Backend: BackendName, WorkerID: a.workerID}
}

// Allocate implements runtime.RuntimeBackend. The pool is chosen internally:
// exactly one registered pool must reference req.TemplateRef.
func (r *Runtime) Allocate(req runtime.AllocateRequest) (runtime.Allocation, error) {
	if req.ClaimID == "" {
		return runtime.Allocation{}, errors.New("claim id is required")
	}
	if req.TemplateRef == "" {
		return runtime.Allocation{}, errors.New("template ref is required")
	}
	if _, exists := r.allocations[req.ClaimID]; exists {
		return runtime.Allocation{}, fmt.Errorf("claim already allocated: %s", req.ClaimID)
	}
	if _, exists := r.claims[req.ClaimID]; exists {
		return runtime.Allocation{}, fmt.Errorf("claim already exists through the reference phase path: %s", req.ClaimID)
	}
	if _, ok := r.templates[req.TemplateRef]; !ok {
		return runtime.Allocation{}, fmt.Errorf("template not found: %s", req.TemplateRef)
	}
	poolName, pool, err := r.poolForTemplate(req.TemplateRef)
	if err != nil {
		return runtime.Allocation{}, err
	}

	claimed, err := pool.ClaimIdle(req.ClaimID)
	if err != nil {
		return runtime.Allocation{}, fmt.Errorf("allocate %s: %w", req.ClaimID, err)
	}
	if other, taken := r.byWorker[claimed.ID]; taken {
		// Never overwrite a historical association; the slot is left as
		// claimed by the pool so the conflict is visible rather than hidden.
		return runtime.Allocation{}, fmt.Errorf("worker %s already associated with claim %s", claimed.ID, other)
	}

	_, held := r.readinessHold[req.ClaimID]
	delete(r.readinessHold, req.ClaimID)

	a := &allocation{
		claimID:  req.ClaimID,
		poolName: poolName,
		pool:     pool,
		workerID: claimed.ID,
		ready:    !held,
		filesystem: runtime.FilesystemBoundary{
			WorkingDirectory: "/workspace",
			OutsideBoundary:  runtime.FilesystemOutsideRuntimeReadOnlyOtherUnavailable,
			Ephemeral:        true,
			EvidenceLevel:    runtime.FilesystemEvidenceSimulated,
		},
		taskFiles: make(map[string][]byte),
	}
	if held {
		a.detail = "readiness held by control hook"
	}
	r.allocations[req.ClaimID] = a
	r.byWorker[claimed.ID] = req.ClaimID
	return runtime.Allocation{ClaimID: req.ClaimID, Identity: a.identity(), Filesystem: a.filesystem}, nil
}

// Observe implements runtime.RuntimeBackend. It is a pure read.
func (r *Runtime) Observe(id v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	a, err := r.allocationFor(id)
	if err != nil {
		return runtime.Observation{}, err
	}
	return runtime.Observation{
		ClaimID:    a.claimID,
		Identity:   a.identity(),
		Filesystem: a.filesystem,
		Ready:      a.ready && !a.released,
		Released:   a.released,
		Replaced:   a.replaced,
		Detail:     a.detail,
	}, nil
}

// Start implements runtime.RuntimeBackend. Readiness never implies start;
// only this call marks the worker running.
func (r *Runtime) Start(id v1alpha1.SandboxClaimBackendIdentity) error {
	a, err := r.allocationFor(id)
	if err != nil {
		return err
	}
	switch {
	case a.released:
		return fmt.Errorf("start %s: %w", a.claimID, runtime.ErrReleased)
	case a.terminated:
		return fmt.Errorf("start %s: %w", a.claimID, runtime.ErrTerminated)
	case a.started:
		return fmt.Errorf("start %s: %w", a.claimID, runtime.ErrAlreadyStarted)
	case !a.ready:
		return fmt.Errorf("start %s: %w", a.claimID, runtime.ErrNotReady)
	}
	if err := a.pool.MarkRunning(a.workerID, a.claimID); err != nil {
		return fmt.Errorf("start %s: %w", a.claimID, err)
	}
	a.started = true
	return nil
}

// Terminate implements runtime.RuntimeBackend. It is idempotent and accepts
// a ready-but-not-started allocation as a cancellation.
func (r *Runtime) Terminate(id v1alpha1.SandboxClaimBackendIdentity) error {
	a, err := r.allocationFor(id)
	if err != nil {
		return err
	}
	if a.released {
		return fmt.Errorf("terminate %s: %w", a.claimID, runtime.ErrReleased)
	}
	if a.terminated {
		return nil
	}
	return r.terminateWorker(a)
}

// terminateWorker marks the worker slot terminated in the pool. The pool's
// Failed phase is reused purely as the private "resource terminated" marker
// because its Bound/Running precondition matches not-started/started workers.
// It is NOT the application Failed outcome: FailClaim is never called and no
// BackendClaim phase changes here.
func (r *Runtime) terminateWorker(a *allocation) error {
	if err := a.pool.MarkFailed(a.workerID, a.claimID); err != nil {
		return fmt.Errorf("terminate %s: %w", a.claimID, err)
	}
	a.terminated = true
	return nil
}

// Cleanup implements runtime.RuntimeBackend. It terminates the worker first
// when necessary, then replaces the pool slot. Failure leaves the terminated
// flag intact so a retry does not repeat the termination transition.
func (r *Runtime) Cleanup(id v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	a, err := r.allocationFor(id)
	if err != nil {
		return runtime.CleanupResult{}, err
	}
	result := runtime.CleanupResult{Identity: a.identity()}
	if a.released {
		result.Released = true
		result.Replaced = a.replaced
		return result, nil
	}
	if !a.terminated {
		if err := r.terminateWorker(a); err != nil {
			return result, fmt.Errorf("cleanup %s: %w", a.claimID, err)
		}
	}
	if _, err := a.pool.Replace(a.workerID, a.claimID); err != nil {
		return result, fmt.Errorf("cleanup %s: %w", a.claimID, err)
	}
	a.released = true
	a.replaced = true
	a.taskFiles = nil
	result.Released = true
	result.Replaced = true
	return result, nil
}

// HoldReadiness makes the next Allocate for claimID report Ready=false until
// MarkReady is called. It exists so the reusable contract suite can prove
// not-ready behavior against the reference; it is a control hook on the
// concrete type and deliberately absent from RuntimeBackend.
func (r *Runtime) HoldReadiness(claimID string) {
	r.readinessHold[claimID] = struct{}{}
}

// MarkReady flips a held allocation to ready. Control hook; see HoldReadiness.
func (r *Runtime) MarkReady(id v1alpha1.SandboxClaimBackendIdentity) error {
	a, err := r.allocationFor(id)
	if err != nil {
		return err
	}
	if a.released {
		return fmt.Errorf("mark ready %s: %w", a.claimID, runtime.ErrReleased)
	}
	a.ready = true
	a.detail = ""
	return nil
}

// Started reports whether Start has been acknowledged for id. It is a test
// probe on the concrete type, not backend contract evidence.
func (r *Runtime) Started(id v1alpha1.SandboxClaimBackendIdentity) bool {
	a, err := r.allocationFor(id)
	return err == nil && a.started
}

func (r *Runtime) allocationFor(id v1alpha1.SandboxClaimBackendIdentity) (*allocation, error) {
	if id.Backend != BackendName || id.WorkerID == "" {
		return nil, fmt.Errorf("%w: %s/%s", runtime.ErrUnknownIdentity, id.Backend, id.WorkerID)
	}
	claimID, ok := r.byWorker[id.WorkerID]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", runtime.ErrUnknownIdentity, id.Backend, id.WorkerID)
	}
	return r.allocations[claimID], nil
}

// poolForTemplate selects the single registered pool for a template. Pool
// names are visited in sorted order so any error is deterministic; supporting
// several pools per template is not part of this contract.
func (r *Runtime) poolForTemplate(templateRef string) (string, *sandbox.WarmPool, error) {
	names := make([]string, 0, len(r.pools))
	for name := range r.pools {
		names = append(names, name)
	}
	sort.Strings(names)
	var matches []string
	for _, name := range names {
		if r.pools[name].TemplateRef == templateRef {
			matches = append(matches, name)
		}
	}
	switch len(matches) {
	case 0:
		return "", nil, fmt.Errorf("no pool registered for template %s", templateRef)
	case 1:
		return matches[0], r.pools[matches[0]], nil
	default:
		return "", nil, fmt.Errorf("ambiguous pool selection for template %s: %v", templateRef, matches)
	}
}
