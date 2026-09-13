// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// fakeKube simulates the upstream controller's observable state at the
// kubeClient seam. It is deliberately explicit: nothing becomes absent or
// bound unless the test says so.
type fakeKube struct {
	claims    map[string]*upstreamSandboxClaim // upstream claim name -> object
	sandboxes map[string]bool                  // sandbox name -> present

	applyErr        error
	applyCreates    bool // when applyErr != nil, whether the server still created the claim
	getErr          error
	existsErr       error
	deleteErr       error
	deleteRemoves   bool // whether delete removes the sandbox too (shutdownPolicy Delete)
	bindOnApply     string
	readyOnApply    bool
	applies, delets int
}

func newFakeKube() *fakeKube {
	return &fakeKube{claims: map[string]*upstreamSandboxClaim{}, sandboxes: map[string]bool{}, deleteRemoves: true}
}

func (k *fakeKube) applyBytes(manifest []byte) error {
	k.applies++
	var obj upstreamSandboxClaim
	if err := json.Unmarshal(manifest, &obj); err != nil {
		return err
	}
	create := func() {
		sc := &upstreamSandboxClaim{Metadata: obj.Metadata}
		if k.bindOnApply != "" {
			sc.Status.Sandbox = &upstreamSandboxRef{Name: k.bindOnApply}
			k.sandboxes[k.bindOnApply] = true
		}
		if k.readyOnApply {
			sc.Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
		}
		k.claims[obj.Metadata.Name] = sc
	}
	if k.applyErr != nil {
		if k.applyCreates {
			create()
		}
		return k.applyErr
	}
	create()
	return nil
}

func (k *fakeKube) get(_ string, name string, dst any) error {
	if k.getErr != nil {
		return k.getErr
	}
	sc, ok := k.claims[name]
	if !ok {
		return fmt.Errorf("kubectl get: exit status 1\noutput: not found")
	}
	raw, _ := json.Marshal(sc)
	return json.Unmarshal(raw, dst)
}

func (k *fakeKube) delete(resource, name string) error {
	k.delets++
	if k.deleteErr != nil {
		return k.deleteErr
	}
	if resource == "sandboxclaims" {
		if sc, ok := k.claims[name]; ok && k.deleteRemoves && sc.Status.Sandbox != nil {
			delete(k.sandboxes, sc.Status.Sandbox.Name)
		}
		delete(k.claims, name)
	}
	return nil
}

func (k *fakeKube) exists(resource, name string) (bool, error) {
	if k.existsErr != nil {
		return false, k.existsErr
	}
	switch resource {
	case "sandboxclaims":
		_, ok := k.claims[name]
		return ok, nil
	case "sandboxes":
		return k.sandboxes[name], nil
	}
	return false, fmt.Errorf("unexpected resource %s", resource)
}

func newTestAdapter(k *fakeKube) *SpikeAdapter {
	a := newSpikeAdapter(k, "agenova-test")
	a.poolRefs["tmpl-v1"] = "agenova-tmpl-tmpl-v1"
	a.pools["my-pool"] = poolEntry{upstreamTemplateName: "agenova-tmpl-tmpl-v1", upstreamPoolName: "agenova-pool-my-pool", replicas: 1}
	a.bindTimeout = 20 * time.Millisecond
	a.cleanupTimeout = 20 * time.Millisecond
	a.pollInterval = time.Millisecond
	return a
}

func allocateOK(t *testing.T, a *SpikeAdapter, claimID string) runtime.Allocation {
	t.Helper()
	alloc, err := a.Allocate(runtime.AllocateRequest{ClaimID: claimID, TemplateRef: "tmpl-v1"})
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	return alloc
}

// --- Allocate ---

func TestAllocate_bindsAndReturnsUpstreamIdentity(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	a := newTestAdapter(k)

	alloc := allocateOK(t, a, "run-1")
	want := v1alpha1.SandboxClaimBackendIdentity{Backend: BackendName, WorkerID: "sbx-1"}
	if alloc.Identity != want || alloc.ClaimID != "run-1" {
		t.Fatalf("allocation = %+v", alloc)
	}
	if _, err := a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"}); err == nil {
		t.Fatal("duplicate ClaimID must be rejected")
	}
	if k.applies != 1 {
		t.Fatalf("duplicate attempt must not re-apply; applies=%d", k.applies)
	}
}

