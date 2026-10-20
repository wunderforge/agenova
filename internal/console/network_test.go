// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/workstore"
	"strings"
	"testing"
)

// Explicitly simulated contract behavior for test-only backends. These methods
// do not claim socket enforcement or replace a real CNI test.
func simulatedNetwork(claim string, id v0.SandboxClaimBackendIdentity, n runtime.NetworkAuthority, state string) runtime.NetworkEvidence {
	return runtime.NetworkEvidence{ClaimID: claim, Identity: id, Profiles: append([]string(nil), n.Profiles...), ExpiresAt: n.ExpiresAt, State: state, EvidenceLevel: "Simulated"}
}

type recoveryNetworkWriteFault struct {
	DurableWork
	failOperation string
	writerErr     error
}

func (s recoveryNetworkWriteFault) CheckWriter(context.Context) error            { return s.writerErr }
func (s recoveryNetworkWriteFault) LastSequence(context.Context) (uint64, error) { return 0, nil }
func (s recoveryNetworkWriteFault) Append(_ context.Context, f facts.Fact, _ *v0.IssuedState) error {
	if f.Operation == s.failOperation {
		return workstore.ErrUnavailable
	}
	return nil
}

type recoveryNetworkCounter struct {
	recoveryTestBackend
	revoked int
}

func (b *recoveryNetworkCounter) RevokeNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	b.revoked++
	return simulatedNetwork(b.claim, id, b.network, "Revoked"), nil
}

func TestRecoveryWriteFailureNeverSkipsVerifiedNetworkRevocation(t *testing.T) {
	for _, operation := range []string{"Failed", "NetworkRevocationAttempt", "NetworkRevoked"} {
		t.Run(operation, func(t *testing.T) {
			prepared, err := app.PrepareReferenceAssignment(verticalRequest(t, "recovery-network-fault"), app.ReferencePrincipalTeamA)
			if err != nil {
				t.Fatal(err)
			}
			id := v0.SandboxClaimBackendIdentity{Backend: "test", WorkerID: "worker", AllocationID: "opaque-original"}
			prepared.Issued.Claim.Phase = v0.ClaimPhaseRunning
			prepared.Issued.Claim.BackendIdentity = &id
			view := evidence.View{Version: "agenova.evidence/v0", RequestRef: prepared.Request.Metadata.Name, Request: prepared.Request, State: prepared.Issued, Facts: []facts.Fact{{Kind: "Runtime", RequestRef: prepared.Request.Metadata.Name, Operation: "Running"}}}
			backend := &recoveryNetworkCounter{}
			err = recoverAllocation(context.Background(), recoveryNetworkWriteFault{failOperation: operation}, backend, workstore.Snapshot{View: view})
			if !errors.Is(err, workstore.ErrUnavailable) || backend.revoked != 1 {
				t.Fatalf("failure=%v revoked=%d", err, backend.revoked)
			}
		})
	}
}
func (b *verticalBackend) ActivateNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	return simulatedNetwork(b.claim, id, b.network, "Active"), nil
}
func (b *verticalBackend) RevokeNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	return simulatedNetwork(b.claim, id, b.network, "Revoked"), nil
}
func (b *httpBackend) ActivateNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	return simulatedNetwork(b.claim, id, b.network, "Active"), nil
}
func (b *httpBackend) RevokeNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	return simulatedNetwork(b.claim, id, b.network, "Revoked"), nil
}

func TestNetworkLifecycleFactsPreserveSimulatedEvidenceLevel(t *testing.T) {
	s, err := NewService(&verticalBackend{}, &verticalExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	view, err := s.Submit(verticalRequest(t, "network-evidence"))
	if err != nil {
		t.Fatal(err)
	}
	result := awaitVertical(t, s, view.RequestRef)
	seen := map[string]bool{}
	for _, f := range result.Facts {
		if f.Operation == app.EventNetworkActive || f.Operation == app.EventNetworkRevoked {
			if !strings.Contains(f.Reason, "Simulated") || !strings.Contains(f.Reason, "not a packet-level") {
				t.Fatalf("network evidence overstated: %+v", f)
			}
			seen[f.Operation] = true
		}
	}
	if !seen[app.EventNetworkActive] || !seen[app.EventNetworkRevoked] {
		t.Fatal("missing network lifecycle facts", seen)
	}
}

func TestRecoveryNetworkReleaseRejectsUnknownOrMismatchedProof(t *testing.T) {
	id := v0.SandboxClaimBackendIdentity{Backend: "test", WorkerID: "worker", AllocationID: "opaque-incarnation"}
	proof := simulatedNetwork("claim", id, runtime.NetworkAuthority{}, "Revoked")
	if !validNetworkRelease(proof, "claim", id) {
		t.Fatal("valid simulated release refused")
	}
	for _, change := range []func(*runtime.NetworkEvidence){
		func(v *runtime.NetworkEvidence) { v.ClaimID = "other" },
		func(v *runtime.NetworkEvidence) { v.Identity.AllocationID = "other" },
		func(v *runtime.NetworkEvidence) { v.State = "Active" },
		func(v *runtime.NetworkEvidence) { v.EvidenceLevel = "" },
		func(v *runtime.NetworkEvidence) { v.EvidenceLevel = "BackendVerified" },
	} {
		bad := proof
		change(&bad)
		if validNetworkRelease(bad, "claim", id) {
			t.Fatalf("accepted invalid proof: %+v", bad)
		}
	}
}
