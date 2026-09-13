// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/sandbox"
)

// Inject below RuntimeBackend so the shared cases execute the real state
// handling around failed resource operations, rather than a backend stub.
func injectWorkerFailure(t *testing.T, r *Runtime, id v1alpha1.SandboxClaimBackendIdentity, operation string, cause error) {
	t.Helper()
	a, err := r.allocationFor(id)
	if err != nil {
		t.Fatal(err)
	}
	a.pool = &failingWorkerPool{workerPool: a.pool, operation: operation, cause: cause}
}

type failingWorkerPool struct {
	workerPool
	operation string
	cause     error
}

func (p *failingWorkerPool) fail(operation string) error {
	if p.operation != operation {
		return nil
	}
	p.operation = ""
	return p.cause
}

func (p *failingWorkerPool) MarkRunning(workerID, claimID string) error {
	if err := p.fail("start"); err != nil {
		return err
	}
	return p.workerPool.MarkRunning(workerID, claimID)
}

func (p *failingWorkerPool) MarkFailed(workerID, claimID string) error {
	if err := p.fail("terminate"); err != nil {
		return err
	}
	return p.workerPool.MarkFailed(workerID, claimID)
}

func (p *failingWorkerPool) Replace(workerID, claimID string) (sandbox.Sandbox, error) {
	if err := p.fail("cleanup"); err != nil {
		return sandbox.Sandbox{}, err
	}
	return p.workerPool.Replace(workerID, claimID)
}