func TestAllocate_rejectsClaimIDHeldByLegacyPathBothWays(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	a := newTestAdapter(k)
	a.claims["legacy"] = claimEntry{phase: v1alpha1.ClaimPhasePending}
	if _, err := a.Allocate(runtime.AllocateRequest{ClaimID: "legacy", TemplateRef: "tmpl-v1"}); err == nil {
		t.Fatal("Allocate must reject a ClaimID held by the legacy path")
	}

	allocateOK(t, a, "contract")
	err := a.AddClaim(runtime.BackendClaim{Metadata: v1alpha1.ObjectMeta{Name: "contract"}, Spec: runtime.BackendClaimSpec{PoolRef: "my-pool"}})
	if err == nil {
		t.Fatal("legacy AddClaim must reject a ClaimID allocated through the contract")
	}
}

// An apply failure looks identical whether the server rejected the create or
// accepted it, bound a worker and lost the response. The adapter can only read
// a worker through the claim's status, so once the claim is absent it cannot
// prove that nothing was allocated; the record must therefore be retained.
func TestAllocate_applyErrorWithUnobservedWorkerRetainsRecovery(t *testing.T) {
	k := newFakeKube()
	k.applyErr = errors.New("exit status 1")
	a := newTestAdapter(k)

	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"})
	if err == nil || !errors.Is(err, errWorkerUnknown) {
		t.Fatalf("error = %v, want errWorkerUnknown", err)
	}
	if strings.Contains(err.Error(), "compensated") {
		t.Fatalf("unconfirmed recovery must not be reported as compensated: %v", err)
	}
	entry, kept := a.allocations["run-1"]
	if !kept || entry.state != allocRecoveryPending {
		t.Fatalf("record must stay recovery-pending, got %+v kept=%t", entry, kept)
	}
	// A retry resumes recovery; it must not start a new apply while the
	// previous attempt is unconfirmed.
	_, err = a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"})
	if err == nil || k.applies != 1 {
		t.Fatalf("retry must not re-apply: err=%v applies=%d", err, k.applies)
	}
}

// The controller can bind between the status read and the delete, so a single
// empty status snapshot is not evidence that no worker exists.
func TestAllocate_lateBindingDuringRecoveryRetainsRecord(t *testing.T) {
	k := newFakeKube()
	k.applyErr = errors.New("response lost after create")
	k.applyCreates = true
	k.deleteRemoves = false
	a := newTestAdapter(k)
	a.kube = &lateBindKube{fakeKube: k}

	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "late", TemplateRef: "tmpl-v1"})
	if err == nil || !errors.Is(err, errWorkerUnknown) {
		t.Fatalf("error = %v, want errWorkerUnknown", err)
	}
	if !k.sandboxes["late-worker"] {
		t.Fatal("probe invalid: the late worker should still exist")
	}
	if _, kept := a.allocations["late"]; !kept {
		t.Fatal("record dropped while an unobserved worker remains")
	}
	if k.delets != 0 {
		t.Fatal("an unknown worker must not trigger a destructive delete")
	}
}

// lateBindKube binds immediately after returning an empty status snapshot.
// This does not depend on the adapter attempting a destructive delete.
type lateBindKube struct {
	*fakeKube
}

func (k *lateBindKube) get(resource, name string, dst any) error {
	if err := k.fakeKube.get(resource, name, dst); err != nil {
		return err
	}
	if sc := k.claims[name]; sc != nil && sc.Status.Sandbox == nil {
		sc.Status.Sandbox = &upstreamSandboxRef{Name: "late-worker"}
		k.sandboxes["late-worker"] = true
	}
	return nil
}

func TestAllocate_applyErrorButServerCreatedAndBound_confirmsWorkerRemoval(t *testing.T) {
	k := newFakeKube()
	k.applyErr = errors.New("timeout waiting for response")
	k.applyCreates = true
	k.bindOnApply = "sbx-hidden"
	a := newTestAdapter(k)

	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"})
	if err == nil || !strings.Contains(err.Error(), "compensated") {
		t.Fatalf("error = %v, want compensated failure", err)
	}
	if k.delets != 1 {
		t.Fatalf("compensation must delete the created claim; deletes=%d", k.delets)
	}
	if k.sandboxes["sbx-hidden"] {
		t.Fatal("test fake: sandbox should be gone after claim delete")
	}
	if _, kept := a.allocations["run-1"]; kept {
		t.Fatal("confirmed compensation must drop the record")
	}
}

