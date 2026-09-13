// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolgateway

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/governance"
)

// Request is a backend-neutral governed tool invocation. Provider credentials
// are never part of this contract.
type Request struct {
	ClaimID       string
	Tool          string
	Action        string
	ResourceScope string
	Parameters    map[string]string
	// CallerReference is untrusted metadata and is never adopted as the
	// gateway-issued invocation identity.
	CallerReference string
}

// Adapter is the provider-side seam. Only Allow decisions reach it.
type Adapter interface {
	Invoke(invocationID string, req Request) error
}

type unconfiguredAdapter struct{}

func (unconfiguredAdapter) Invoke(string, Request) error {
	return errors.New("no provider adapter configured")
}

// Policy evaluates a structurally valid request after the authoritative
// Running-only baseline. Effective-authority enforcement is supplied by #34.
type Policy func(req Request) gateway.Outcome

// Gateway governs tool attempts against the application-owned claim view.
type Gateway struct {
	claims  app.ClaimReader
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

func NewGateway(claims app.ClaimReader, lineage *governance.Lineage, store *facts.Store, opts ...Option) *Gateway {
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
		return gateway.Decision{}, errors.New("tool gateway is unavailable")
	}
	id := g.ids()

	if outcome, rejected := validate(req); rejected {
		return decision(id, outcome), nil
	}
	if outcome, unresolved := g.resolveClaim(req.ClaimID); unresolved {
		return decision(id, outcome), nil
	}
	if outcome, denied := g.claimBaseline(req.ClaimID); denied {
		return g.record(req, id, outcome), nil
	}
	outcome := g.policy(req).Normalize()
	if outcome.Result != gateway.ResultAllow {
		return g.record(req, id, outcome), nil
	}

	allowed := g.record(req, id, outcome)
	if err := g.adapter.Invoke(id, req); err != nil {
		return allowed, fmt.Errorf("tool adapter invocation %s: %w", id, err)
	}
	return allowed, nil
}

func decision(id string, outcome gateway.Outcome) gateway.Decision {
	return gateway.Decision{
		InvocationID: id,
		Result:       outcome.Result,
		Category:     outcome.Category,
		Reason:       outcome.Reason,
	}
}

func (g *Gateway) record(req Request, id string, outcome gateway.Outcome) gateway.Decision {
	g.store.RecordToolInvocation(req.ClaimID, toolName(req), id, outcome.Result)
	return decision(id, outcome)
}

func validate(req Request) (gateway.Outcome, bool) {
	if strings.TrimSpace(req.ClaimID) == "" {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryMissingClaimIdentity, Reason: "request carries no claim identity"}, true
	}
	if strings.TrimSpace(req.Tool) == "" || strings.TrimSpace(req.Action) == "" {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryIncompleteOperation, Reason: "tool request must identify tool and action"}, true
	}
	if gateway.AmbiguousResourceScope(req.ResourceScope) {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryAmbiguousResourceScope, Reason: fmt.Sprintf("resource scope %q does not name one bounded resource", req.ResourceScope)}, true
	}
	if key, found := gateway.FindReservedCredentialKey(req.Parameters); found {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategorySecretValue, Reason: fmt.Sprintf("parameter %q is a reserved credential key; credentials stay behind gateway adapters", key)}, true
	}
	return gateway.Allowed(), false
}

// resolveClaim prevents an untrusted or unknown claim ID from acquiring a
// fabricated claim-attributed fact.
func (g *Gateway) resolveClaim(claimID string) (gateway.Outcome, bool) {
	if g.claims == nil {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryGatewayUnavailable, Reason: "authoritative claim reader is unavailable"}, true
	}
	if _, ok := g.claims.Claim(claimID); !ok {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryUnknownClaim, Reason: fmt.Sprintf("unknown claim: %s", claimID)}, true
	}
	return gateway.Allowed(), false
}

func (g *Gateway) claimBaseline(claimID string) (gateway.Outcome, bool) {
	if outcome, denied := g.requireRunning(claimID); denied {
		return outcome, true
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

func (g *Gateway) requireRunning(claimID string) (gateway.Outcome, bool) {
	if err := app.RequireRunningClaim(g.claims, claimID); err != nil {
		return gateway.Outcome{Result: gateway.ResultDeny, Category: gateway.CategoryClaimNotActive, Reason: err.Error()}, true
	}
	return gateway.Allowed(), false
}

func toolName(req Request) string {
	return req.Tool + "." + req.Action
}
