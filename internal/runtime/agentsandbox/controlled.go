// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// workerControl is an adapter-owned, opt-in worker protocol. Upstream Agent
// Sandbox v0.4.6 has no native task start/stop operation. A compatible image
// must implement /agenova-workerctl start|stop|status <claim-id> and return
// exact claim-bound acknowledgements. This is a local E2E protocol, not an
// assertion that arbitrary sandbox images or hostile workers are governed.
type workerControl interface {
	execWorker(workerID string, args ...string) (string, error)
}

type controlPhase uint8

const (
	controlPending controlPhase = iota
	controlStarting
	controlRunning
	controlUnknown
	controlStopping
	controlTerminated
	controlCleaning
	controlCleanupUnknown
	controlReleased
)

// ControlledAdapter adds a deliberate worker-control channel to SpikeAdapter.
// Use it only with an image implementing /agenova-workerctl. New() continues
// to report ErrUnsupported for Start/Terminate on ordinary images.
type ControlledAdapter struct {
	*SpikeAdapter
	control workerControl
	mu      sync.Mutex
	phases  map[string]controlPhase // keyed by the observed worker identity
}

var _ runtime.RuntimeBackend = (*ControlledAdapter)(nil)

// NewControlled configures the opt-in protocol adapter for one context and
// namespace. The worker image must be registered separately via AddTemplate.
func NewControlled(kubeContext, namespace string) *ControlledAdapter {
	kube := newKubectlRunner(kubeContext, namespace)
	return newControlledAdapter(newSpikeAdapter(kube, namespace), kube)
}

func newControlledAdapter(base *SpikeAdapter, control workerControl) *ControlledAdapter {
	return &ControlledAdapter{
		SpikeAdapter: base,
		control:      control,
		phases:       make(map[string]controlPhase),
	}
}

// Start requires a Ready observation of the same bound worker, then an exact
// claim-bound acknowledgement from the worker process. Failed/ambiguous
// transport leaves the phase unknown: a retry could start work twice.
func (a *ControlledAdapter) Start(id v1alpha1.SandboxClaimBackendIdentity) error {
	entry, err := a.entryFor(id)
	if err != nil {
		return err
	}
	obs, err := a.Observe(id)
	if err != nil {
		return fmt.Errorf("start %s: verify binding: %w", entry.claimID, err)
	}
	if obs.Released {
		return fmt.Errorf("start %s: %w", entry.claimID, runtime.ErrReleased)
	}
	if !obs.Ready {
		return fmt.Errorf("start %s: %w", entry.claimID, runtime.ErrNotReady)
	}
	a.mu.Lock()
	switch a.phases[id.WorkerID] {
	case controlStarting, controlRunning, controlUnknown, controlStopping:
		a.mu.Unlock()
		return fmt.Errorf("start %s: %w", entry.claimID, runtime.ErrAlreadyStarted)
	case controlTerminated:
		a.mu.Unlock()
		return fmt.Errorf("start %s: %w", entry.claimID, runtime.ErrTerminated)
	case controlCleaning, controlCleanupUnknown, controlReleased:
		a.mu.Unlock()
		return fmt.Errorf("start %s: %w: cleanup in progress or complete", entry.claimID, runtime.ErrReleased)
	}
	a.phases[id.WorkerID] = controlStarting
	a.mu.Unlock()

	out, err := a.control.execWorker(id.WorkerID, "start", entry.claimID)
	if err == nil && !workerAck(out, "started", entry.claimID) {
		err = fmt.Errorf("unexpected worker acknowledgement %q", strings.TrimSpace(out))
	}
	if err == nil {
		out, err = a.control.execWorker(id.WorkerID, "status", entry.claimID)
		if err == nil && !workerAck(out, "running", entry.claimID) {
			err = fmt.Errorf("unexpected worker status %q", strings.TrimSpace(out))
		}
	}
	a.mu.Lock()
	if err == nil {
		a.phases[id.WorkerID] = controlRunning
	} else {
		a.phases[id.WorkerID] = controlUnknown
	}
	a.mu.Unlock()
	if err != nil {
		return fmt.Errorf("start %s: acknowledgement unconfirmed: %w", entry.claimID, err)
	}
	return nil
}