func TestAllocate_bindReadFailure_leavesRecoveryPendingAndRetryResumesWithoutReapply(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	k.getErr = errors.New("connection refused")
	k.existsErr = errors.New("connection refused")
	a := newTestAdapter(k)

	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"})
	if err == nil || !strings.Contains(err.Error(), "recovery unconfirmed") {
		t.Fatalf("error = %v, want recovery unconfirmed", err)
	}
	entry := a.allocations["run-1"]
	if entry == nil || entry.state != allocRecoveryPending {
		t.Fatalf("record must be kept in recovery-pending, got %+v", entry)
	}
	if k.applies != 1 {
		t.Fatalf("applies = %d", k.applies)
	}

	// Still failing: retry continues recovery and never re-applies.
	_, err = a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"})
	if err == nil || k.applies != 1 {
		t.Fatalf("retry must resume recovery without re-apply: err=%v applies=%d", err, k.applies)
	}

	// Cluster reachable again: recovery learns the worker, deletes, confirms, then a fresh apply succeeds.
	k.getErr, k.existsErr = nil, nil
	k.bindOnApply = "sbx-2"
	alloc := allocateOK(t, a, "run-1")
	if alloc.Identity.WorkerID != "sbx-2" || k.applies != 2 || k.delets != 1 {
		t.Fatalf("recovered allocation = %+v applies=%d deletes=%d", alloc, k.applies, k.delets)
	}
	if k.sandboxes["sbx-1"] {
		t.Fatal("previous worker must be confirmed removed before reuse")
	}
}

func TestAllocate_claimGoneButWorkerRemains_staysUnconfirmed(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	k.deleteRemoves = false // controller does not remove the sandbox with the claim
	k.getErr = errors.New("connection refused")
	a := newTestAdapter(k)

	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"})
	if err == nil || !strings.Contains(err.Error(), "recovery unconfirmed") {
		t.Fatalf("error = %v", err)
	}
	if a.allocations["run-1"].state != allocRecoveryPending {
		t.Fatal("record must stay recovery-pending")
	}
	k.getErr = nil
	_, err = a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"})
	if err == nil || !errors.Is(err, errCleanupUnconfirmed) {
		t.Fatalf("worker still present must keep recovery unconfirmed, got %v", err)
	}
	if k.applies != 1 {
		t.Fatalf("no new apply while worker unconfirmed; applies=%d", k.applies)
	}
	if _, kept := a.allocations["run-1"]; !kept {
		t.Fatal("record must be retained")
	}
}

func TestAllocate_bindTimeoutRetainsRecordUntilWorkerIsProvenAbsent(t *testing.T) {
	k := newFakeKube() // never binds
	a := newTestAdapter(k)
	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "run-1", TemplateRef: "tmpl-v1"})
	if err == nil || !strings.Contains(err.Error(), "bind timeout") {
		t.Fatalf("error = %v, want bind timeout", err)
	}
	if !errors.Is(err, errWorkerUnknown) {
		t.Fatalf("error = %v, want errWorkerUnknown: an unbound claim may still bind before deletion", err)
	}
	if len(k.claims) != 1 || k.delets != 0 {
		t.Fatal("keep the unbound claim until its worker can be checked safely")
	}
	if _, kept := a.allocations["run-1"]; !kept {
		t.Fatal("record must be retained while the worker was never observed")
	}
	// Keeping the claim also preserves the chance to learn a late binding.
	k.claims[resourceName("claim", "run-1")].Status.Sandbox = &upstreamSandboxRef{Name: "late-worker"}
	k.sandboxes["late-worker"] = true
	k.bindOnApply = "new-worker"
	alloc := allocateOK(t, a, "run-1")
	if alloc.Identity.WorkerID != "new-worker" || k.sandboxes["late-worker"] || k.delets != 1 {
		t.Fatalf("retry did not confirm the old worker before reallocating: %+v deletes=%d", alloc, k.delets)
	}
}

