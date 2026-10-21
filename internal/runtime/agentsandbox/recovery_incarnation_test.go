// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"errors"
	"testing"

	"github.com/wunderforge/agenova/internal/runtime"
)

// API/controller simulation only: unlike networkKube, GET of an absent Pod
// really fails. The error-aware existence query must establish absence first.
type partialRecoveryKube struct {
	*incarnationCleanupKube
	podQueryErr, podGetErr error
	podGets                int
}

func (k *partialRecoveryKube) exists(resource, name string) (bool, error) {
	if resource == "pods" && k.podQueryErr != nil {
		return false, k.podQueryErr
	}
	return k.incarnationCleanupKube.exists(resource, name)
}
func (k *partialRecoveryKube) get(resource, name string, dst any) error {
	if resource == "pods" {
		k.podGets++
		if k.podGetErr != nil {
			return k.podGetErr
		}
		if !k.podPresent {
			return errors.New("simulated Pod NotFound; not absence proof")
		}
	}
	return k.incarnationCleanupKube.get(resource, name, dst)
}

func restoredPartialAdapter(k *partialRecoveryKube) *ControlledAdapter {
	a := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
	a.kube = k
	return a
}

func TestRestoreOriginalClaimAfterIndependentExpiryRemovedPod(t *testing.T) {
	for _, sandboxPresent := range []bool{false, true} {
		t.Run(map[bool]string{false: "sandbox-already-gone", true: "sandbox-still-present"}[sandboxPresent], func(t *testing.T) {
			_, base, allocation := incarnationFixture(t)
			k := &partialRecoveryKube{incarnationCleanupKube: base}
			k.podPresent = false
			k.sandboxes[allocation.Identity.WorkerID] = sandboxPresent
			a := restoredPartialAdapter(k)
			if err := a.RestoreForCleanup(allocation.ClaimID, allocation.Identity); err != nil {
				t.Fatal("confirmed original Pod absence blocked recovery", err)
			}
			if k.podGets != 0 {
				t.Fatal("queried a Pod already confirmed absent")
			}
			entry := a.allocations[allocation.ClaimID]
			if entry.identity() != allocation.Identity || entry.state == allocReleased {
				t.Fatal("restored cleanup lost original incarnation or prematurely claimed full release")
			}
			if err := a.Start(allocation.Identity); err == nil {
				t.Fatal("cleanup-only absent-Pod state regained execution")
			}
			if evidence, err := a.RevokeNetwork(allocation.Identity); err != nil || evidence.State != "Revoked" {
				t.Fatal("original reservation could not be safely released", evidence, err)
			}
			if err := a.Terminate(allocation.Identity); err != nil {
				t.Fatal(err)
			}
			if result, err := a.Cleanup(allocation.Identity); err != nil || !result.Released {
				t.Fatal("recovery teardown did not complete", result, err)
			}
			if k.unpinned != 0 || k.pinnedDeletes != 1 || k.podPresent || len(k.claims) != 0 {
				t.Fatal("recovery deleted without original UID guard or retained resources")
			}
		})
	}
}

func TestRestorePodQueryFailuresDoNotProveAbsence(t *testing.T) {
	for _, failure := range []string{"existence-query", "present-Pod-read"} {
		t.Run(failure, func(t *testing.T) {
			_, base, allocation := incarnationFixture(t)
			k := &partialRecoveryKube{incarnationCleanupKube: base}
			readErr := errors.New("API unavailable")
			if failure == "existence-query" {
				k.podQueryErr = readErr
				k.podPresent = false
			} else {
				k.podGetErr = readErr
			}
			a := restoredPartialAdapter(k)
			if err := a.RestoreForCleanup(allocation.ClaimID, allocation.Identity); !errors.Is(err, readErr) {
				t.Fatal("failed query treated as confirmed absence", err)
			}
			if _, err := a.Observe(allocation.Identity); !errors.Is(err, runtime.ErrUnknownIdentity) {
				t.Fatal("failed restore published an identity", err)
			}
			if k.unpinned != 0 || k.pinnedDeletes != 0 {
				t.Fatal("failed read caused deletion")
			}
		})
	}
}

func TestRestoreNeverAdoptsSameNameReplacementOrOrphanPod(t *testing.T) {
	for _, scenario := range []string{"foreign-claim-missing-Pod", "same-claim-replacement-Pod", "missing-claim-remaining-Pod", "missing-claim-remaining-Sandbox"} {
		t.Run(scenario, func(t *testing.T) {
			_, base, allocation := incarnationFixture(t)
			k := &partialRecoveryKube{incarnationCleanupKube: base}
			name := allocation.Identity.WorkerID
			var expected error = errIdentityMismatch
			switch scenario {
			case "foreign-claim-missing-Pod":
				replaceIncarnation(base, name)
				k.podPresent = false
			case "same-claim-replacement-Pod":
				k.podUID = "replacement-Pod-same-name"
			case "missing-claim-remaining-Pod":
				delete(k.claims, name)
				delete(k.sandboxes, allocation.Identity.WorkerID)
				expected = errCleanupUnconfirmed
			case "missing-claim-remaining-Sandbox":
				delete(k.claims, name)
				k.podPresent = false
				expected = errCleanupUnconfirmed
			}
			a := restoredPartialAdapter(k)
			if err := a.RestoreForCleanup(allocation.ClaimID, allocation.Identity); !errors.Is(err, expected) {
				t.Fatal("unsafe partial state adopted", scenario, err)
			}
			if len(a.allocations) != 0 || k.unpinned != 0 || k.pinnedDeletes != 0 {
				t.Fatal("uncertain restore published ownership or deleted resources")
			}
		})
	}
}

func TestRestoreFullyAbsentIncarnationRemainsReleased(t *testing.T) {
	_, base, allocation := incarnationFixture(t)
	k := &partialRecoveryKube{incarnationCleanupKube: base}
	delete(k.claims, allocation.Identity.WorkerID)
	delete(k.sandboxes, allocation.Identity.WorkerID)
	k.podPresent = false
	a := restoredPartialAdapter(k)
	if err := a.RestoreForCleanup(allocation.ClaimID, allocation.Identity); err != nil {
		t.Fatal(err)
	}
	if result, err := a.Cleanup(allocation.Identity); err != nil || !result.Released || result.Identity != allocation.Identity {
		t.Fatal("confirmed full absence was lost", result, err)
	}
	if k.podGets != 0 || k.unpinned != 0 || k.pinnedDeletes != 0 {
		t.Fatal("absence recovery issued unnecessary read/delete")
	}
}

func TestRestoreMissingPodCannotDeleteLaterSameNameReplacement(t *testing.T) {
	_, base, allocation := incarnationFixture(t)
	k := &partialRecoveryKube{incarnationCleanupKube: base}
	k.podPresent = false
	a := restoredPartialAdapter(k)
	if err := a.RestoreForCleanup(allocation.ClaimID, allocation.Identity); err != nil {
		t.Fatal(err)
	}
	name := allocation.Identity.WorkerID
	k.beforePinned = func() { replaceIncarnation(base, name); k.podPresent = true }
	if _, err := a.RevokeNetwork(allocation.Identity); !errors.Is(err, errIdentityMismatch) {
		t.Fatal("recovery delete adopted a replacement between read/delete", err)
	}
	if result, err := a.SpikeAdapter.Cleanup(allocation.Identity); !errors.Is(err, errIdentityMismatch) || result.Released {
		t.Fatal("generic recovery fallback deleted replacement", result, err)
	}
	assertReplacementIntact(t, base, name)
}
