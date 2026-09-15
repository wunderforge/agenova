// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelgateway

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/governance"
)

// Request identifies a governed model profile, never a provider model or
// credential-bearing SDK request.
type Request struct {
	ClaimID         string
	Profile         string
	Parameters      map[string]string
	CallerReference string
}

type Adapter interface {
	Invoke(invocationID string, req Request) error
}

type unconfiguredAdapter struct{}

func (unconfiguredAdapter) Invoke(string, Request) error {
	return errors.New("no provider adapter configured")
}

// Policy evaluates a request only after the authoritative Running and
// effective-authority ceilings allow it. It may further restrict a request;
// it can never widen the system-issued grant.
type Policy func(req Request) gateway.Outcome

type Gateway struct {
	claims  app.ClaimAuthorityReader
	lineage *governance.Lineage
	store   *facts.Store
	adapter Adapter
	ids     gateway.IDSource
	policy  Policy
}

type Option func(*Gateway)

func WithAdapter(adapter Adapter) Option {
	return func(g *Gateway) {
		if adapter != nil {
			g.adapter = adapter
		}
	}
}

func WithIDSource(ids gateway.IDSource) Option {
	return func(g *Gateway) {
		if ids != nil {
			g.ids = ids
		}
	}
}

func WithPolicy(policy Policy) Option {
	return func(g *Gateway) {
		if policy != nil {
			g.policy = policy
		}
	}
}

func NewGateway(claims app.ClaimAuthorityReader, lineage *governance.Lineage, store *facts.Store, opts ...Option) *Gateway {
	if store == nil {
		store = facts.NewStore()
	}
	g := &Gateway{
		claims:  claims,
		lineage: lineage,
		store:   store,
		adapter: unconfiguredAdapter{},
		ids:     gateway.RandomIDSource(),
		policy:  func(Request) gateway.Outcome { return gateway.Allowed() },
	}
	for _, option := range opts {
		option(g)
	}
	return g
}

// Invoke returns a typed governance decision. Deny and ApprovalRequired never
// invoke the provider adapter. The error channel is reserved for gateway or
// adapter execution failure, not a governance outcome.
func (g *Gateway) Invoke(req Request) (gateway.Decision, error) {
	if g == nil {
		return gateway.Decision{}, errors.New("model gateway is unavailable")
	}
	id := g.ids()
	// Detach caller-owned parameters before validation; later caller mutation
	// cannot add a credential key after the boundary check.
	req.Parameters = cloneStringMap(req.Parameters)

	if outcome, rejected := validate(req); rejected {
		return decision(id, outcome), nil
	}
	snapshot, outcome, unresolved := g.resolveClaim(req.ClaimID)
	if unresolved {
		return decision(id, outcome), nil
	}
	if outcome, denied := g.claimBaseline(req.ClaimID, snapshot.Claim); denied {
		return g.record(req, id, outcome), nil
	}
	if outcome, denied := enforceAuthority(req, snapshot); denied {
		return g.record(req, id, outcome), nil
	}
	// Policy receives its own defensive copy so policy mutation cannot alter
	// the request passed to the provider adapter after validation.
	policyReq := req
	policyReq.Parameters = cloneStringMap(req.Parameters)
	outcome = g.policy(policyReq).Normalize()
	if outcome.Result != gateway.ResultAllow {
		return g.record(req, id, outcome), nil
	}

	allowed := g.record(req, id, outcome)
	if err := g.adapter.Invoke(id, req); err != nil {
		return allowed, fmt.Errorf("model adapter invocation %s: %w", id, err)
	}
	return allowed, nil
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func decision(id string, outcome gateway.Outcome) gateway.Decision {
	return gateway.Decision{InvocationID: id, Result: outcome.Result, Category: outcome.Category, Reason: outcome.Reason}
}

func (g *Gateway) record(req Request, id string, outcome gateway.Outcome) gateway.Decision {
	g.store.RecordModelInvocation(req.ClaimID, req.Profile, id, outcome.Result)
	return decision(id, outcome)
}

func validate(req Request) (gateway.Outcome, bool) {
	if strings.TrimSpace(req.ClaimID) == "" {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryMissingClaimIdentity, Reason: "request carries no claim identity"}, true
	}
	if strings.TrimSpace(req.Profile) == "" {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryIncompleteOperation, Reason: "model request must identify the approved profile"}, true
	}
	if key, found := gateway.FindReservedCredentialKey(req.Parameters); found {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategorySecretValue, Reason: fmt.Sprintf("parameter %q is a reserved credential key; credentials stay behind gateway adapters", key)}, true
	}
	return gateway.Allowed(), false
}

func (g *Gateway) resolveClaim(claimID string) (app.ClaimAuthoritySnapshot, gateway.Outcome, bool) {
	if g.claims == nil {
		return app.ClaimAuthoritySnapshot{}, gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryGatewayUnavailable, Reason: "authoritative claim and authority reader is unavailable"}, true
	}
	snapshot, ok := g.claims.ClaimAuthority(claimID)
	if !ok {
		return app.ClaimAuthoritySnapshot{}, gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryUnknownClaim, Reason: fmt.Sprintf("unknown claim: %s", claimID)}, true
	}
	return snapshot, gateway.Allowed(), false
}

func (g *Gateway) claimBaseline(claimID string, claim v1alpha1.SandboxClaim) (gateway.Outcome, bool) {
	if err := app.RequireRunningClaimSnapshot(claim, claimID); err != nil {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryClaimNotActive, Reason: err.Error()}, true
	}
	if g.lineage != nil {
		if parentID, ok := g.lineage.Parent(claimID); ok {
			if _, denied := g.requireRunning(parentID); denied {
				return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryOutOfParentScope, Reason: fmt.Sprintf("child claim %q is out of parent scope: parent %q is not running", claimID, parentID)}, true
			}
		}
	}
	return gateway.Allowed(), false
}

func enforceAuthority(req Request, snapshot app.ClaimAuthoritySnapshot) (gateway.Outcome, bool) {
	authority := snapshot.EffectiveAuthority
	if authority.ID == "" || authority.ID != snapshot.Claim.AuthorityRef {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: gateway.CategoryAuthorityUnavailable,
			Reason:   fmt.Sprintf("claim %q has no matching system-issued effective authority", req.ClaimID),
		}, true
	}
	if authority.ModelProfile != req.Profile {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: gateway.CategoryModelProfileNotGranted,
			Reason:   fmt.Sprintf("model profile %q is not granted to claim %q", req.Profile, req.ClaimID),
		}, true
	}
	return gateway.Allowed(), false
}

func (g *Gateway) requireRunning(claimID string) (gateway.Outcome, bool) {
	if err := app.RequireRunningClaim(g.claims, claimID); err != nil {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryClaimNotActive, Reason: err.Error()}, true
	}
	return gateway.Allowed(), false
}