// A worker that already belongs to another claim must never be released by
// recovering the conflicting attempt: with shutdownPolicy Delete, deleting the
// second claim can destroy the first claim's worker.
func TestAllocate_identityConflictLeavesExistingWorkerIntact(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "shared-worker"
	a := newTestAdapter(k)

	first := allocateOK(t, a, "first")
	deletesBefore := k.delets

	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "second", TemplateRef: "tmpl-v1"})
	if err == nil || !errors.Is(err, errIdentityConflict) {
		t.Fatalf("error = %v, want errIdentityConflict", err)
	}
	if !k.sandboxes[first.Identity.WorkerID] {
		t.Fatal("recovering the conflicting attempt destroyed the first claim's worker")
	}
	if k.delets != deletesBefore {
		t.Fatalf("conflicting attempt must not delete anything; deletes %d -> %d", deletesBefore, k.delets)
	}
	// The first allocation is still usable and still owns the identity.
	if obs, err := a.Observe(first.Identity); err != nil || obs.ClaimID != "first" {
		t.Fatalf("first allocation disturbed: obs=%+v err=%v", obs, err)
	}
	if a.byWorker["shared-worker"] != "first" {
		t.Fatalf("worker ownership changed to %q", a.byWorker["shared-worker"])
	}
	// The conflict is terminal for the second ClaimID and never recovered
	// destructively.
	entry := a.allocations["second"]
	if entry == nil || entry.state != allocConflicted {
		t.Fatalf("second attempt state = %+v, want allocConflicted", entry)
	}
	_, err = a.Allocate(runtime.AllocateRequest{ClaimID: "second", TemplateRef: "tmpl-v1"})
	if err == nil || !errors.Is(err, errIdentityConflict) {
		t.Fatalf("retry error = %v, want errIdentityConflict", err)
	}
	if k.delets != deletesBefore {
		t.Fatal("retry of a conflicted attempt must stay non-destructive")
	}
}

// Two callers retrying the same recovery-pending ClaimID must not read and
// write the attempt's state concurrently, and only one recovery may run.
func TestAllocate_concurrentRecoveryOfSameClaimIsSerialised(t *testing.T) {
	a := newTestAdapter(newFakeKube())
	a.kube = unreachableKube{}
	a.allocations["retry"] = &allocationEntry{
		claimID:           "retry",
		upstreamClaimName: resourceName("claim", "retry"),
		state:             allocRecoveryPending,
		applyConfirmed:    true,
	}

	begin := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-begin
			for j := 0; j < 200; j++ {
				if _, err := a.Allocate(runtime.AllocateRequest{ClaimID: "retry", TemplateRef: "tmpl-v1"}); err == nil {
					t.Error("unreachable cluster must not yield a successful allocation")
					return
				}
			}
		}()
	}
	close(begin)
	wg.Wait()

	entry := a.allocations["retry"]
	if entry == nil || entry.state != allocRecoveryPending {
		t.Fatalf("entry = %+v, want retained recovery-pending", entry)
	}
	if entry.recovering {
		t.Fatal("recovery guard was left set")
	}
}

// unreachableKube fails every call with no shared mutable state, so a race
// report can only come from the adapter itself.
type unreachableKube struct{}

func (unreachableKube) applyBytes([]byte) error       { return errors.New("unreachable") }
func (unreachableKube) get(string, string, any) error { return errors.New("unreachable") }
func (unreachableKube) delete(string, string) error   { return errors.New("unreachable") }
func (unreachableKube) exists(string, string) (bool, error) {
	return false, errors.New("unreachable")
}

// --- Observe / Start / Terminate ---

func TestObserve_readinessIsBoundLevelEvidenceOnly(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	a := newTestAdapter(k)
	alloc := allocateOK(t, a, "run-1")

	obs, err := a.Observe(alloc.Identity)
	if err != nil || obs.Ready {
		t.Fatalf("not yet ready: obs=%+v err=%v", obs, err)
	}
	k.claims[resourceName("claim", "run-1")].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	obs, err = a.Observe(alloc.Identity)
	if err != nil || !obs.Ready || obs.Released || obs.Replaced {
		t.Fatalf("ready observation = %+v err=%v", obs, err)
	}
	// Readiness does not unlock Start on this backend.
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("start error = %v, want ErrUnsupported", err)
	}
	if err := a.Terminate(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("terminate error = %v, want ErrUnsupported", err)
	}
}