// Terminate confirms a started worker's task has stopped through the worker
// protocol. Before Start it cancels the still-local start capability; no work
// existed to stop. It never infers stop from Pod deletion or Ready changes.
func (a *ControlledAdapter) Terminate(id v1alpha1.SandboxClaimBackendIdentity) error {
	entry, err := a.entryFor(id)
	if err != nil {
		return err
	}
	obs, err := a.Observe(id)
	if err != nil {
		return fmt.Errorf("terminate %s: verify binding: %w", entry.claimID, err)
	}
	if obs.Released {
		return fmt.Errorf("terminate %s: %w", entry.claimID, runtime.ErrReleased)
	}
	a.mu.Lock()
	phase := a.phases[id.WorkerID]
	if phase == controlTerminated {
		a.mu.Unlock()
		return nil
	}
	if phase == controlPending {
		a.phases[id.WorkerID] = controlTerminated
		a.mu.Unlock()
		return nil
	}
	if phase == controlStarting {
		a.mu.Unlock()
		return fmt.Errorf("terminate %s: %w: start still in flight", entry.claimID, runtime.ErrUnsupported)
	}
	if phase == controlStopping {
		a.mu.Unlock()
		return fmt.Errorf("terminate %s: %w: stop already in progress", entry.claimID, runtime.ErrUnsupported)
	}
	if phase == controlCleaning || phase == controlCleanupUnknown || phase == controlReleased {
		a.mu.Unlock()
		return fmt.Errorf("terminate %s: %w: cleanup in progress or complete", entry.claimID, runtime.ErrReleased)
	}
	a.phases[id.WorkerID] = controlStopping
	a.mu.Unlock()

	out, err := a.control.execWorker(id.WorkerID, "stop", entry.claimID)
	if err == nil && !workerAck(out, "stopped", entry.claimID) {
		err = fmt.Errorf("unexpected worker acknowledgement %q", strings.TrimSpace(out))
	}
	if err == nil {
		out, err = a.control.execWorker(id.WorkerID, "status", entry.claimID)
		if err == nil && !workerAck(out, "stopped", entry.claimID) {
			err = fmt.Errorf("unexpected worker status %q", strings.TrimSpace(out))
		}
	}
	if err != nil {
		a.mu.Lock()
		if a.phases[id.WorkerID] == controlStopping {
			a.phases[id.WorkerID] = controlUnknown
		}
		a.mu.Unlock()
		return fmt.Errorf("terminate %s: stop unconfirmed: %w", entry.claimID, err)
	}
	a.mu.Lock()
	if a.phases[id.WorkerID] == controlStopping {
		a.phases[id.WorkerID] = controlTerminated
	}
	a.mu.Unlock()
	return nil
}

func workerAck(output, state, claimID string) bool {
	line := strings.TrimSpace(output)
	prefix := "state=" + state + " claim=" + claimID + " result="
	if !strings.HasPrefix(line, prefix) {
		return false
	}
	result := strings.TrimPrefix(line, prefix)
	return result != "" && !strings.ContainsAny(result, " \t\r\n")
}

// Cleanup refuses to hide unconfirmed running/unknown work behind resource
// deletion. The underlying adapter still confirms both upstream objects gone.
func (a *ControlledAdapter) Cleanup(id v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	entry, err := a.entryFor(id)
	if err != nil {
		return runtime.CleanupResult{}, err
	}
	a.mu.Lock()
	phase := a.phases[id.WorkerID]
	if phase == controlStarting || phase == controlRunning || phase == controlUnknown || phase == controlStopping {
		a.mu.Unlock()
		return runtime.CleanupResult{}, fmt.Errorf("cleanup %s: %w: worker stop unconfirmed", entry.claimID, runtime.ErrUnsupported)
	}
	if phase == controlCleaning {
		a.mu.Unlock()
		return runtime.CleanupResult{}, fmt.Errorf("cleanup %s: %w: cleanup already in progress", entry.claimID, runtime.ErrUnsupported)
	}
	// An earlier delete/confirmation failure can be retried, but work must
	// never restart: the previous deletion may already have reached the API.
	a.phases[id.WorkerID] = controlCleaning
	a.mu.Unlock()
	result, err := a.SpikeAdapter.Cleanup(id)
	if err != nil {
		a.mu.Lock()
		a.phases[id.WorkerID] = controlCleanupUnknown
		a.mu.Unlock()
		return result, err
	}
	if !result.Released {
		a.mu.Lock()
		a.phases[id.WorkerID] = controlCleanupUnknown
		a.mu.Unlock()
		return result, errors.New("cleanup returned no release evidence")
	}
	a.mu.Lock()
	a.phases[id.WorkerID] = controlReleased
	a.mu.Unlock()
	return result, nil
}
