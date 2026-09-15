// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

type fakeWorkerControl struct {
	claim     string
	worker    string
	state     string
	starts    int
	stops     int
	failStart bool
	failStop  bool
}

func (f *fakeWorkerControl) execWorker(workerID string, args ...string) (string, error) {
	claimToken := controlClaimToken(f.claim)
	if workerID != f.worker || len(args) != 2 || args[1] != claimToken {
		return "", fmt.Errorf("wrong worker or claim: %s %v", workerID, args)
	}
	switch args[0] {
	case "start":
		f.starts++
		if f.failStart {
			return "", errors.New("start acknowledgement lost")
		}
		f.state = "running"
		return "state=started claim=" + claimToken + " result=probe-ok\n", nil
	case "stop":
		f.stops++
		if f.failStop {
			return "", errors.New("stop acknowledgement lost")
		}
		f.state = "stopped"
		return "state=stopped claim=" + claimToken + " result=probe-ok\n", nil
	case "status":
		return "state=" + f.state + " claim=" + claimToken + " result=probe-ok\n", nil
	}
	return "", fmt.Errorf("unexpected command %s", args[0])
}

func controlledFixture(t *testing.T) (*ControlledAdapter, *fakeKube, *fakeWorkerControl, runtime.Allocation) {
	t.Helper()
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	f := &fakeWorkerControl{claim: "run-1", worker: "sbx-1", state: "idle"}
	a := newControlledAdapter(newTestAdapter(k), f)
	alloc := allocateOK(t, a.SpikeAdapter, f.claim)
	return a, k, f, alloc
}

func TestControlledAdapter_realStartStopAreSeparateFromReadyAndDelete(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrNotReady) {
		t.Fatalf("start before Ready = %v", err)
	}
	if f.starts != 0 {
		t.Fatal("worker started before Ready")
	}
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	obs, err := a.Observe(alloc.Identity)
	if err != nil || !obs.Ready || f.starts != 0 {
		t.Fatalf("Ready must be infrastructure-only: obs=%+v err=%v starts=%d", obs, err, f.starts)
	}
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if f.starts != 1 || f.state != "running" {
		t.Fatalf("no work-start acknowledgement: %+v", f)
	}
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrAlreadyStarted) {
		t.Fatalf("duplicate Start = %v", err)
	}
	if _, err := a.Cleanup(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) || k.delets != 0 {
		t.Fatalf("cleanup must refuse running work: err=%v deletes=%d", err, k.delets)
	}
	if err := a.Terminate(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if f.stops != 1 || f.state != "stopped" || k.delets != 0 {
		t.Fatalf("stop must precede deletion: worker=%+v deletes=%d", f, k.delets)
	}
	if err := a.Terminate(alloc.Identity); err != nil || f.stops != 1 {
		t.Fatalf("idempotent Terminate = %v, stops=%d", err, f.stops)
	}
	result, err := a.Cleanup(alloc.Identity)
	if err != nil || !result.Released {
		t.Fatalf("cleanup = %+v, %v", result, err)
	}
	obs, err = a.Observe(alloc.Identity)
	if err != nil || !obs.Released || obs.Ready {
		t.Fatalf("released observation = %+v, %v", obs, err)
	}
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrReleased) {
		t.Fatalf("start after cleanup = %v", err)
	}
}

func TestControlledAdapter_wrongIdentityAndStaleBindingFailClosed(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	foreign := v1alpha1.SandboxClaimBackendIdentity{Backend: BackendName, WorkerID: "other"}
	if err := a.Start(foreign); !errors.Is(err, runtime.ErrUnknownIdentity) {
		t.Fatalf("foreign Start = %v", err)
	}
	k.claims[resourceName("claim", f.claim)].Status.Sandbox.Name = "rebound"
	if err := a.Start(alloc.Identity); !errors.Is(err, errIdentityMismatch) {
		t.Fatalf("stale Start = %v", err)
	}
	if f.starts != 0 {
		t.Fatal("stale worker was started")
	}
}