// Readiness belongs to whichever worker the claim currently carries. A
// different worker's readiness must never be attributed to a stale identity.
func TestObserve_rejectsReadinessOfADifferentWorker(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "original-worker"
	a := newTestAdapter(k)
	alloc := allocateOK(t, a, "observe")

	sc := k.claims[resourceName("claim", "observe")]
	sc.Status.Sandbox = &upstreamSandboxRef{Name: "different-worker"}
	sc.Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}

	obs, err := a.Observe(alloc.Identity)
	if !errors.Is(err, errIdentityMismatch) {
		t.Fatalf("obs=%+v err=%v, want errIdentityMismatch", obs, err)
	}
	if obs.Ready {
		t.Fatal("another worker's readiness was attributed to this identity")
	}

	// A claim that lost its worker is equally inconsistent, not "not ready".
	sc.Status.Sandbox = nil
	if _, err := a.Observe(alloc.Identity); !errors.Is(err, errIdentityMismatch) {
		t.Fatalf("missing upstream worker err = %v, want errIdentityMismatch", err)
	}
}

func TestObserve_queryFailurePropagates(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	a := newTestAdapter(k)
	alloc := allocateOK(t, a, "run-1")
	k.existsErr = errors.New("connection refused")
	if _, err := a.Observe(alloc.Identity); err == nil {
		t.Fatal("observe must surface real query failures")
	}
}

func TestIdentity_unknownAndForeignBackendRejected(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	a := newTestAdapter(k)
	allocateOK(t, a, "run-1")
	for _, id := range []v1alpha1.SandboxClaimBackendIdentity{
		{Backend: BackendName, WorkerID: "sbx-other"},
		{Backend: "memory", WorkerID: "sbx-1"},
		{},
	} {
		if _, err := a.Observe(id); !errors.Is(err, runtime.ErrUnknownIdentity) {
			t.Fatalf("observe %+v: %v", id, err)
		}
		if err := a.Start(id); !errors.Is(err, runtime.ErrUnknownIdentity) {
			t.Fatalf("start %+v: %v", id, err)
		}
		if err := a.Terminate(id); !errors.Is(err, runtime.ErrUnknownIdentity) {
			t.Fatalf("terminate %+v: %v", id, err)
		}
		if _, err := a.Cleanup(id); !errors.Is(err, runtime.ErrUnknownIdentity) {
			t.Fatalf("cleanup %+v: %v", id, err)
		}
	}
}

// --- Cleanup ---

func TestCleanup_releasedOnlyWhenClaimAndSandboxConfirmedAbsent(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	a := newTestAdapter(k)
	alloc := allocateOK(t, a, "run-1")

	res, err := a.Cleanup(alloc.Identity)
	if err != nil || !res.Released || res.Replaced {
		t.Fatalf("cleanup = %+v err=%v", res, err)
	}
	again, err := a.Cleanup(alloc.Identity)
	if err != nil || again != res {
		t.Fatalf("second cleanup = %+v err=%v, want %+v", again, err, res)
	}
	obs, err := a.Observe(alloc.Identity)
	if err != nil || !obs.Released || obs.Ready || obs.ClaimID != "run-1" {
		t.Fatalf("observe after release = %+v err=%v", obs, err)
	}
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrReleased) {
		t.Fatalf("start after release = %v, want ErrReleased (checked before unsupported)", err)
	}
	if err := a.Terminate(alloc.Identity); !errors.Is(err, runtime.ErrReleased) {
		t.Fatalf("terminate after release = %v, want ErrReleased", err)
	}
}

func TestCleanup_sandboxStillPresentIsNotReleased(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	k.deleteRemoves = false
	a := newTestAdapter(k)
	alloc := allocateOK(t, a, "run-1")

	res, err := a.Cleanup(alloc.Identity)
	if err == nil || !errors.Is(err, errCleanupUnconfirmed) {
		t.Fatalf("expected unconfirmed cleanup, got res=%+v err=%v", res, err)
	}
	if res.Released || res.Identity != alloc.Identity {
		t.Fatalf("unconfirmed cleanup must not report released and must keep identity: %+v", res)
	}
	obs, _ := a.Observe(alloc.Identity)
	if obs.Released {
		t.Fatal("record must not be marked released")
	}
	// Once the controller removes the sandbox, a retry confirms.
	delete(k.sandboxes, "sbx-1")
	res, err = a.Cleanup(alloc.Identity)
	if err != nil || !res.Released {
		t.Fatalf("retry cleanup = %+v err=%v", res, err)
	}
}

