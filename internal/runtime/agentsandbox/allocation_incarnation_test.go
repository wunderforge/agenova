// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"errors"
	"testing"

	"github.com/wunderforge/agenova/internal/runtime"
)

// API/controller simulation: pinned deletion enforces both Kubernetes
// preconditions, and deliberately replacing between read/delete is supported.
type incarnationCleanupKube struct {
	*networkKube
	beforePinned                            func()
	unpinned, pinnedAttempts, pinnedDeletes int
}

func (k *incarnationCleanupKube) delete(resource, name string) error {
	k.unpinned++
	return k.networkKube.delete(resource, name)
}
func (k *incarnationCleanupKube) deletePinned(resource, name, uid, version string) error {
	k.pinnedAttempts++
	if k.beforePinned != nil {
		f := k.beforePinned
		k.beforePinned = nil
		f()
	}
	if resource == "sandboxclaims" {
		current := k.claims[name]
		if current == nil || current.Metadata.UID != uid || current.Metadata.ResourceVersion != version {
			return errIdentityMismatch
		}
	}
	if err := k.networkKube.deletePinned(resource, name, uid, version); err != nil {
		return err
	}
	k.pinnedDeletes++
	return nil
}

func incarnationFixture(t *testing.T) (*ControlledAdapter, *incarnationCleanupKube, runtime.Allocation) {
	t.Helper()
	a, k, _, allocation := networkFixture(t, nil)
	wrapper := &incarnationCleanupKube{networkKube: k}
	a.kube = wrapper
	return a, wrapper, allocation
}

func replaceIncarnation(k *incarnationCleanupKube, name string) {
	k.claimUID, k.podUID = "replacement-claim", "replacement-pod"
	k.claims[name].Metadata.UID = k.claimUID
	k.claims[name].Metadata.ResourceVersion = "replacement-version"
	// A coherent replacement deliberately retains the deterministic worker name
	// and even copies the old annotations. The original UID is the authority.
}

func assertReplacementIntact(t *testing.T, k *incarnationCleanupKube, name string) {
	t.Helper()
	if k.claims[name] == nil || !k.podPresent || !k.sandboxes[k.bindOnApply] || k.unpinned != 0 || k.pinnedDeletes != 0 {
		t.Fatalf("foreign replacement touched: claim=%v pod=%v unpinned=%d pinned=%d", k.claims[name], k.podPresent, k.unpinned, k.pinnedDeletes)
	}
}

// Compose the real backend methods in RunService's teardown order. This is
// backend/API simulation, not an actual RunService or real-cluster test.
func TestIncarnationBackendTeardownRejectsCoherentSameNameReplacement(t *testing.T) {
	a, k, allocation := incarnationFixture(t)
	name := resourceName("claim", allocation.ClaimID)
	replaceIncarnation(k, name)
	if _, err := a.RevokeNetwork(allocation.Identity); !errors.Is(err, errIdentityMismatch) {
		t.Fatal("strong revocation should reject replacement", err)
	}
	if err := a.Terminate(allocation.Identity); !errors.Is(err, errIdentityMismatch) {
		t.Fatal("termination must not adopt replacement after revocation failed", err)
	}
	if result, err := a.Cleanup(allocation.Identity); !errors.Is(err, errIdentityMismatch) || result.Released {
		t.Fatal("generic cleanup weakened revocation identity guard", result, err)
	}
	assertReplacementIntact(t, k, name)
	if observation, err := a.Observe(allocation.Identity); !errors.Is(err, errIdentityMismatch) || observation.Ready {
		t.Fatal("replacement was reported Ready", observation, err)
	}
}

