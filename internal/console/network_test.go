// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/connectedclient"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/workstore"
	"net/http"
	"net/http/httptest"
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
	revoked, restores int
}

func (b *recoveryNetworkCounter) RestoreForCleanup(claim string, id v0.SandboxClaimBackendIdentity) error {
	b.restores++
	return b.recoveryTestBackend.RestoreForCleanup(claim, id)
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

type recoveryActivationCrashStore struct {
	recoveryNetworkWriteFault
	snapshot workstore.Snapshot
	finished *evidence.View
}

func (s *recoveryActivationCrashStore) Pending(context.Context) ([]workstore.Snapshot, error) {
	if s.finished != nil {
		return nil, nil
	}
	return []workstore.Snapshot{s.snapshot}, nil
}

func (s *recoveryActivationCrashStore) LastSequence(context.Context) (uint64, error) {
	var last uint64
	for _, fact := range s.snapshot.View.Facts {
		if fact.Sequence > last {
			last = fact.Sequence
		}
	}
	return last, nil
}

func (s *recoveryActivationCrashStore) Finish(_ context.Context, view evidence.View) error {
	copy := evidence.Clone(view)
	s.finished = &copy
	return nil
}

// The activation failure is committed as a Failed claim before teardown. A
// crash in that window must not block all subsequent admissions on restart.
func TestRecoveryRecognizesCommittedNetworkActivationFailure(t *testing.T) {
	for _, operation := range []string{app.EventNetworkActivationFailed, "UnrecognizedNetworkFailure"} {
		t.Run(operation, func(t *testing.T) {
			prepared, err := app.PrepareReferenceAssignment(verticalRequest(t, "activation-crash"), app.ReferencePrincipalTeamA)
			if err != nil {
				t.Fatal(err)
			}
			id := v0.SandboxClaimBackendIdentity{Backend: "test", WorkerID: "worker", AllocationID: "opaque-original"}
			prepared.Issued.Claim.Phase = v0.ClaimPhaseFailed
			prepared.Issued.Claim.BackendIdentity = &id
			view := evidence.View{Version: "agenova.evidence/v0", RequestRef: prepared.Request.Metadata.Name, Request: prepared.Request, State: prepared.Issued, Facts: []facts.Fact{{Kind: "Runtime", RequestRef: prepared.Request.Metadata.Name, Operation: operation}}}
			store := &recoveryActivationCrashStore{snapshot: workstore.Snapshot{View: view}}
			backend := &recoveryNetworkCounter{}
			count, err := RecoverDurableWork(context.Background(), store, backend)
			if operation != app.EventNetworkActivationFailed {
				if !errors.Is(err, ErrRecoveryRequired) || count != 0 || backend.restores != 0 || backend.revoked != 0 || store.finished != nil {
					t.Fatalf("unknown terminal was adopted: count=%d err=%v backend=%+v", count, err, backend)
				}
				return
			}
			if err != nil || count != 1 || backend.restores != 1 || backend.revoked != 1 || backend.stopCalls != 1 || backend.cleanupCalls != 1 {
				t.Fatalf("activation failure not safely recovered: count=%d err=%v backend=%+v", count, err, backend)
			}
			if store.finished == nil || store.finished.Outcome == nil || store.finished.Outcome.Status != "Failed" || store.finished.Outcome.Text != "" || store.finished.Outcome.Model != nil {
				t.Fatal("recovery fabricated success or missed final failure")
			}
			count, err = RecoverDurableWork(context.Background(), store, backend)
			if err != nil || count != 0 || backend.revoked != 1 {
				t.Fatal("recovery repeated release or execution", count, err)
			}
		})
	}
}

type activationFailureBackend struct{ verticalBackend }

func (*activationFailureBackend) ActivateNetwork(v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	return runtime.NetworkEvidence{}, runtime.ErrUnsupported
}

// Exercise the actual service's network facts through the production CLI
// decoder, not just raw JSON or hand-authored browser fixtures.
func TestNetworkLifecycleEvidenceRoundTripsThroughConnectedClient(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "activation-failed"}[failure], func(t *testing.T) {
			var backend runtime.RuntimeBackend = &verticalBackend{}
			if failure {
				backend = &activationFailureBackend{}
			}
			service, err := NewService(backend, &verticalExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA)
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			initial, err := service.Submit(verticalRequest(t, "network-cli-roundtrip"))
			if err != nil {
				t.Fatal(err)
			}
			view := awaitVertical(t, service, initial.RequestRef)
			want := "Succeeded"
			if failure {
				want = "Failed"
			}
			if view.Outcome == nil || view.Outcome.Status != want {
				t.Fatal("incorrect source outcome", view.Outcome)
			}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, view) }))
			defer server.Close()
			client := connectedclient.Client{Server: server.URL, HTTPClient: server.Client(), AccessToken: func(context.Context) (string, error) { return "synthetic-fixture-token", nil }}
			got, err := client.Show(view.RequestRef)
			if err != nil || got.Outcome == nil || got.Outcome.Status != want {
				t.Fatalf("CLI rejected actual network lifecycle evidence: %v", err)
			}
			if failure {
				// Recreate the exact crash window from genuine service facts:
				// Failed was committed, but network teardown/Finish was not.
				crash := evidence.Clone(view)
				for i, fact := range crash.Facts {
					if fact.Operation == app.EventNetworkActivationFailed {
						crash.Facts = crash.Facts[:i+1]
						break
					}
				}
				crash.Outcome = nil
				crash.State.Evidence.RuntimeEvents = nil
				for _, fact := range crash.Facts {
					if fact.Kind == "Runtime" && fact.Operation != "Pending" {
						crash.State.Evidence.RuntimeEvents = append(crash.State.Evidence.RuntimeEvents, v0.EvidenceRuntimeEvent{Kind: fact.Operation})
					}
				}
				store := &recoveryActivationCrashStore{snapshot: workstore.Snapshot{View: crash}}
				backend := &recoveryNetworkCounter{}
				if count, err := RecoverDurableWork(context.Background(), store, backend); err != nil || count != 1 || backend.revoked != 1 {
					t.Fatal("actual activation-failure snapshot could not recover", count, err)
				}
				view = *store.finished
				got, err = client.Show(view.RequestRef)
				if err != nil || got.Outcome == nil || got.Outcome.Status != "Failed" {
					t.Fatal("CLI rejected actual recovered activation-failure evidence", err)
				}
			}
		})
	}
}