func TestCleanup_queryFailureIsNotReleaseEvidence(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	a := newTestAdapter(k)
	alloc := allocateOK(t, a, "run-1")
	k.existsErr = errors.New("exec plugin: executable file not found in $PATH")

	res, err := a.Cleanup(alloc.Identity)
	if err == nil || res.Released {
		t.Fatalf("query failure must not produce release: res=%+v err=%v", res, err)
	}
	if errors.Is(err, errCleanupUnconfirmed) {
		t.Fatalf("query failure should surface as the query error, not a timeout: %v", err)
	}
}

func TestCleanup_deleteFailureReturnsIdentityAndNoRelease(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	a := newTestAdapter(k)
	alloc := allocateOK(t, a, "run-1")
	k.deleteErr = errors.New("forbidden")

	res, err := a.Cleanup(alloc.Identity)
	if err == nil || res.Released || res.Identity != alloc.Identity {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestCleanup_validatesCurrentBindingBeforeDelete(t *testing.T) {
	for _, name := range []string{"different worker", "missing binding", "empty worker", "read failure"} {
		t.Run(name, func(t *testing.T) {
			k := newFakeKube()
			k.bindOnApply = "first-worker"
			a := newTestAdapter(k)
			first := allocateOK(t, a, "first")
			k.bindOnApply = "second-worker"
			second := allocateOK(t, a, "second")
			sc := k.claims[resourceName("claim", "first")]
			want := errIdentityMismatch
			switch name {
			case "different worker":
				sc.Status.Sandbox = &upstreamSandboxRef{Name: second.Identity.WorkerID}
			case "missing binding":
				sc.Status.Sandbox = nil
			case "empty worker":
				sc.Status.Sandbox = &upstreamSandboxRef{}
			case "read failure":
				want = errors.New("binding query unavailable")
				k.getErr = want
			}

			res, err := a.Cleanup(first.Identity)
			if !errors.Is(err, want) || res.Identity != first.Identity || res.Released || res.Replaced {
				t.Errorf("unverified binding must reject cleanup with identity intact: %+v %v", res, err)
			}
			if k.delets != 0 || !k.sandboxes[first.Identity.WorkerID] || !k.sandboxes[second.Identity.WorkerID] {
				t.Fatalf("unverified binding caused destructive cleanup: deletes=%d first=%t second=%t", k.delets, k.sandboxes[first.Identity.WorkerID], k.sandboxes[second.Identity.WorkerID])
			}
			if a.isReleased(a.allocations["first"]) {
				t.Fatal("unverified cleanup was recorded as released")
			}

			// A later retry may proceed only after the original binding can
			// be verified again. The other allocation remains untouched.
			k.getErr = nil
			sc.Status.Sandbox = &upstreamSandboxRef{Name: first.Identity.WorkerID}
			res, err = a.Cleanup(first.Identity)
			if err != nil || !res.Released || !k.sandboxes[second.Identity.WorkerID] {
				t.Fatalf("retry after binding verification: %+v %v", res, err)
			}
		})
	}
}

// TestReducedContractSupportMatrix documents, per operation, what this spike
// adapter supports against a simulated controller. It is NOT the reference
// contract suite: Start and Terminate are unsupported by design and real
// cluster evidence is a separate gate.
func TestReducedContractSupportMatrix(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	k.readyOnApply = true
	a := newTestAdapter(k)
	alloc := allocateOK(t, a, "matrix")
	if alloc.Filesystem.EvidenceLevel != runtime.FilesystemEvidenceUnsupported || alloc.Filesystem.WorkingDirectory != "" {
		t.Fatalf("filesystem boundary must remain explicitly unsupported until #48/#51 verify it: %+v", alloc.Filesystem)
	}

	obs, err := a.Observe(alloc.Identity)
	if err != nil || !obs.Ready {
		t.Fatalf("Observe: supported (ready) expected, got %+v %v", obs, err)
	}
	if obs.Filesystem != alloc.Filesystem {
		t.Fatalf("filesystem capability drifted between allocation and observation: %+v / %+v", alloc.Filesystem, obs.Filesystem)
	}
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("Start: unsupported expected, got %v", err)
	}
	if err := a.Terminate(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("Terminate: unsupported expected, got %v", err)
	}
	res, err := a.Cleanup(alloc.Identity)
	if err != nil || !res.Released {
		t.Fatalf("Cleanup: supported expected, got %+v %v", res, err)
	}
	t.Log("support matrix: Allocate=supported Observe=supported(readiness only) Start=unsupported Terminate=unsupported Cleanup=supported(release; replacement not observable) Filesystem=unsupported; real-cluster evidence=blocked (no verified kube context)")
}

// Ownership checks must apply to worker identities
// first learned during compensation, not just successful waitForSandbox.
func TestAllocate_recoveryIdentityConflictProtectsExistingWorker(t *testing.T) {
	for _, path := range []string{"apply-response-error", "bind-status-error", "resumed-recovery"} {
		t.Run(path, func(t *testing.T) {
			k := newFakeKube()
			k.bindOnApply = "shared-worker"
			a := newTestAdapter(k)
			first := allocateOK(t, a, "first")
			deletesBefore := k.delets
			req := runtime.AllocateRequest{ClaimID: "second", TemplateRef: "tmpl-v1"}
			var err error
			switch path {
			case "apply-response-error":
				k.applyCreates = true
				k.applyErr = errors.New("response lost after server accepted and bound second claim")
				_, err = a.Allocate(req)
			case "bind-status-error":
				a.kube = &firstGetFailsKube{fakeKube: k, failNext: true}
				_, err = a.Allocate(req)
			case "resumed-recovery":
				k.getErr = errors.New("temporary status failure")
				k.existsErr = errors.New("temporary recovery failure")
				if _, initialErr := a.Allocate(req); initialErr == nil {
					t.Fatal("expected initial allocation to remain recovery-pending")
				}
				k.getErr, k.existsErr = nil, nil
				k.bindOnApply = "next-worker"
				_, err = a.Allocate(req)
			}
			t.Logf("path=%s result=%v deletes=%d first-worker-present=%t", path, err, k.delets-deletesBefore, k.sandboxes[first.Identity.WorkerID])
			if !errors.Is(err, errIdentityConflict) {
				t.Errorf("recovery must reject the foreign worker with errIdentityConflict; got %v", err)
			}
			if k.delets != deletesBefore || !k.sandboxes[first.Identity.WorkerID] {
				t.Error("recovery of second claim deleted resources while its worker belongs to first")
			}
			if entry := a.allocations["second"]; entry == nil || entry.state != allocConflicted {
				t.Errorf("conflicting attempt must be retained without destructive compensation: %+v", entry)
			}
		})
	}
}

// The bind read fails once; recovery can then read the real assigned worker.
type firstGetFailsKube struct {
	*fakeKube
	failNext bool
}

func (k *firstGetFailsKube) get(resource, name string, dst any) error {
	if k.failNext {
		k.failNext = false
		return errors.New("temporary bind status query failure")
	}
	return k.fakeKube.get(resource, name, dst)
}

func TestAllocate_recoveryReservationSurvivesDeleteFailure(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "recovering-worker"
	k.applyCreates = true
	k.applyErr = errors.New("response lost")
	k.deleteErr = errors.New("forbidden")
	a := newTestAdapter(k)
	if _, err := a.Allocate(runtime.AllocateRequest{ClaimID: "recovering", TemplateRef: "tmpl-v1"}); err == nil {
		t.Fatal("expected failed compensation")
	}
	if a.byWorker["recovering-worker"] != "recovering" {
		t.Fatal("failed cleanup lost the worker reservation")
	}
	assertUnissuedIdentity(t, a, "recovering-worker")

	k.applyErr = nil
	before := k.delets
	if _, err := a.Allocate(runtime.AllocateRequest{ClaimID: "other", TemplateRef: "tmpl-v1"}); !errors.Is(err, errIdentityConflict) {
		t.Fatalf("allocation must not acquire a worker still being recovered: %v", err)
	}
	if k.delets != before || !k.sandboxes["recovering-worker"] {
		t.Fatal("conflict destroyed the reserved worker")
	}

	k.deleteErr = nil
	k.bindOnApply = "new-worker"
	alloc := allocateOK(t, a, "recovering")
	if _, held := a.byWorker["recovering-worker"]; held || alloc.Identity.WorkerID != "new-worker" {
		t.Fatalf("confirmed recovery did not release its reservation: %+v", alloc)
	}
}

func TestAllocate_recoveryRejectsChangedBindingBeforeDelete(t *testing.T) {
	for _, rebound := range []string{"other-worker", ""} {
		t.Run("binding="+rebound, func(t *testing.T) {
			k := newFakeKube()
			k.bindOnApply = "recorded-worker"
			k.applyCreates = true
			k.applyErr = errors.New("response lost")
			k.deleteErr = errors.New("forbidden")
			a := newTestAdapter(k)
			req := runtime.AllocateRequest{ClaimID: "recovering", TemplateRef: "tmpl-v1"}
			if _, err := a.Allocate(req); err == nil {
				t.Fatal("expected failed compensation")
			}
			k.claims[resourceName("claim", req.ClaimID)].Status.Sandbox = &upstreamSandboxRef{Name: rebound}
			k.sandboxes["other-worker"] = true
			k.deleteErr = nil
			before := k.delets
			_, err := a.Allocate(req)
			want := errIdentityMismatch
			if rebound == "" {
				want = errWorkerUnknown
			}
			if !errors.Is(err, want) || k.delets != before || !k.sandboxes["other-worker"] {
				t.Fatalf("changed binding must not cause deletion: error=%v deletes=%d", err, k.delets-before)
			}
		})
	}
}

// Once recovery has reserved a worker and is waiting on delete, a concurrent
// allocation of that worker must fail without deleting either claim.
func TestAllocate_concurrentBindingCannotAcquireRecoveringWorker(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "recovering-worker"
	k.applyCreates = true
	k.applyErr = errors.New("response lost")
	a := newTestAdapter(k)
	block := &blockingRecoveryDeleteKube{fakeKube: k, entered: make(chan struct{}), release: make(chan struct{})}
	a.kube = block
	done := make(chan error, 1)
	go func() {
		_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "recovering", TemplateRef: "tmpl-v1"})
		done <- err
	}()
	defer func() {
		close(block.release)
		if err := <-done; err == nil || !strings.Contains(err.Error(), "compensated") {
			t.Errorf("first attempt should finish confirmed compensation: %v", err)
		}
	}()
	select {
	case <-block.entered:
	case <-time.After(time.Second):
		t.Fatal("recovery never reached delete")
	}
	// The recovery goroutine is paused before touching the fake maps again;
	// the channel handoff makes this test safe under the race detector.
	assertUnissuedIdentity(t, a, "recovering-worker")
	k.applyErr = nil
	_, err := a.Allocate(runtime.AllocateRequest{ClaimID: "other", TemplateRef: "tmpl-v1"})
	if !errors.Is(err, errIdentityConflict) || k.delets != 0 {
		t.Fatalf("concurrent binding must fail without deleting: error=%v deletes=%d", err, k.delets)
	}
}

