// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gatewaytest

import "github.com/wunderforge/agenova/api/v1alpha1"

// Claims is a mutable authoritative claim view for gateway tests.
type Claims struct {
	Items map[string]v1alpha1.SandboxClaim
}

func NewClaims() *Claims {
	return &Claims{Items: make(map[string]v1alpha1.SandboxClaim)}
}

func (c *Claims) Claim(claimID string) (v1alpha1.SandboxClaim, bool) {
	if c == nil {
		return v1alpha1.SandboxClaim{}, false
	}
	claim, ok := c.Items[claimID]
	return claim, ok
}

func (c *Claims) Put(claimID string, phase v1alpha1.ClaimPhase) {
	c.Items[claimID] = ClaimSnapshot(claimID, phase)
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
