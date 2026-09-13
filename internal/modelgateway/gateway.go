// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelgateway

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/governance"
	"github.com/wunderforge/agenova/internal/runtime"
)

// Request is a governed model invocation request from an agent claim. It
// identifies the approved model profile, never a provider SDK shape or
// credential.
type Request struct {
	ClaimID string
	Profile string
	// Parameters is operation data for the adapter. Keys naming credential
	// material are rejected before any adapter invocation.
	Parameters map[string]string
	// CallerReference is optional caller-supplied metadata. It is never
	// adopted as the trusted invocation identity.
	CallerReference string
}

// Adapter is the provider-side seam. Only Allow decisions reach it; provider
// shapes and credentials stay behind this boundary.
type Adapter interface {
	Invoke(invocationID string, req Request) error
}

// unconfiguredAdapter is the default: a gateway with no provider adapter
// cannot honestly attempt an allowed invocation, so it reports a configuration
// failure instead of silently succeeding and leaving evidence of a call that
// never happened.
type unconfiguredAdapter struct{}

func (unconfiguredAdapter) Invoke(string, Request) error {
	return errors.New("no provider adapter configured")
}

// Policy evaluates a validated request against granted authority. E4-T1 ships
// the claim-activity baseline built into the gateway; model-profile
// enforcement layers on through this seam (E4-T3), and an approval-routing
// policy may return ApprovalRequired without granting anything.
type Policy func(req Request) gateway.Outcome

// Gateway authorizes model requests for active claims and records
// invocation facts correlated by a gateway-assigned invocation ID.
type Gateway struct {
	backend runtime.ClaimReader
	lineage *governance.Lineage
	store   *facts.Store
	adapter Adapter
	ids     gateway.IDSource
	policy  Policy
}

// Option configures a Gateway.
type Option func(*Gateway)

// WithAdapter sets the provider adapter invoked for Allow decisions. A nil
// adapter keeps the fail-closed default rather than leaving the gateway in a
// state that panics on the first allowed request.
func WithAdapter(a Adapter) Option {
	return func(g *Gateway) {
		if a != nil {
			g.adapter = a
		}
	}
}

// WithIDSource sets the trusted invocation-identity source; nil keeps the default.
func WithIDSource(ids gateway.IDSource) Option {
	return func(g *Gateway) {
		if ids != nil {
			g.ids = ids
		}
	}
}

// WithPolicy sets the policy evaluated after the claim-activity baseline; nil
// keeps the default.
func WithPolicy(p Policy) Option {
	return func(g *Gateway) {
		if p != nil {
			g.policy = p
		}
	}
}