func TestIncarnationCleanupPinnedDeleteClosesClaimTOCTOU(t *testing.T) {
	for _, change := range []string{"UID", "resourceVersion"} {
		t.Run(change, func(t *testing.T) {
			a, k, allocation := incarnationFixture(t)
			name := resourceName("claim", allocation.ClaimID)
			k.beforePinned = func() {
				if change == "UID" {
					replaceIncarnation(k, name)
				} else {
					k.claims[name].Metadata.ResourceVersion = "changed-after-read"
				}
			}
			result, err := a.SpikeAdapter.Cleanup(allocation.Identity)
			if !errors.Is(err, errIdentityMismatch) || result.Released || k.pinnedAttempts != 1 {
				t.Fatal("read/delete precondition failed open", result, err, k.pinnedAttempts)
			}
			assertReplacementIntact(t, k, name)
		})
	}
}

func TestIncarnationCleanupValidatesOwnershipAndPodBinding(t *testing.T) {
	for _, change := range []string{"attempt", "claim-id", "pod-uid", "pod-label", "missing-version"} {
		t.Run(change, func(t *testing.T) {
			a, k, allocation := incarnationFixture(t)
			name := resourceName("claim", allocation.ClaimID)
			switch change {
			case "attempt":
				k.claims[name].Metadata.Annotations["agenova.io/allocation-attempt"] = "foreign-attempt"
			case "claim-id":
				k.claims[name].Metadata.Annotations["agenova.io/claim-id"] = "foreign-work"
			case "pod-uid":
				k.podUID = "foreign-pod"
			case "pod-label":
				k.wrongPodBinding = true
			case "missing-version":
				k.claims[name].Metadata.ResourceVersion = ""
			}
			if result, err := a.SpikeAdapter.Cleanup(allocation.Identity); !errors.Is(err, errIdentityMismatch) || result.Released {
				t.Fatal("changed ownership deleted", result, err)
			}
			assertReplacementIntact(t, k, name)
			if observation, err := a.Observe(allocation.Identity); err == nil || observation.Ready {
				t.Fatal("changed incarnation reported Ready", observation, err)
			}
		})
	}
}

func TestRestoredIncarnationCleanupUsesOriginalUID(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "replacement"}[replacement], func(t *testing.T) {
			_, k, allocation := incarnationFixture(t)
			name := resourceName("claim", allocation.ClaimID)
			restored := newControlledAdapter(newTestAdapter(k.fakeKube), &fakeWorkerControl{})
			restored.kube = k
			if err := restored.RestoreForCleanup(allocation.ClaimID, allocation.Identity); err != nil {
				t.Fatal(err)
			}
			if replacement {
				replaceIncarnation(k, name)
			}
			result, err := restored.SpikeAdapter.Cleanup(allocation.Identity)
			if replacement {
				if !errors.Is(err, errIdentityMismatch) || result.Released {
					t.Fatal("restored cleanup adopted replacement", result, err)
				}
				assertReplacementIntact(t, k, name)
			} else if err != nil || !result.Released || k.unpinned != 0 || k.pinnedDeletes != 1 {
				t.Fatal("restored original could not be safely cleaned", result, err)
			}
		})
	}
}

func TestLegacyIdentityCannotAdoptGovernedClaimForCleanup(t *testing.T) {
	a, k, allocation := incarnationFixture(t)
	name := resourceName("claim", allocation.ClaimID)
	entry := a.allocations[allocation.ClaimID]
	entry.networkClaimUID, entry.networkPodUID, entry.attemptID = "", "", ""
	legacy := allocation.Identity
	legacy.AllocationID = ""
	if result, err := a.SpikeAdapter.Cleanup(legacy); !errors.Is(err, errIdentityMismatch) || result.Released {
		t.Fatal("legacy identity adopted governed allocation", result, err)
	}
	assertReplacementIntact(t, k, name)
}

func TestIncarnationCleanupDoesNotInferPodReleaseFromClaimDeletion(t *testing.T) {
	a, k, allocation := incarnationFixture(t)
	k.retainPod = true
	result, err := a.SpikeAdapter.Cleanup(allocation.Identity)
	if !errors.Is(err, errCleanupUnconfirmed) || result.Released || !k.podPresent || k.unpinned != 0 || k.pinnedDeletes != 1 {
		t.Fatal("claim deletion falsely proved Pod release", result, err)
	}
}