func TestControlledAdapter_uncertainStopCannotBeCleanedUp(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	f.failStop = true
	if err := a.Terminate(alloc.Identity); err == nil {
		t.Fatal("stop failure must propagate")
	}
	if _, err := a.Cleanup(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) || k.delets != 0 {
		t.Fatalf("uncertain stop must block deletion: err=%v deletes=%d", err, k.delets)
	}
	f.failStop = false
	if err := a.Terminate(alloc.Identity); err != nil {
		t.Fatalf("retry stop: %v", err)
	}
}

func TestControlledAdapter_uncertainStartRequiresConfirmedCancelBeforeCleanup(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	f.failStart = true
	if err := a.Start(alloc.Identity); err == nil {
		t.Fatal("lost Start acknowledgement must fail")
	}
	if _, err := a.Cleanup(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) || k.delets != 0 {
		t.Fatalf("uncertain Start allowed cleanup: err=%v deletes=%d", err, k.delets)
	}
	if err := a.Terminate(alloc.Identity); err != nil {
		t.Fatalf("explicit stop/cancel should recover uncertain Start: %v", err)
	}
	if res, err := a.Cleanup(alloc.Identity); err != nil || !res.Released {
		t.Fatalf("cleanup after confirmed cancel: %+v %v", res, err)
	}
}

func TestControlledAdapter_prestartTerminationCancelsStart(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	if err := a.Terminate(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrTerminated) {
		t.Fatalf("Start after prestart cancel = %v", err)
	}
	if f.starts != 0 || f.stops != 0 {
		t.Fatal("prestart cancel should not signal worker task")
	}
	delete(k.claims, resourceName("claim", f.claim))
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrTerminated) {
		t.Fatalf("Start after cancellation depended on mutable backend readiness: %v", err)
	}
}

func TestControlledAdapter_encodesEverySystemIssuedClaimForWorkerProtocol(t *testing.T) {
	claimID := "claim:支付 timeout / " + strings.Repeat("long name ", 24)
	k := newFakeKube()
	k.bindOnApply = "sbx-1"
	f := &fakeWorkerControl{claim: claimID, worker: "sbx-1", state: "idle"}
	a := newControlledAdapter(newTestAdapter(k), f)
	alloc := allocateOK(t, a.SpikeAdapter, claimID)
	k.claims[resourceName("claim", claimID)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatalf("Start rejected a shared-contract claim ID: %v", err)
	}
	if err := a.Terminate(alloc.Identity); err != nil {
		t.Fatalf("Terminate rejected a shared-contract claim ID: %v", err)
	}
}

type blockedWorkerStart struct {
	*fakeWorkerControl
	entered chan struct{}
	release chan struct{}
}

func (c *blockedWorkerStart) execWorker(workerID string, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "start" {
		close(c.entered)
		<-c.release
	}
	return c.fakeWorkerControl.execWorker(workerID, args...)
}

