// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/runtime"
	backend "github.com/wunderforge/agenova/internal/runtime/agentsandbox"
	"github.com/wunderforge/agenova/internal/workstore"
)

func compatibilitySnapshot(t *testing.T) workstore.Snapshot {
	t.Helper()
	p, err := app.PrepareReferenceAssignment(verticalRequest(t, "legacy-recovery"), app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	p.Issued.Claim.ID = "legacy-recovery"
	p.Issued.Evidence.ClaimID = p.Issued.Claim.ID
	p.Issued.Claim.Phase = v0.ClaimPhaseRunning
	p.Issued.Claim.BackendIdentity = &v0.SandboxClaimBackendIdentity{Backend: backend.BackendName, WorkerID: "legacy-worker"}
	p.Issued.Evidence.RuntimeEvents = []v0.EvidenceRuntimeEvent{{Kind: "Bound"}, {Kind: "Running"}}
	return workstore.Snapshot{View: evidence.View{Version: "agenova.evidence/v0", RequestRef: p.Request.Metadata.Name, Request: p.Request, State: p.Issued, Facts: []facts.Fact{{Kind: "Runtime", RequestRef: p.Request.Metadata.Name, ClaimID: p.Issued.Claim.ID, Operation: "Running"}}}}
}

// Hiding just the optional capability must not turn a new-contract allocation
// into an old one. This wrapper still supports ordinary trusted cleanup.
type legacyCleanupOnly struct{ b *recoveryNetworkCounter }

func (b legacyCleanupOnly) Allocate(r runtime.AllocateRequest) (runtime.Allocation, error) {
	return b.b.Allocate(r)
}
func (b legacyCleanupOnly) Observe(id v0.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	return b.b.Observe(id)
}
func (b legacyCleanupOnly) Start(id v0.SandboxClaimBackendIdentity) error { return b.b.Start(id) }
func (b legacyCleanupOnly) Terminate(id v0.SandboxClaimBackendIdentity) error {
	return b.b.Terminate(id)
}
func (b legacyCleanupOnly) Cleanup(id v0.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	return b.b.Cleanup(id)
}
func (b legacyCleanupOnly) RestoreForCleanup(claim string, id v0.SandboxClaimBackendIdentity) error {
	return b.b.RestoreForCleanup(claim, id)
}

func TestRecoveryNetworkCompatibilityMarkersNeverDowngrade(t *testing.T) {
	for _, marker := range []string{"none", "allocation-incarnation", "requested-public", "effective-public", "runtime-fact", "state-event", "fact-incarnation", "authority-fact", "unknown-network-event"} {
		t.Run(marker, func(t *testing.T) {
			s := compatibilitySnapshot(t)
			switch marker {
			case "allocation-incarnation":
				s.View.State.Claim.BackendIdentity.AllocationID = "original-claim/original-pod"
			case "requested-public":
				s.View.Request.Spec.RequestedAccess.NetworkProfiles = []string{"public-web"}
			case "effective-public":
				s.View.State.EffectiveAuthority.NetworkProfiles = []string{"public-web"}
			case "runtime-fact":
				s.View.Facts = append(s.View.Facts, facts.Fact{Kind: "Runtime", RequestRef: s.View.RequestRef, ClaimID: s.View.State.Claim.ID, Operation: "NetworkActivationAttempt"})
			case "state-event":
				s.View.State.Evidence.RuntimeEvents = append(s.View.State.Evidence.RuntimeEvents, v0.EvidenceRuntimeEvent{Kind: "NetworkActivationAttempt"})
			case "fact-incarnation":
				id := *s.View.State.Claim.BackendIdentity
				id.AllocationID = "original-claim/original-pod"
				s.View.Facts[0].BackendIdentity = &id
			case "authority-fact":
				a := *s.View.State.EffectiveAuthority
				a.NetworkProfiles = []string{"public-web"}
				s.View.Facts = append(s.View.Facts, facts.Fact{Kind: "AuthorityResolved", RequestRef: s.View.RequestRef, ClaimID: s.View.State.Claim.ID, Authority: &a})
			case "unknown-network-event":
				s.View.Facts = append(s.View.Facts, facts.Fact{Kind: "Runtime", RequestRef: s.View.RequestRef, ClaimID: s.View.State.Claim.ID, Operation: "NetworkUnrecognizedFutureEvent"})
			}
			if requiresRecoveryNetworkRelease(s.View) != (marker != "none") {
				t.Fatal("incorrect compatibility classification")
			}
			b := &recoveryNetworkCounter{}
			store := &recoveryActivationCrashStore{snapshot: s}
			count, err := RecoverDurableWork(context.Background(), store, legacyCleanupOnly{b})
			if marker == "none" {
				if err != nil || count != 1 || b.stopCalls != 1 || b.cleanupCalls != 1 || b.revoked != 0 {
					t.Fatalf("ordinary legacy cleanup regressed: count=%d err=%v backend=%+v", count, err, b)
				}
			} else if !errors.Is(err, ErrRecoveryRequired) || count != 0 || b.restores != 0 || b.stopCalls != 0 || b.cleanupCalls != 0 || store.finished != nil {
				t.Fatalf("new contract silently downgraded: count=%d err=%v backend=%+v", count, err, b)
			}
		})
	}
}

func TestRecoveryLegacyFailedPersistenceUsesOrdinarySafetyCleanup(t *testing.T) {
	s := compatibilitySnapshot(t)
	b := &recoveryNetworkCounter{}
	err := recoverAllocation(context.Background(), recoveryNetworkWriteFault{failOperation: "Failed"}, b, s)
	if !errors.Is(err, workstore.ErrUnavailable) || b.restores != 1 || b.revoked != 0 || b.stopCalls != 1 || b.cleanupCalls != 1 {
		t.Fatalf("legacy failure invented network revocation or skipped safety cleanup: %v %+v", err, b)
	}
}

// This is a production console -> production ControlledAdapter -> production
// kubectlRunner composition. Only the kubectl executable is replaced, so no
// cluster, authentication or cloud API can be reached by these tests.
type recoveryCLIState struct {
	NewMetadata    bool
	Absent         bool
	ReadFailure    bool
	DeleteFailure  bool
	ChangedBinding bool
	Deleted        bool
	Calls          []string
}

func TestRecoveryActualControlledAdapterLegacyCompatibility(t *testing.T) {
	if stdruntime.GOOS == "windows" {
		t.Skip("POSIX executable shim exercised by required Linux gate; portable classification tests run on Windows")
	}
	for _, kind := range []string{"legacy-live", "legacy-already-absent", "new-metadata-missing-incarnation", "forged-public-legacy-identity", "binding-changed", "read-failed", "delete-failed"} {
		t.Run(kind, func(t *testing.T) {
			s := compatibilitySnapshot(t)
			state := recoveryCLIState{NewMetadata: kind == "new-metadata-missing-incarnation", Absent: kind == "legacy-already-absent", ReadFailure: kind == "read-failed", DeleteFailure: kind == "delete-failed", ChangedBinding: kind == "binding-changed"}
			if kind == "forged-public-legacy-identity" {
				s.View.Request.Spec.RequestedAccess.NetworkProfiles = []string{"public-web"}
				s.View.State.EffectiveAuthority.NetworkProfiles = []string{"public-web"}
			}
			dir := t.TempDir()
			statePath := filepath.Join(dir, "fixture.json")
			raw, _ := json.Marshal(state)
			if err := os.WriteFile(statePath, raw, 0600); err != nil {
				t.Fatal(err)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			shim := []byte("#!/bin/sh\nexec \"$AGENOVA_CONSOLE_TEST_BINARY\" -test.run '^TestRecoveryKubectlProcess$' -- \"$@\"\n")
			if err = os.WriteFile(filepath.Join(dir, "kubectl"), shim, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("AGENOVA_CONSOLE_TEST_BINARY", exe)
			t.Setenv("AGENOVA_CONSOLE_KUBECTL_FIXTURE", statePath)
			t.Setenv("GORACE", "atexit_sleep_ms=0")
			store := &recoveryActivationCrashStore{snapshot: s}
			adapter := backend.NewControlled("console-fixture-only", "fixture-namespace")
			count, err := RecoverDurableWork(context.Background(), store, adapter)
			raw, readErr := os.ReadFile(statePath)
			if readErr != nil || json.Unmarshal(raw, &state) != nil {
				t.Fatal("fixture state unavailable")
			}
			legacySuccess := kind == "legacy-live" || kind == "legacy-already-absent"
			if legacySuccess {
				if err != nil || count != 1 || store.finished == nil || store.finished.Outcome.Status != "Failed" {
					t.Fatalf("actual legacy recovery failed: count=%d err=%v calls=%v", count, err, state.Calls)
				}
				for _, f := range store.finished.Facts {
					if strings.HasPrefix(f.Operation, "Network") {
						t.Fatal("invented network evidence for historical allocation")
					}
				}
				if kind == "legacy-live" && !state.Deleted {
					t.Fatal("actual cleanup did not remove original legacy claim")
				}
			} else if !errors.Is(err, ErrRecoveryRequired) || count != 0 || store.finished != nil {
				t.Fatalf("unsafe recovery accepted: count=%d err=%v calls=%v", count, err, state.Calls)
			}
			for _, call := range state.Calls {
				if strings.Contains(call, " start ") || strings.Contains(call, "apply") || strings.Contains(call, "create") {
					t.Fatal("recovery allocated or started")
				}
				if legacySuccess && strings.Contains(call, "networkpolicies") {
					t.Fatal("legacy path touched NetworkPolicy")
				}
				if !legacySuccess && kind != "delete-failed" && strings.HasPrefix(call, "delete ") {
					t.Fatal("unsafe identity caused destructive fallback")
				}
			}
		})
	}
}

func TestRecoveryKubectlProcess(t *testing.T) {
	path := os.Getenv("AGENOVA_CONSOLE_KUBECTL_FIXTURE")
	if path == "" {
		return
	}
	exit := func(code int) { os.Exit(code) }
	var args []string
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) < 5 || args[0] != "--context" || args[1] != "console-fixture-only" || args[2] != "--namespace" || args[3] != "fixture-namespace" {
		exit(90)
		return
	}
	args = args[4:]
	raw, err := os.ReadFile(path)
	if err != nil {
		exit(91)
		return
	}
	var s recoveryCLIState
	if json.Unmarshal(raw, &s) != nil {
		exit(92)
		return
	}
	s.Calls = append(s.Calls, strings.Join(args, " "))
	persist := func() {
		raw, _ := json.Marshal(s)
		if os.WriteFile(path, raw, 0600) != nil {
			exit(93)
		}
	}
	persist()
	if s.ReadFailure {
		exit(94)
		return
	}
	if args[0] == "get" && len(args) >= 3 {
		resource, name := args[1], args[2]
		present := !s.Absent && !s.Deleted && (resource == "sandboxclaims" || resource == "sandboxes" || resource == "pods")
		if strings.Contains(strings.Join(args, " "), "--ignore-not-found=true") {
			if present {
				fmt.Print(resource + "/" + name)
			}
			exit(0)
			return
		}
		if resource == "sandboxclaims" && present {
			worker := "legacy-worker"
			if s.ChangedBinding {
				worker = "other-worker"
			}
			meta := map[string]any{"name": name, "uid": "original-claim", "resourceVersion": "1"}
			spec := map[string]any{}
			if s.NewMetadata {
				meta["annotations"] = map[string]string{"agenova.io/claim-id": "legacy-recovery", "agenova.io/allocation-attempt": "new-owned-attempt"}
				spec["lifecycle"] = map[string]string{"shutdownTime": "2027-01-01T00:00:00Z"}
			}
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"metadata": meta, "spec": spec, "status": map[string]any{"sandbox": map[string]string{"name": worker}, "conditions": []any{map[string]string{"type": "Ready", "status": "True"}}}})
			exit(0)
			return
		}
	}
	if args[0] == "exec" && len(args) == 8 && args[1] == "pod/legacy-worker" && args[5] == "/agenova-workerctl" && (args[6] == "stop" || args[6] == "status") {
		fmt.Printf("state=stopped claim=%s result=fixture-stopped\n", args[7])
		exit(0)
		return
	}
	if args[0] == "delete" && len(args) >= 3 && args[1] == "sandboxclaims" {
		if s.DeleteFailure {
			exit(95)
			return
		}
		s.Deleted = true
		persist()
		exit(0)
		return
	}
	exit(96)
}