func NewGateway(backend runtime.ClaimReader, lineage *governance.Lineage, store *facts.Store, opts ...Option) *Gateway {
	g := &Gateway{
		backend: backend,
		lineage: lineage,
		store:   store,
		adapter: unconfiguredAdapter{},
		ids:     gateway.RandomIDSource(),
		policy:  func(Request) gateway.Outcome { return gateway.Allowed() },
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Invoke decides one governed model attempt. The returned Decision carries the
// gateway-assigned InvocationID used by the decision, any attempted external
// call, its result, and recorded evidence. Once claim identity resolves, every
// decision — Allow, Deny, and ApprovalRequired alike — appends one
// claim-scoped invocation fact, so denied attempts stay inspectable; a
// rejection raised before the claim resolves returns the issued ID without
// fabricating a claim-attributed fact. Deny and ApprovalRequired never reach
// the adapter; ApprovalRequired grants nothing. The error reports an adapter
// failure only, never a governance outcome.
func (g *Gateway) Invoke(req Request) (gateway.Decision, error) {
	// The trusted correlation identity is issued at entry, ahead of structural
	// validation and policy evaluation, and never comes from the caller.
	id := g.ids()

	// Pre-resolution rejections: the asserted claim is untrusted here, so the
	// decision is returned without claim-attributed evidence.
	if out, rejected := validate(req); rejected {
		return decision(id, out), nil
	}
	if out, unresolved := g.resolveClaim(req.ClaimID); unresolved {
		return decision(id, out), nil
	}

	// The claim is resolved from here on, so every outcome is recorded.
	if out, denied := g.claimBaseline(req.ClaimID); denied {
		return g.record(req, id, out), nil
	}
	// The normalized outcome is carried through on every branch, so an Allow
	// keeps the policy's own Reason instead of being rebuilt as a bare verdict.
	out := g.policy(req).Normalize()
	if out.Result != gateway.ResultAllow {
		return g.record(req, id, out), nil
	}

	allowed := g.record(req, id, out)
	if err := g.adapter.Invoke(id, req); err != nil {
		return allowed, fmt.Errorf("model adapter invocation %s: %w", id, err)
	}
	return allowed, nil
}

func decision(id string, out gateway.Outcome) gateway.Decision {
	return gateway.Decision{
		InvocationID: id,
		Result:       out.Result,
		Category:     out.Category,
		Reason:       out.Reason,
	}
}

func (g *Gateway) record(req Request, id string, out gateway.Outcome) gateway.Decision {
	g.store.RecordModelInvocation(req.ClaimID, req.Profile, id, out.Result)
	return decision(id, out)
}

// validate rejects malformed or unsafe requests before any adapter path.
func validate(req Request) (gateway.Outcome, bool) {
	if strings.TrimSpace(req.ClaimID) == "" {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: gateway.CategoryMissingClaimIdentity,
			Reason:   "request carries no claim identity",
		}, true
	}
	if strings.TrimSpace(req.Profile) == "" {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: gateway.CategoryIncompleteOperation,
			Reason:   "model request must identify the approved profile",
		}, true
	}
	if key, found := gateway.FindReservedCredentialKey(req.Parameters); found {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: gateway.CategorySecretValue,
			Reason:   fmt.Sprintf("parameter %q is a reserved credential key; credentials stay behind gateway adapters", key),
		}, true
	}
	return gateway.Allowed(), false
}

// resolveClaim reports whether the asserted claim identity is unknown to the
// runtime backend. An unknown claim cannot own claim-attributed evidence.
func (g *Gateway) resolveClaim(claimID string) (gateway.Outcome, bool) {
	if _, ok := g.backend.Claim(claimID); !ok {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: gateway.CategoryUnknownClaim,
			Reason:   fmt.Sprintf("unknown claim: %s", claimID),
		}, true
	}
	return gateway.Allowed(), false
}

// claimBaseline enforces claim-scoped authority: only an active Running claim
// whose parent (if any) is also Running may use governed interfaces.
func (g *Gateway) claimBaseline(claimID string) (gateway.Outcome, bool) {
	if out, denied := g.requireRunning(claimID); denied {
		return out, true
	}
	if parentID, ok := g.lineage.Parent(claimID); ok {
		if _, denied := g.requireRunning(parentID); denied {
			return gateway.Outcome{
				Result:   gateway.ResultDeny,
				Category: gateway.CategoryOutOfParentScope,
				Reason:   fmt.Sprintf("child claim %q is out of parent scope: parent %q is not running", claimID, parentID),
			}, true
		}
	}
	return gateway.Allowed(), false
}

func (g *Gateway) requireRunning(claimID string) (gateway.Outcome, bool) {
	claim, ok := g.backend.Claim(claimID)
	if !ok {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: gateway.CategoryUnknownClaim,
			Reason:   fmt.Sprintf("unknown claim: %s", claimID),
		}, true
	}
	if claim.Status.Phase != v1alpha1.ClaimPhaseRunning {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: gateway.CategoryClaimNotActive,
			Reason:   fmt.Sprintf("claim %q is not running (phase: %s)", claimID, claim.Status.Phase),
		}, true
	}
	return gateway.Allowed(), false
}
