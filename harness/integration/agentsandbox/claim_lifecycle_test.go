// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Package agentsandbox verifies the supported reduced RuntimeBackend path.
// Use an explicitly confirmed test context with Agent Sandbox installed:
//
// go test -v -tags integration -timeout 5m ./harness/integration/agentsandbox/ -args -kube-context <test-context>
package agentsandbox

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
)

var (
	kubeContext = flag.String("kube-context", "", "explicit kubectl context for the test cluster (required)")
	namespace   = flag.String("namespace", "default", "namespace for unique test resources")
)

func TestRuntimeBackend_AllocateObserveCleanup(t *testing.T) {
	f := newIntegrationFixture(t)
	adapter, claimID := f.adapter, f.claimID
	alloc := f.allocate(t)
	if alloc.ClaimID != claimID || alloc.Identity.Backend != agentsandbox.BackendName || alloc.Identity.WorkerID == "" {
		t.Fatalf("incomplete allocation identity: %+v", alloc)
	}
	t.Logf("allocated claim=%s backend=%s worker=%s", claimID, alloc.Identity.Backend, alloc.Identity.WorkerID)

	deadline := time.Now().Add(90 * time.Second)
	for {
		obs, err := adapter.Observe(alloc.Identity)
		if err != nil {
			t.Fatalf("Observe: %v", err)
		}
		if obs.Identity != alloc.Identity || obs.ClaimID != claimID || obs.Released || obs.Replaced {
			t.Fatalf("invalid allocation observation: %+v", obs)
		}
		if obs.Ready {
			t.Logf("readiness observed for the allocated worker: %+v", obs)
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("readiness not observed: %+v", obs)
		}
		time.Sleep(time.Second)
	}
	if err := adapter.Start(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("Start must expose the missing work-start channel: %v", err)
	}
	if err := adapter.Terminate(alloc.Identity); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("Terminate must expose the missing worker-stop channel: %v", err)
	}
	assertReleased(t, adapter, alloc)
}

func TestRuntimeBackend_CleanupWithoutStart(t *testing.T) {
	f := newIntegrationFixture(t)
	alloc := f.allocate(t)
	// Cancellation does not require claiming application Running or success.
	assertReleased(t, f.adapter, alloc)
}

func assertReleased(t *testing.T, adapter *agentsandbox.SpikeAdapter, alloc runtime.Allocation) {
	t.Helper()
	id := alloc.Identity
	res, err := adapter.Cleanup(id)
	if err != nil || !res.Released || res.Replaced || res.Identity != id {
		t.Fatalf("Cleanup needs confirmed release, not inferred replacement: %+v %v", res, err)
	}
	// Query independently so this gate does not merely trust local bookkeeping.
	for _, resource := range []string{"sandboxes/" + id.WorkerID, "sandboxclaims/agenova-claim-" + alloc.ClaimID} {
		if out, err := kubectl("get", resource, "--ignore-not-found=true", "-o", "name"); err != nil || len(out) != 0 {
			t.Fatalf("resource absence not independently confirmed for %s: %s %v", resource, out, err)
		}
	}
	obs, err := adapter.Observe(id)
	if err != nil || !obs.Released || obs.Ready || obs.Identity != id {
		t.Fatalf("released identity must remain resolvable: %+v %v", obs, err)
	}
	if again, err := adapter.Cleanup(id); err != nil || again != res {
		t.Fatalf("repeated cleanup: %+v %v", again, err)
	}
	if err := adapter.Start(id); !errors.Is(err, runtime.ErrReleased) {
		t.Fatalf("released identity must fail before unsupported start: %v", err)
	}
	t.Logf("release confirmed; replacement remains unobserved: %+v", res)
}

type integrationFixture struct {
	adapter     *agentsandbox.SpikeAdapter
	templateRef string
	claimID     string
	allocation  *runtime.Allocation
}

func (f *integrationFixture) allocate(t *testing.T) runtime.Allocation {
	t.Helper()
	alloc, err := f.adapter.Allocate(runtime.AllocateRequest{ClaimID: f.claimID, TemplateRef: f.templateRef})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	f.allocation = &alloc
	return alloc
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()
	if *kubeContext == "" {
		t.Fatal("an explicitly confirmed test -kube-context is required; no cluster was contacted")
	}
	if out, err := kubectl("cluster-info"); err != nil {
		t.Fatalf("test context %q unavailable; backend evidence blocked: %v %s", *kubeContext, err, out)
	}

	// Unique names isolate this run; never pre-delete another run's resources.
	prefix := fmt.Sprintf("e2e-%x", time.Now().UnixNano())
	templateRef, poolName, claimID := prefix+"-tmpl", prefix+"-pool", prefix+"-claim"
	adapter := agentsandbox.New(*kubeContext, *namespace)
	f := &integrationFixture{adapter: adapter, templateRef: templateRef, claimID: claimID}
	t.Cleanup(func() {
		if f.allocation == nil {
			t.Logf("retain test resources for manual recovery: allocation identity is unconfirmed; prefix=%s", prefix)
			return
		}
		if res, err := adapter.Cleanup(f.allocation.Identity); err != nil || !res.Released {
			t.Errorf("test resource release remains unconfirmed; prefix=%s: %+v %v", prefix, res, err)
			return
		}
		for _, resource := range []string{
			"sandboxwarmpools/agenova-pool-" + poolName,
			"sandboxtemplates/agenova-tmpl-" + templateRef,
		} {
			if out, err := kubectl("delete", resource, "--ignore-not-found=true"); err != nil {
				t.Logf("test resource cleanup (not release evidence): %s: %v %s", resource, err, out)
			}
		}
	})
	if err := adapter.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: templateRef},
		Spec:     v1alpha1.AgentSandboxTemplateSpec{Image: "busybox:stable", Command: []string{"sh", "-c", "sleep 300"}},
	}); err != nil {
		t.Fatalf("AddTemplate: %v", err)
	}
	if err := adapter.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: poolName},
		Spec:     v1alpha1.SandboxWarmPoolSpec{TemplateRef: templateRef, Replicas: 1},
	}); err != nil {
		t.Fatalf("AddWarmPool: %v", err)
	}
	return f
}

func kubectl(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fullArgs := append([]string{"--context", *kubeContext, "--namespace", *namespace}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", fullArgs...)
	cmd.WaitDelay = time.Second
	return cmd.CombinedOutput()
}
