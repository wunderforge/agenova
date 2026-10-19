// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration && controlled

package agentsandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
)

var controlledWorkerImage = flag.String("controlled-worker-image", "agenova-testworker:kind", "compatible worker image for the selected cluster")

// TestControlledRuntimeBackend_Kind proves one actual task process on the
// selected upstream v0.4.6 kind substrate. It never creates/deletes a cluster
// and uses unique resource names in the explicitly selected namespace.
// Before running, build/load agenova-testworker:kind into that kind cluster.
func TestControlledRuntimeBackend_Kind(t *testing.T) {
	testControlledLifecycle(t, false)
}

// This drops all adapter memory while the real worker remains running. It tests
// cleanup-only reattachment, not a full service/database crash campaign.
func TestControlledRuntimeBackend_Recovery(t *testing.T) {
	testControlledLifecycle(t, true)
}

func testControlledLifecycle(t *testing.T, recoverWorker bool) {
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
			Image: *controlledWorkerImage, Command: []string{"/agenova-workerctl", "serve"},
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
	claimToken := workerControlClaimToken(claimID)
	status, err := kubectl("exec", "pod/"+alloc.Identity.WorkerID, "-c", "agent", "--", "/agenova-workerctl", "status", claimToken)
	if err != nil {
		t.Fatalf("read task result: %v %s", err, status)
	}
	sum := sha256.Sum256([]byte("agenova-test-worker:" + claimToken))
	result := "probe-" + hex.EncodeToString(sum[:8])
	wantRunning := "state=running claim=" + claimToken + " result=" + result
	if strings.TrimSpace(string(status)) != wantRunning {
		t.Fatalf("wrong worker result: got %q, want %q", status, wantRunning)
	}
	t.Logf("actual child-task result: %s", strings.TrimSpace(string(status)))
	if recoverWorker {
		adapter = agentsandbox.NewControlled(*kubeContext, *namespace)
		if err := adapter.RestoreForCleanup(claimID, alloc.Identity); err != nil {
			t.Fatal(err)
		}
		if err := adapter.Start(alloc.Identity); !errors.Is(err, runtime.ErrAlreadyStarted) {
			t.Fatalf("recovery allowed a second start: %v", err)
		}
		if _, err := adapter.Cleanup(alloc.Identity); err == nil {
			t.Fatal("recovery deleted a worker before confirmed stop")
		}
		after, err := kubectl("exec", "pod/"+alloc.Identity.WorkerID, "-c", "agent", "--", "/agenova-workerctl", "status", claimToken)
		if err != nil || string(after) != string(status) {
			t.Fatalf("recovery changed running worker: %q %v", after, err)
		}
		t.Log("fresh adapter rejected Start and premature cleanup; worker status unchanged")
	}

	if err := adapter.Terminate(alloc.Identity); err != nil {
		t.Fatalf("Terminate has no independent stop acknowledgement: %v", err)
	}
	started = false
	status, err = kubectl("exec", "pod/"+alloc.Identity.WorkerID, "-c", "agent", "--", "/agenova-workerctl", "status", claimToken)
	wantStopped := "state=stopped claim=" + claimToken + " result=" + result
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

func workerControlClaimToken(claimID string) string {
	sum := sha256.Sum256([]byte("agenova-worker-control:" + claimID))
	return "sha256:" + hex.EncodeToString(sum[:])
}
