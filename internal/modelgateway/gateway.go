package modelgateway

import (
	"errors"
	"fmt"

	"github.com/donozhang1992/agenova/api/v1alpha1"
	"github.com/donozhang1992/agenova/internal/facts"
	"github.com/donozhang1992/agenova/internal/governance"
	"github.com/donozhang1992/agenova/internal/runtime"
)

// Request is a model invocation request from an agent claim.
type Request struct {
	ClaimID   string
	ModelName string
}

// Gateway authorizes model requests by active claim and records ModelInvocation facts.
// A claim must be in the Running phase to invoke a model. Child claims whose parent
// is no longer Running are denied (child-out-of-scope).
type Gateway struct {
	backend runtime.RuntimeBackend
	lineage *governance.Lineage
	store   *facts.Store
}

func NewGateway(backend runtime.RuntimeBackend, lineage *governance.Lineage, store *facts.Store) *Gateway {
	return &Gateway{backend: backend, lineage: lineage, store: store}
}

// Authorize checks whether the requesting claim may invoke a model.
// On success it records a ModelInvocation fact. On failure it returns a
// descriptive authorization error and records nothing.
func (g *Gateway) Authorize(req Request) error {
	if req.ClaimID == "" {
		return errors.New("claim id is required")
	}
	if req.ModelName == "" {
		return errors.New("model name is required")
	}
	if err := g.requireRunning(req.ClaimID); err != nil {
		return err
	}
	// If this is a child claim, its parent must also be Running.
	if parentID, ok := g.lineage.Parent(req.ClaimID); ok {
		if err := g.requireRunning(parentID); err != nil {
			return fmt.Errorf("child claim %q out of parent scope: %w", req.ClaimID, err)
		}
	}
	g.store.RecordModelInvocation(req.ClaimID, req.ModelName)
	return nil
}

func (g *Gateway) requireRunning(claimID string) error {
	claim, ok := g.backend.Claim(claimID)
	if !ok {
		return fmt.Errorf("unknown claim: %s", claimID)
	}
	if claim.Status.Phase != v1alpha1.ClaimPhaseRunning {
		return fmt.Errorf("claim %q is not running (phase: %s)", claimID, claim.Status.Phase)
	}
	return nil
}
