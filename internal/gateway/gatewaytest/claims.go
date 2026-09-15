// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gatewaytest

import (
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
)

// Claims is a mutable authoritative claim view for gateway tests.
type Claims struct {
	Items       map[string]v1alpha1.SandboxClaim
	Authorities map[string]v1alpha1.EffectiveAuthority
}

func NewClaims() *Claims {
	return &Claims{
		Items:       make(map[string]v1alpha1.SandboxClaim),
		Authorities: make(map[string]v1alpha1.EffectiveAuthority),
	}
}

func (c *Claims) Claim(claimID string) (v1alpha1.SandboxClaim, bool) {
	if c == nil {
		return v1alpha1.SandboxClaim{}, false
	}
	claim, ok := c.Items[claimID]
	return claim, ok
}

func (c *Claims) ClaimAuthority(claimID string) (app.ClaimAuthoritySnapshot, bool) {
	if c == nil {
		return app.ClaimAuthoritySnapshot{}, false
	}
	claim, ok := c.Items[claimID]
	if !ok {
		return app.ClaimAuthoritySnapshot{}, false
	}
	authority := c.Authorities[claimID]
	authority.Tools = append([]string(nil), authority.Tools...)
	authority.ResourceScopes = append([]string(nil), authority.ResourceScopes...)
	authority.MemoryScopes = append([]string(nil), authority.MemoryScopes...)
	if claim.BackendIdentity != nil {
		identity := *claim.BackendIdentity
		claim.BackendIdentity = &identity
	}
	return app.ClaimAuthoritySnapshot{
		Claim:              claim,
		EffectiveAuthority: authority,
	}, true
}

func (c *Claims) Put(claimID string, phase v1alpha1.ClaimPhase) {
	c.Items[claimID] = ClaimSnapshot(claimID, phase)
	c.Authorities[claimID] = AuthoritySnapshot(claimID)
}

func ClaimSnapshot(claimID string, phase v1alpha1.ClaimPhase) v1alpha1.SandboxClaim {
	return v1alpha1.SandboxClaim{
		ID:           claimID,
		RequestRef:   "request:" + claimID,
		TemplateRef:  "engineer",
		AuthorityRef: "authority:" + claimID,
		Phase:        phase,
		BackendIdentity: &v1alpha1.SandboxClaimBackendIdentity{
			Backend:  "reference",
			WorkerID: "worker:" + claimID,
		},
	}
}

func AuthoritySnapshot(claimID string) v1alpha1.EffectiveAuthority {
	return v1alpha1.EffectiveAuthority{
		ID:             "authority:" + claimID,
		Tools:          []string{"git.read"},
		ResourceScopes: []string{"repo:acme/payments"},
		ModelProfile:   "approved-coding-model",
		Runtime: v1alpha1.EffectiveAuthorityRuntime{
			ProfileRef: "standard-isolated",
			Timeout:    v1alpha1.Duration(20 * time.Minute),
		},
	}
}