func TestControlledAdapter_terminateWaitsForInFlightStartThenStops(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	block := &blockedWorkerStart{fakeWorkerControl: f, entered: make(chan struct{}), release: make(chan struct{})}
	a.control = block
	started := make(chan error, 1)
	go func() { started <- a.Start(alloc.Identity) }()
	select {
	case <-block.entered:
	case <-time.After(time.Second):
		t.Fatal("Start did not enter worker protocol")
	}
	terminated := make(chan error, 1)
	go func() { terminated <- a.Terminate(alloc.Identity) }()
	select {
	case err := <-terminated:
		t.Fatalf("Terminate returned while Start could still begin work: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(block.release)
	if err := <-started; err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := <-terminated; err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if f.state != "stopped" || f.starts != 1 || f.stops != 1 {
		t.Fatalf("in-flight Start was not stopped exactly once: %+v", f)
	}
}

func TestKubectlWorkerControl_usesExactContextNamespaceAndPod(t *testing.T) {
	r := newKubectlRunner("kind-agenova-k8s-lab", "agenova-e2e")
	r.command = func(_ context.Context, _ []byte, args ...string) ([]byte, error) {
		got := strings.Join(args, " ")
		want := "--context kind-agenova-k8s-lab --namespace agenova-e2e exec pod/sbx-1 -c agent -- /agenova-workerctl start run-1"
		if got != want {
			t.Fatalf("args = %q, want %q", got, want)
		}
		return []byte("state=started claim=run-1 result=probe-ok\n"), nil
	}
	out, err := r.execWorker("sbx-1", "start", "run-1")
	if err != nil || out != "state=started claim=run-1 result=probe-ok\n" {
		t.Fatalf("output=%q err=%v", out, err)
	}
	if _, err := r.execWorker("../other", "start", "run-1"); err == nil {
		t.Fatal("unsafe worker name accepted")
	}
}

type blockedControlDelete struct {
	*fakeKube
	entered chan struct{}
	release chan struct{}
}

func (k *blockedControlDelete) delete(resource, name string) error {
	if resource == "sandboxclaims" {
		close(k.entered)
		<-k.release
	}
	return k.fakeKube.delete(resource, name)
}

func TestControlledAdapter_cleanupSerializesWithStart(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	block := &blockedControlDelete{fakeKube: k, entered: make(chan struct{}), release: make(chan struct{})}
	a.SpikeAdapter.kube = block
	done := make(chan error, 1)
	go func() {
		_, err := a.Cleanup(alloc.Identity)
		done <- err
	}()
	select {
	case <-block.entered:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not enter deletion")
	}
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrReleased) {
		t.Fatalf("Start raced with cleanup: %v", err)
	}
	if err := a.Terminate(alloc.Identity); !errors.Is(err, runtime.ErrReleased) {
		t.Fatalf("Terminate raced with cleanup: %v", err)
	}
	if f.starts != 0 {
		t.Fatal("worker started after cleanup began")
	}
	close(block.release)
	if err := <-done; err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

func TestControlledAdapter_failedCleanupNeverReopensStart(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	k.deleteErr = errors.New("delete response uncertain")
	if _, err := a.Cleanup(alloc.Identity); err == nil {
		t.Fatal("expected cleanup failure")
	}
	if err := a.Start(alloc.Identity); !errors.Is(err, runtime.ErrReleased) || f.starts != 0 {
		t.Fatalf("failed cleanup reopened Start: err=%v starts=%d", err, f.starts)
	}
	k.deleteErr = nil
	if res, err := a.Cleanup(alloc.Identity); err != nil || !res.Released {
		t.Fatalf("cleanup retry = %+v %v", res, err)
	}
}

type blockedWorkerStop struct {
	*fakeWorkerControl
	entered chan struct{}
	release chan struct{}
}

func (c *blockedWorkerStop) execWorker(workerID string, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "stop" {
		close(c.entered)
		<-c.release
	}
	return c.fakeWorkerControl.execWorker(workerID, args...)
}

func TestControlledAdapter_concurrentTerminateCannotClobberStop(t *testing.T) {
	a, k, f, alloc := controlledFixture(t)
	k.claims[resourceName("claim", f.claim)].Status.Conditions = []upstreamCondition{{Type: conditionTypeReady, Status: conditionStatusTrue}}
	if err := a.Start(alloc.Identity); err != nil {
		t.Fatal(err)
	}
	block := &blockedWorkerStop{fakeWorkerControl: f, entered: make(chan struct{}), release: make(chan struct{})}
	a.control = block
	done := make(chan error, 1)
	go func() { done <- a.Terminate(alloc.Identity) }()
	select {
	case <-block.entered:
	case <-time.After(time.Second):
		t.Fatal("Terminate did not enter worker stop")
	}
	second := make(chan error, 1)
	go func() { second <- a.Terminate(alloc.Identity) }()
	select {
	case err := <-second:
		t.Fatalf("concurrent Terminate returned before stop was confirmed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if _, err := a.Cleanup(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("cleanup while stop in flight = %v", err)
	}
	close(block.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatalf("idempotent concurrent Terminate: %v", err)
	}
	if f.stops != 1 {
		t.Fatalf("stop invoked %d times", f.stops)
	}
	if res, err := a.Cleanup(alloc.Identity); err != nil || !res.Released {
		t.Fatalf("cleanup after serialized stop: %+v %v", res, err)
	}
}
