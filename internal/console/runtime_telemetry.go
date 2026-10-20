// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
	"sync"
)

type observedRuntime struct {
	runtime.RuntimeBackend
	s      *Service
	mu     sync.Mutex
	claims map[v0.SandboxClaimBackendIdentity]string
}

func (r *observedRuntime) start(claim, op string) func(bool) {
	r.s.mu.RLock()
	ctx := context.Background()
	ref := ""
	for key, record := range r.s.records {
		if record.view.State != nil && record.view.State.Claim != nil && record.view.State.Claim.ID == claim {
			ref = key
			if record.ctx != nil {
				ctx = record.ctx
			}
			break
		}
	}
	r.s.mu.RUnlock()
	_, end := r.s.telemetry.Start(ctx, op, ref, claim, "")
	return end
}
func (r *observedRuntime) claim(id v0.SandboxClaimBackendIdentity) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.claims[id]
}
func (r *observedRuntime) Allocate(req runtime.AllocateRequest) (runtime.Allocation, error) {
	end := r.start(req.ClaimID, "runtime.allocate")
	result, err := r.RuntimeBackend.Allocate(req)
	end(err == nil)
	if err == nil {
		r.mu.Lock()
		r.claims[result.Identity] = req.ClaimID
		r.mu.Unlock()
	}
	return result, err
}
func (r *observedRuntime) Observe(id v0.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	end := r.start(r.claim(id), "runtime.observe")
	v, err := r.RuntimeBackend.Observe(id)
	end(err == nil)
	return v, err
}
func (r *observedRuntime) Start(id v0.SandboxClaimBackendIdentity) error {
	end := r.start(r.claim(id), "runtime.start")
	err := r.RuntimeBackend.Start(id)
	end(err == nil)
	return err
}
func (r *observedRuntime) Terminate(id v0.SandboxClaimBackendIdentity) error {
	end := r.start(r.claim(id), "runtime.terminate")
	err := r.RuntimeBackend.Terminate(id)
	end(err == nil)
	return err
}
func (r *observedRuntime) Cleanup(id v0.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	end := r.start(r.claim(id), "runtime.cleanup")
	v, err := r.RuntimeBackend.Cleanup(id)
	end(err == nil)
	return v, err
}

type observedNetworkRuntime struct {
	*observedRuntime
	network runtime.NetworkBackend
}

func (r *observedNetworkRuntime) ActivateNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	end := r.start(r.claim(id), "runtime.network.activate")
	v, err := r.network.ActivateNetwork(id)
	end(err == nil)
	if err == nil {
		r.s.networkEvidence.Store(v.ClaimID, v)
	}
	return v, err
}

func (r *observedNetworkRuntime) RevokeNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	end := r.start(r.claim(id), "runtime.network.revoke")
	v, err := r.network.RevokeNetwork(id)
	end(err == nil)
	if err == nil {
		r.s.networkEvidence.Store(v.ClaimID, v)
	}
	return v, err
}
