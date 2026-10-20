// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"errors"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// These operations are reference state-machine evidence, not process/network
// isolation. The memory backend never claims real socket enforcement.
func (r *Runtime) ActivateNetwork(id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	a, err := r.allocationFor(id)
	if err != nil {
		return runtime.NetworkEvidence{}, err
	}
	if !a.started || a.terminated || a.released {
		return runtime.NetworkEvidence{}, runtime.ErrTerminated
	}
	if a.network.ExpiresAt.IsZero() {
		return runtime.NetworkEvidence{}, errors.New("network authority expiry is missing")
	}
	a.networkActive = true
	return memoryNetworkEvidence(a, "Active"), nil
}

func (r *Runtime) RevokeNetwork(id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	a, err := r.allocationFor(id)
	if err != nil {
		return runtime.NetworkEvidence{}, err
	}
	a.networkActive = false
	if !a.released && !a.terminated {
		if err := r.terminateWorker(a); err != nil {
			return runtime.NetworkEvidence{}, err
		}
	}
	return memoryNetworkEvidence(a, "Revoked"), nil
}

func memoryNetworkEvidence(a *allocation, state string) runtime.NetworkEvidence {
	return runtime.NetworkEvidence{ClaimID: a.claimID, Identity: a.identity(), Profiles: append([]string(nil), a.network.Profiles...), ExpiresAt: a.network.ExpiresAt, State: state, EvidenceLevel: "Simulated"}
}
