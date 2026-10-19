// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package agentsandbox

import (
	"errors"
	"testing"

	"github.com/wunderforge/agenova/internal/runtime"
)

func TestRestoreForCleanupNeverRestartsWorker(t *testing.T) {
	original, k, worker, allocation := controlledFixture(t)
	k.claims[resourceName("claim", worker.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	if err := original.Start(allocation.Identity); err != nil {
		t.Fatal(err)
	}
	restored := newControlledAdapter(newTestAdapter(k), worker)
	if err := restored.RestoreForCleanup(worker.claim, allocation.Identity); err != nil {
		t.Fatal(err)
	}
	if err := restored.Start(allocation.Identity); !errors.Is(err, runtime.ErrAlreadyStarted) {
		t.Fatal("restored start admitted", err)
	}
	if _, err := restored.Cleanup(allocation.Identity); err == nil {
		t.Fatal("cleanup before confirmed stop")
	}
	if err := restored.Terminate(allocation.Identity); err != nil {
		t.Fatal(err)
	}
	if result, err := restored.Cleanup(allocation.Identity); err != nil || !result.Released {
		t.Fatal("cleanup", err)
	}
	if worker.starts != 1 || worker.stops != 1 {
		t.Fatal("worker restarted or stop omitted", worker.starts, worker.stops)
	}
	afterCleanup := newControlledAdapter(newTestAdapter(k), worker)
	if err := afterCleanup.RestoreForCleanup(worker.claim, allocation.Identity); err != nil {
		t.Fatal(err)
	}
	if err := afterCleanup.Start(allocation.Identity); !errors.Is(err, runtime.ErrReleased) {
		t.Fatal("released worker restarted", err)
	}
	if err := afterCleanup.Terminate(allocation.Identity); !errors.Is(err, runtime.ErrReleased) {
		t.Fatal("absence fabricated stop acknowledgement", err)
	}
	if result, err := afterCleanup.Cleanup(allocation.Identity); err != nil || !result.Released {
		t.Fatal("confirmed absence lost", err)
	}
}
func TestRestoreForCleanupRejectsChangedBinding(t *testing.T) {
	_, k, worker, allocation := controlledFixture(t)
	k.claims[resourceName("claim", worker.claim)].Status.Sandbox.Name = "different-worker"
	restored := newControlledAdapter(newTestAdapter(k), worker)
	if err := restored.RestoreForCleanup(worker.claim, allocation.Identity); !errors.Is(err, errIdentityMismatch) {
		t.Fatal("changed binding accepted", err)
	}
	if worker.starts != 0 || worker.stops != 0 || k.delets != 0 {
		t.Fatal("changed binding mutated backend")
	}
}
func TestRestoreForCleanupDoesNotInferOrphanRelease(t *testing.T) {
	_, k, worker, allocation := controlledFixture(t)
	delete(k.claims, resourceName("claim", worker.claim))
	restored := newControlledAdapter(newTestAdapter(k), worker)
	if err := restored.RestoreForCleanup(worker.claim, allocation.Identity); !errors.Is(err, errCleanupUnconfirmed) {
		t.Fatal("orphan worker accepted", err)
	}
	if k.delets != 0 || worker.stops != 0 {
		t.Fatal("orphan worker mutated")
	}
}

func TestRestoreForCleanupReadFailureDoesNotProveRelease(t *testing.T) {
	_, k, worker, allocation := controlledFixture(t)
	k.existsErr = errors.New("API unavailable")
	restored := newControlledAdapter(newTestAdapter(k), worker)
	if err := restored.RestoreForCleanup(worker.claim, allocation.Identity); err == nil {
		t.Fatal("failed read treated as release")
	}
	if k.delets != 0 || worker.stops != 0 {
		t.Fatal("failed read mutated backend")
	}
	if _, err := restored.Observe(allocation.Identity); !errors.Is(err, runtime.ErrUnknownIdentity) {
		t.Fatal("failed restore published identity", err)
	}
}
