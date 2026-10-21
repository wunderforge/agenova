// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration && controlled

package agentsandbox

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/runtime"
	runtimebackend "github.com/wunderforge/agenova/internal/runtime/agentsandbox"
)

// These fakes exercise wrapper/application composition WITHOUT kubectl,
// process allocation, inference or CNI claims. Evidence is explicitly simulated.
type recordingLegacyFake struct {
	request     runtime.AllocateRequest
	allocation  runtime.Allocation
	allocations int
}

func (f *recordingLegacyFake) Allocate(req runtime.AllocateRequest) (runtime.Allocation, error) {
	f.request = req
	f.allocations++
	f.allocation = runtime.Allocation{ClaimID: req.ClaimID, Identity: v0.SandboxClaimBackendIdentity{Backend: runtimebackend.BackendName, WorkerID: "cluster-free-worker", AllocationID: "simulated-incarnation"}}
	return f.allocation, nil
}
func (f *recordingLegacyFake) Observe(id v0.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	return runtime.Observation{ClaimID: f.request.ClaimID, Identity: id, Ready: true}, nil
}
func (*recordingLegacyFake) Start(v0.SandboxClaimBackendIdentity) error     { return nil }
func (*recordingLegacyFake) Terminate(v0.SandboxClaimBackendIdentity) error { return nil }
func (*recordingLegacyFake) Cleanup(id v0.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	return runtime.CleanupResult{Identity: id, Released: true}, nil
}

type recordingNetworkFake struct {
	*recordingLegacyFake
	activationErr, errorOnRevoke   error
	lastActivation, lastRevocation v0.SandboxClaimBackendIdentity
	activationContext              context.Context
}

func (f *recordingNetworkFake) ActivateNetwork(ctx context.Context, id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	f.activationContext = ctx
	if err := ctx.Err(); err != nil {
		return runtime.NetworkEvidence{}, err
	}
	f.lastActivation = id
	return runtime.NetworkEvidence{ClaimID: f.request.ClaimID, Identity: id, Profiles: append([]string(nil), f.request.Network.Profiles...), ExpiresAt: f.request.Network.ExpiresAt, State: "Active", EvidenceLevel: "Simulated"}, f.activationErr
}
func (f *recordingNetworkFake) RevokeNetwork(id v0.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	f.lastRevocation = id
	return runtime.NetworkEvidence{ClaimID: f.request.ClaimID, Identity: id, Profiles: append([]string(nil), f.request.Network.Profiles...), ExpiresAt: f.request.Network.ExpiresAt, State: "Revoked", EvidenceLevel: "Simulated"}, f.errorOnRevoke
}

func TestRecordingRuntimeNetworkCapability_ClusterFree(t *testing.T) {
	backend := &recordingNetworkFake{recordingLegacyFake: &recordingLegacyFake{}}
	recorder, recorded := newRecordingRuntimeBackend(backend)
	if _, ok := recorded.(runtime.NetworkBackend); !ok {
		t.Fatal("wrapper hid NetworkBackend")
	}
	recorder.afterStart = func(string, v0.SandboxClaimBackendIdentity) error {
		recorder.workerResult = "SIMULATED wrapper regression, no process/CNI evidence"
		return nil
	}
	path := filepath.Join("..", "..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", "valid-team-a-engineer.yaml")
	report, err := app.SubmitClaimRequestFile(path, recorded, app.ReferencePrincipalTeamA)
	if err != nil || report.Phase != v0.ClaimPhaseSucceeded {
		t.Fatalf("governed composition failed: %+v %v", report, err)
	}
	if err := recorder.assertCompleted(report.ClaimID); err != nil {
		t.Fatal(err)
	}
	if backend.lastActivation != recorder.allocation.Identity || backend.lastRevocation != recorder.allocation.Identity {
		t.Fatal("wrapper changed network binding")
	}
	if backend.request.Network.ExpiresAt.IsZero() || len(backend.request.Network.Profiles) != 0 {
		t.Fatal("offline authority/expiry not forwarded")
	}
	want := []string{"Allocate", "Observe", "Start", "ActivateNetwork", "RevokeNetwork", "Terminate", "Cleanup"}
	if !reflect.DeepEqual(recorder.calls, want) {
		t.Fatalf("call order %v want %v", recorder.calls, want)
	}
}

func TestRecordingRuntimeNetworkUnsupportedFailsBeforeAllocation_ClusterFree(t *testing.T) {
	backend := &recordingLegacyFake{}
	recorder, recorded := newRecordingRuntimeBackend(backend)
	if _, ok := recorded.(runtime.NetworkBackend); ok {
		t.Fatal("wrapper fabricated NetworkBackend")
	}
	path := filepath.Join("..", "..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", "valid-team-a-engineer.yaml")
	_, err := app.SubmitClaimRequestFile(path, recorded, app.ReferencePrincipalTeamA)
	if !errors.Is(err, runtime.ErrUnsupported) || backend.allocations != 0 || recorder.callCount() != 0 {
		t.Fatalf("unsupported backend reached allocation: %v calls=%v", err, recorder.calls)
	}
}

func TestRecordingRuntimeNetworkErrorsForwarded_ClusterFree(t *testing.T) {
	activationErr, revokeErr := errors.New("activation rejected"), errors.New("revoke unconfirmed")
	backend := &recordingNetworkFake{recordingLegacyFake: &recordingLegacyFake{}, activationErr: activationErr, errorOnRevoke: revokeErr}
	recorder, recorded := newRecordingRuntimeBackend(backend)
	network := recorded.(runtime.NetworkBackend)
	id := v0.SandboxClaimBackendIdentity{Backend: "test", WorkerID: "original", AllocationID: "original-incarnation"}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), struct{ label string }{"test-context"}, true))
	defer cancel()
	proof, err := network.ActivateNetwork(ctx, id)
	if !errors.Is(err, activationErr) || proof.Identity != id || backend.lastActivation != id {
		t.Fatal("activation error/binding suppressed")
	}
	if backend.activationContext != ctx {
		t.Fatal("wrapper replaced caller activation context")
	}
	proof, err = network.RevokeNetwork(id)
	if !errors.Is(err, revokeErr) || proof.Identity != id || backend.lastRevocation != id {
		t.Fatal("revocation error/binding suppressed")
	}
	if !reflect.DeepEqual(recorder.calls, []string{"ActivateNetwork", "RevokeNetwork"}) {
		t.Fatal("network calls not recorded")
	}
}

func TestRecordingRuntimeNetworkCancellationForwarded_ClusterFree(t *testing.T) {
	backend := &recordingNetworkFake{recordingLegacyFake: &recordingLegacyFake{}}
	_, recorded := newRecordingRuntimeBackend(backend)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := recorded.(runtime.NetworkBackend).ActivateNetwork(ctx, v0.SandboxClaimBackendIdentity{Backend: "test", WorkerID: "original"})
	if !errors.Is(err, context.Canceled) || backend.activationContext != ctx || backend.lastActivation.WorkerID != "" {
		t.Fatal("wrapper lost cancellation or allowed activation")
	}
}

func TestGovernedNetworkPrerequisite_ClusterFree(t *testing.T) {
	if _, err := governedNetworkPrerequisite(false); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatal("kind/network isolation was silently assumed")
	}
	if mode, err := governedNetworkPrerequisite(true); err != nil || mode != "strict" {
		t.Fatalf("explicit attestation %q %v", mode, err)
	}
}
