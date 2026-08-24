// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"errors"
	"fmt"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/sandbox"
)

type Runtime struct {
	templates map[string]v1alpha1.AgentSandboxTemplate
	pools     map[string]*sandbox.WarmPool
	claims    map[string]v1alpha1.SandboxClaim
}

func NewRuntime() *Runtime {
	return &Runtime{
		templates: make(map[string]v1alpha1.AgentSandboxTemplate),
		pools:     make(map[string]*sandbox.WarmPool),
		claims:    make(map[string]v1alpha1.SandboxClaim),
	}
}

func (r *Runtime) AddTemplate(template v1alpha1.AgentSandboxTemplate) error {
	if template.Metadata.Name == "" {
		return errors.New("template name is required")
	}
	if template.Spec.Image == "" {
		return errors.New("template image is required")
	}
	r.templates[template.Metadata.Name] = template
	return nil
}

func (r *Runtime) AddWarmPool(pool v1alpha1.SandboxWarmPool) error {
	if _, ok := r.templates[pool.Spec.TemplateRef]; !ok {
		return fmt.Errorf("template not found: %s", pool.Spec.TemplateRef)
	}
	warmPool, err := sandbox.NewWarmPool(pool)
	if err != nil {
		return err
	}
	r.pools[pool.Metadata.Name] = warmPool
	return nil
}

func (r *Runtime) AddClaim(claim v1alpha1.SandboxClaim) error {
	if claim.Metadata.Name == "" {
		return errors.New("claim name is required")
	}
	if claim.Spec.PoolRef == "" {
		return errors.New("claim pool ref is required")
	}
	if _, exists := r.claims[claim.Metadata.Name]; exists {
		return fmt.Errorf("claim already exists: %s", claim.Metadata.Name)
	}
	if claim.Status.Phase == "" {
		claim.Status.Phase = v1alpha1.ClaimPhasePending
	}
	if claim.Status.Phase != v1alpha1.ClaimPhasePending {
		return fmt.Errorf("new claim must start pending, got %s", claim.Status.Phase)
	}
	r.claims[claim.Metadata.Name] = claim
	return nil
}

func (r *Runtime) BindClaim(name string) error {
	claim, err := r.claimForTransition(name, v1alpha1.ClaimPhasePending)
	if err != nil {
		return err
	}
	pool, ok := r.pools[claim.Spec.PoolRef]
	if !ok {
		return fmt.Errorf("warm pool not found: %s", claim.Spec.PoolRef)
	}

	claimedSandbox, err := pool.ClaimIdle(claim.Metadata.Name)
	if err != nil {
		return err
	}
	claim.Status.Phase = v1alpha1.ClaimPhaseBound
	claim.Status.SandboxID = claimedSandbox.ID
	r.claims[name] = claim
	return nil
}

func (r *Runtime) StartClaim(name string) error {
	claim, err := r.claimForTransition(name, v1alpha1.ClaimPhaseBound)
	if err != nil {
		return err
	}
	pool := r.pools[claim.Spec.PoolRef]
	if err := pool.MarkRunning(claim.Status.SandboxID, claim.Metadata.Name); err != nil {
		return err
	}
	claim.Status.Phase = v1alpha1.ClaimPhaseRunning
	r.claims[name] = claim
	return nil
}

// SucceedClaim moves a running claim to its terminal Succeeded phase. The
// bound sandbox is destroyed and replaced automatically: sandbox cleanup is a
// resource-side fact recorded on the claim status, not a claim phase.
func (r *Runtime) SucceedClaim(name string) error {
	claim, err := r.claimForTransition(name, v1alpha1.ClaimPhaseRunning)
	if err != nil {
		return err
	}
	pool := r.pools[claim.Spec.PoolRef]
	if err := pool.MarkSucceeded(claim.Status.SandboxID, claim.Metadata.Name); err != nil {
		return err
	}
	if _, err := pool.Replace(claim.Status.SandboxID, claim.Metadata.Name); err != nil {
		return err
	}
	claim.Status.Phase = v1alpha1.ClaimPhaseSucceeded
	claim.Status.SandboxReplaced = true
	r.claims[name] = claim
	return nil
}

// FailClaim moves a bound or running claim to its terminal Failed phase.
// Bound is a legal source because a sandbox can be lost or unusable between
// bind and start. Like SucceedClaim, the bound sandbox is destroyed and
// replaced automatically so both terminal paths are symmetric.
func (r *Runtime) FailClaim(name string, summary string) error {
	claim, err := r.claimForTransition(name, v1alpha1.ClaimPhaseBound, v1alpha1.ClaimPhaseRunning)
	if err != nil {
		return err
	}
	pool := r.pools[claim.Spec.PoolRef]
	if err := pool.MarkFailed(claim.Status.SandboxID, claim.Metadata.Name); err != nil {
		return err
	}
	if _, err := pool.Replace(claim.Status.SandboxID, claim.Metadata.Name); err != nil {
		return err
	}
	claim.Status.Phase = v1alpha1.ClaimPhaseFailed
	claim.Status.Error = summary
	claim.Status.SandboxReplaced = true
	r.claims[name] = claim
	return nil
}

func (r *Runtime) ExpireClaim(name string, summary string) error {
	claim, err := r.claimForTransition(name, v1alpha1.ClaimPhasePending)
	if err != nil {
		return err
	}
	claim.Status.Phase = v1alpha1.ClaimPhaseExpired
	claim.Status.Error = summary
	r.claims[name] = claim
	return nil
}

func (r *Runtime) Claim(name string) (v1alpha1.SandboxClaim, bool) {
	claim, ok := r.claims[name]
	return claim, ok
}

func (r *Runtime) PoolStatus(name string) (v1alpha1.SandboxWarmPoolStatus, bool) {
	pool, ok := r.pools[name]
	if !ok {
		return v1alpha1.SandboxWarmPoolStatus{}, false
	}
	return pool.Status(), true
}

func (r *Runtime) PoolSandboxes(name string) ([]sandbox.Sandbox, bool) {
	pool, ok := r.pools[name]
	if !ok {
		return nil, false
	}
	return pool.Sandboxes(), true
}

func (r *Runtime) claimForTransition(name string, from ...v1alpha1.ClaimPhase) (v1alpha1.SandboxClaim, error) {
	claim, ok := r.claims[name]
	if !ok {
		return v1alpha1.SandboxClaim{}, fmt.Errorf("claim not found: %s", name)
	}
	for _, phase := range from {
		if claim.Status.Phase == phase {
			return claim, nil
		}
	}
	return v1alpha1.SandboxClaim{}, fmt.Errorf("invalid claim transition from %s; expected one of %v", claim.Status.Phase, from)
}