func assertUnissuedIdentity(t *testing.T, a *SpikeAdapter, workerID string) {
	t.Helper()
	id := v1alpha1.SandboxClaimBackendIdentity{Backend: BackendName, WorkerID: workerID}
	if _, err := a.Observe(id); !errors.Is(err, runtime.ErrUnknownIdentity) {
		t.Fatalf("recovery reservation must not expose observation: %v", err)
	}
	if err := a.Start(id); !errors.Is(err, runtime.ErrUnknownIdentity) {
		t.Fatalf("recovery reservation must not expose start: %v", err)
	}
	if err := a.Terminate(id); !errors.Is(err, runtime.ErrUnknownIdentity) {
		t.Fatalf("recovery reservation must not expose termination: %v", err)
	}
	if _, err := a.Cleanup(id); !errors.Is(err, runtime.ErrUnknownIdentity) {
		t.Fatalf("recovery reservation must not expose cleanup: %v", err)
	}
}

type blockingRecoveryDeleteKube struct {
	*fakeKube
	entered chan struct{}
	release chan struct{}
}

func (k *blockingRecoveryDeleteKube) delete(resource, name string) error {
	if name == resourceName("claim", "recovering") {
		close(k.entered)
		<-k.release
	}
	return k.fakeKube.delete(resource, name)
}
