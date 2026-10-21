// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package agentsandbox

import (
	"fmt"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// RestoreForCleanup reattaches a durably recorded allocation for stop/cleanup
// ONLY. It never allocates, starts a worker, or restores application authority.
// Call during fenced startup before accepting Work. Unknown allocation attempts
// without a recorded identity require separate operator reconciliation.
func (a *ControlledAdapter) RestoreForCleanup(claimID string, id v0.SandboxClaimBackendIdentity) error {
	if claimID == "" || id.Backend != BackendName || id.WorkerID == "" {
		return runtime.ErrUnknownIdentity
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.SpikeAdapter.mu.Lock()
	_, claimExists := a.allocations[claimID]
	_, workerExists := a.byWorker[id.WorkerID]
	a.SpikeAdapter.mu.Unlock()
	if claimExists && workerExists && a.recovered[id.WorkerID] == claimID {
		return nil
	}
	if claimExists || workerExists {
		return fmt.Errorf("restore cleanup: allocation already registered")
	}
	name := resourceName("claim", claimID)
	dedicated := isDedicatedClaimName(id.WorkerID)
	if dedicated {
		if id.AllocationID == "" {
			return errIdentityMismatch
		}
		name = id.WorkerID
	}
	present, err := a.kube.exists("sandboxclaims", name)
	if err != nil {
		return fmt.Errorf("restore cleanup: read claim: %w", err)
	}
	released := false
	var pinnedClaimUID string
	var pinnedPodUID string
	if id.AllocationID != "" {
		var valid bool
		pinnedClaimUID, pinnedPodUID, valid = strings.Cut(id.AllocationID, "/")
		if !valid || pinnedClaimUID == "" || pinnedPodUID == "" || strings.Contains(pinnedPodUID, "/") {
			return errIdentityMismatch
		}
	}
	var shutdownTime string
	var attemptID string
	if present {
		var claim upstreamSandboxClaim
		if err := a.kube.get("sandboxclaims", name, &claim); err != nil {
			return fmt.Errorf("restore cleanup: read binding: %w", err)
		}
		if claim.Metadata.Name != name || claim.Status.Sandbox == nil || claim.Status.Sandbox.Name != id.WorkerID {
			return errIdentityMismatch
		}
		attemptID = claim.Metadata.Annotations["agenova.io/allocation-attempt"]
		if dedicated && (attemptID == "" || dedicatedClaimName(claimID, attemptID) != name || claim.Spec.Warmpool != "none") {
			return errIdentityMismatch
		}
		if id.AllocationID != "" {
			if claim.Metadata.UID != pinnedClaimUID || claim.Metadata.Annotations["agenova.io/claim-id"] != claimID {
				return errIdentityMismatch
			}
			// Expiry/controller cleanup may remove the Pod before its original
			// claim disappears. Confirmed absence is cleanup-only progress, not
			// a failed identity read or permission to adopt a replacement Pod.
			podPresent, err := a.kube.exists("pods", id.WorkerID)
			if err != nil {
				return fmt.Errorf("restore cleanup: check Pod absence: %w", err)
			}
			if podPresent {
				var pod struct {
					Metadata upstreamMeta `json:"metadata"`
				}
				if err := a.kube.get("pods", id.WorkerID, &pod); err != nil {
					return fmt.Errorf("restore cleanup: read Pod incarnation: %w", err)
				}
				if pod.Metadata.Name != id.WorkerID || pod.Metadata.UID != pinnedPodUID || pod.Metadata.Labels[claimUIDLabel] != pinnedClaimUID {
					return errIdentityMismatch
				}
			}
		} else if claim.Metadata.Annotations["agenova.io/allocation-attempt"] != "" || (claim.Spec.Lifecycle != nil && claim.Spec.Lifecycle.ShutdownTime != "") {
			return fmt.Errorf("restore cleanup: missing durable allocation incarnation; operator reconciliation required")
		}
		if claim.Spec.Lifecycle != nil {
			shutdownTime = claim.Spec.Lifecycle.ShutdownTime
		}
	} else {
		exists, err := a.kube.exists("sandboxes", id.WorkerID)
		if err != nil {
			return fmt.Errorf("restore cleanup: read worker: %w", err)
		}
		if exists {
			return errCleanupUnconfirmed
		}
		released = true
		if id.AllocationID != "" {
			podPresent, err := a.kube.exists("pods", id.WorkerID)
			if err != nil {
				return err
			}
			if podPresent {
				return errCleanupUnconfirmed
			}
		}
	}
	entry := &allocationEntry{claimID: claimID, upstreamClaimName: name, workerID: id.WorkerID, state: allocBound, applyConfirmed: true}
	entry.attemptID = attemptID
	entry.networkClaimUID = pinnedClaimUID
	entry.networkPodUID = pinnedPodUID
	if shutdownTime != "" {
		entry.network.ExpiresAt, _ = time.Parse(time.RFC3339Nano, shutdownTime)
	}
	if released {
		entry.state = allocReleased
	}
	a.SpikeAdapter.mu.Lock()
	// Allocate may have raced the backend reads. Never replace live ownership.
	if _, ok := a.allocations[claimID]; ok {
		a.SpikeAdapter.mu.Unlock()
		return errIdentityConflict
	}
	if _, ok := a.byWorker[id.WorkerID]; ok {
		a.SpikeAdapter.mu.Unlock()
		return errIdentityConflict
	}
	a.allocations[claimID] = entry
	a.byWorker[id.WorkerID] = claimID
	a.SpikeAdapter.mu.Unlock()
	a.recovered[id.WorkerID] = claimID
	a.phases[id.WorkerID] = controlUnknown
	if released {
		a.phases[id.WorkerID] = controlReleased
		a.releases[id.WorkerID] = runtime.CleanupResult{Identity: id, Released: true}
	}
	return nil
}
