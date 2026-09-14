// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package agentsandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
)

// TestControlledRuntimeBackend_Kind proves one actual task process on the
// selected upstream v0.4.6 kind substrate. It never creates/deletes a cluster
// and uses unique resource names in the explicitly selected namespace.
// Before running, build/load agenova-testworker:kind into that kind cluster.
func TestControlledRuntimeBackend_Kind(t *testing.T) {
	if *kubeContext == "" {
		t.Fatal("explicit -kube-context required; no cluster was contacted")
	}
	if out, err := kubectl("cluster-info"); err != nil {
		t.Fatalf("context %q unavailable: %v %s", *kubeContext, err, out)
	}
	prefix := fmt.Sprintf("run51-%x", time.Now().UnixNano())
	templateRef, poolName, claimID := prefix+"-tmpl", prefix+"-pool", prefix+"-claim"
	adapter := agentsandbox.NewControlled(*kubeContext, *namespace)
	var alloc *runtime.Allocation
	var started, released bool
	t.Cleanup(func() {
		if alloc != nil && !released {
			if started {
				if err := adapter.Terminate(alloc.Identity); err != nil {
					t.Errorf("worker stop unconfirmed; retaining resources for inspection: claim=%s worker=%s err=%v", claimID, alloc.Identity.WorkerID, err)
					return
				}
			}
			res, err := adapter.Cleanup(alloc.Identity)
			if err != nil || !res.Released {
				t.Errorf("release unconfirmed; retaining template/pool for inspection: claim=%s worker=%s result=%+v err=%v", claimID, alloc.Identity.WorkerID, res, err)
				return
			}
		} else if alloc == nil {
			t.Logf("allocation identity unconfirmed; retaining resources for inspection: prefix=%s", prefix)
			return
		}
		for _, resource := range []string{
			"sandboxwarmpools/agenova-pool-" + poolName,
			"sandboxtemplates/agenova-tmpl-" + templateRef,
		} {
			if out, err := kubectl("delete", resource, "--ignore-not-found=true"); err != nil {
				t.Errorf("cleanup own %s: %v %s", resource, err, out)
			}
		}
	})
	if err := adapter.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: templateRef},
		Spec: v1alpha1.AgentSandboxTemplateSpec{
			Image: "agenova-testworker:kind", Command: []string{"/agenova-workerctl", "serve"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: poolName},
		Spec:     v1alpha1.SandboxWarmPoolSpec{TemplateRef: templateRef, Replicas: 1},
	}); err != nil {
		t.Fatal(err)
	}
	value, err := adapter.Allocate(runtime.AllocateRequest{ClaimID: claimID, TemplateRef: templateRef})
	if err != nil {
		t.Fatalf("Allocate: %v; resource prefix retained for inspection: %s", err, prefix)
	}
	alloc = &value
	if alloc.Identity.Backend != agentsandbox.BackendName || alloc.Identity.WorkerID == "" || alloc.ClaimID != claimID {
		t.Fatalf("incomplete neutral allocation: %+v", alloc)
	}
	t.Logf("allocated claim=%s worker=%s backend=%s", claimID, alloc.Identity.WorkerID, alloc.Identity.Backend)

	deadline := time.Now().Add(120 * time.Second)
	for {
		obs, err := adapter.Observe(alloc.Identity)
		if err != nil || obs.Identity != alloc.Identity || obs.ClaimID != claimID {
			t.Fatalf("Observe binding: %+v %v", obs, err)
		}
		if obs.Ready {
			t.Logf("Ready is infrastructure-only: claim=%s worker=%s", claimID, alloc.Identity.WorkerID)
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("worker not Ready: %+v", obs)
		}
		time.Sleep(time.Second)
	}
	if err := adapter.Start(alloc.Identity); err != nil {
		t.Fatalf("Start has no real task acknowledgement: %v", err)
	}
	started = true
	status, err := kubectl("exec", "pod/"+alloc.Identity.WorkerID, "-c", "agent", "--", "/agenova-workerctl", "status", claimID)
	if err != nil {
		t.Fatalf("read task result: %v %s", err, status)
	}
	sum := sha256.Sum256([]byte("agenova-test-worker:" + claimID))
	result := "probe-" + hex.EncodeToString(sum[:8])
	wantRunning := "state=running claim=" + claimID + " result=" + result
	if strings.TrimSpace(string(status)) != wantRunning {
		t.Fatalf("wrong worker result: got %q, want %q", status, wantRunning)
	}
	t.Logf("actual child-task result: %s", strings.TrimSpace(string(status)))
	if err := adapter.Terminate(alloc.Identity); err != nil {
		t.Fatalf("Terminate has no independent stop acknowledgement: %v", err)
	}
	started = false
	status, err = kubectl("exec", "pod/"+alloc.Identity.WorkerID, "-c", "agent", "--", "/agenova-workerctl", "status", claimID)
	wantStopped := "state=stopped claim=" + claimID + " result=" + result
	if err != nil || strings.TrimSpace(string(status)) != wantStopped {
		t.Fatalf("worker stop unconfirmed: status=%q err=%v", status, err)
	}
	t.Logf("task process stopped before resource deletion: %s", strings.TrimSpace(string(status)))
	cleanup, err := adapter.Cleanup(alloc.Identity)
	if err != nil || !cleanup.Released || cleanup.Identity != alloc.Identity {
		t.Fatalf("Cleanup: %+v %v", cleanup, err)
	}
	released = true
	for _, resource := range []string{
		"sandboxclaims/agenova-claim-" + claimID,
		"sandboxes/" + alloc.Identity.WorkerID,
		"pods/" + alloc.Identity.WorkerID,
	} {
		deadline := time.Now().Add(30 * time.Second)
		for {
			out, err := kubectl("get", resource, "--ignore-not-found=true", "-o", "name")
			if err != nil {
				t.Fatalf("independent release query %s: %v %s", resource, err, out)
			}
			if len(strings.TrimSpace(string(out))) == 0 {
				break
			}
			if !time.Now().Before(deadline) {
				t.Fatalf("resource still present after Cleanup: %s %s", resource, out)
			}
			time.Sleep(time.Second)
		}
	}
	obs, err := adapter.Observe(alloc.Identity)
	if err != nil || !obs.Released || obs.Ready {
		t.Fatalf("released observation: %+v %v", obs, err)
	}
	t.Logf("confirmed cleanup: claim=%s worker=%s released=%t", claimID, alloc.Identity.WorkerID, cleanup.Released)
}
