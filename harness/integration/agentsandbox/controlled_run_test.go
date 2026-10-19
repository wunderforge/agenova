// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration && controlled

package agentsandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/gateway/gatewaytest"
	"github.com/wunderforge/agenova/internal/modelgateway"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/runtime/agentsandbox"
	"github.com/wunderforge/agenova/internal/toolgateway"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"github.com/wunderforge/agenova/internal/workloadidentity"
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
	workload, err := workloadidentity.New([]byte("0123456789abcdef0123456789abcdef"), nil)
	if err != nil {
		t.Fatal(err)
	}
	modelToken, err := workload.Issue(claimID, alloc.Identity.WorkerID, "model-gateway", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	toolToken, err := workload.Issue(claimID, alloc.Identity.WorkerID, "tool-gateway", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	claims := gatewaytest.NewClaims()
	claims.Put(claimID, v1alpha1.ClaimPhaseRunning)
	bound := claims.Items[claimID]
	bound.BackendIdentity = &alloc.Identity
	claims.Items[claimID] = bound
	claims.Put("foreign-claim", v1alpha1.ClaimPhaseRunning)
	modelAdapter, toolAdapter := &kindModelSpy{}, &kindToolSpy{}
	store := facts.NewStore()
	modelGW := modelgateway.NewGateway(claims, nil, store, modelgateway.WithWorkloadIdentity(workload), modelgateway.WithAdapter(modelAdapter))
	toolGW := toolgateway.NewGateway(claims, nil, store, toolgateway.WithWorkloadIdentity(workload), toolgateway.WithAdapter(toolAdapter))
	callbackCalls := 0
	answer, err := adapter.Execute(context.Background(), alloc.Identity, workerprotocol.Task{ClaimID: claimID, ModelToken: modelToken, ToolToken: toolToken, Objective: "Say hello", ModelProfile: "approved-coding-model", Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "git.read", Description: "Read a repository file", ResourceScope: "repo:acme/payments", Parameter: "file", AllowedValues: []string{"README.md"}}}}, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
		callbackCalls++
		audience := "model-gateway"
		if op.Kind == "tool" {
			audience = "tool-gateway"
		}
		verified, verifyErr := workload.Verify(op.WorkloadToken, audience)
		if verifyErr != nil || verified.ClaimID != claimID || verified.WorkerID != alloc.Identity.WorkerID {
			return workerprotocol.Reply{}, fmt.Errorf("worker identity rejected")
		}
		if op.Kind == "model" {
			decision, invokeErr := modelGW.InvokeAuthenticated(modelgateway.Request{ClaimID: op.ClaimID, Profile: op.Profile}, op.WorkloadToken)
			if invokeErr != nil || decision.Result != gateway.ResultAllow {
				return workerprotocol.Reply{}, fmt.Errorf("model Gateway rejected live worker")
			}
		} else {
			decision, invokeErr := toolGW.InvokeAuthenticated(toolgateway.Request{ClaimID: op.ClaimID, Tool: "git", Action: "read", ResourceScope: op.ResourceScope}, op.WorkloadToken)
			if invokeErr != nil || decision.Result != gateway.ResultAllow {
				return workerprotocol.Reply{}, fmt.Errorf("tool Gateway rejected live worker")
			}
		}
		switch callbackCalls {
		case 1:
			return workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"git.read","input":"README.md","answer":""}`}, nil
		case 2:
			if op.Kind != "tool" {
				return workerprotocol.Reply{}, fmt.Errorf("expected governed tool call")
			}
			return workerprotocol.Reply{Allowed: true, Text: "synthetic observation"}, nil
		default:
			return workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","tool":"","input":"","answer":"hello"}`}, nil
		}
	})
	if err != nil || answer != "hello" || callbackCalls != 3 {
		t.Fatalf("claim-bound worker exchange: answer=%q calls=%d err=%v", answer, callbackCalls, err)
	}
	if modelAdapter.calls != 2 || toolAdapter.calls != 1 {
		t.Fatalf("unexpected external call counts: model=%d tool=%d", modelAdapter.calls, toolAdapter.calls)
	}
	if d, _ := modelGW.InvokeAuthenticated(modelgateway.Request{ClaimID: "foreign-claim", Profile: "approved-coding-model"}, modelToken); d.Result != gateway.ResultDeny || modelAdapter.calls != 2 || len(store.ModelInvocations("foreign-claim")) != 0 {
		t.Fatal("worker borrowed foreign model authority")
	}
	if d, _ := toolGW.InvokeAuthenticated(toolgateway.Request{ClaimID: "foreign-claim", Tool: "git", Action: "read", ResourceScope: "repo:acme/payments"}, toolToken); d.Result != gateway.ResultDeny || toolAdapter.calls != 1 || len(store.ToolInvocations("foreign-claim")) != 0 {
		t.Fatal("worker borrowed foreign tool authority")
	}
	bound.Phase = v1alpha1.ClaimPhaseSucceeded
	claims.Items[claimID] = bound
	if d, _ := modelGW.InvokeAuthenticated(modelgateway.Request{ClaimID: claimID, Profile: "approved-coding-model"}, modelToken); d.Result != gateway.ResultDeny || modelAdapter.calls != 2 {
		t.Fatal("terminal claim invoked model adapter")
	}
	if d, _ := toolGW.InvokeAuthenticated(toolgateway.Request{ClaimID: claimID, Tool: "git", Action: "read", ResourceScope: "repo:acme/payments"}, toolToken); d.Result != gateway.ResultDeny || toolAdapter.calls != 1 {
		t.Fatal("terminal claim invoked tool adapter")
	}
	t.Logf("real worker: model=2 tool=1; cross-claim and terminal adapter calls=0; tokens omitted")
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
	t.Log("task process stopped before resource deletion; control token omitted")
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

type kindModelSpy struct{ calls int }

func (s *kindModelSpy) Invoke(string, modelgateway.Request) error { s.calls++; return nil }

type kindToolSpy struct{ calls int }

func (s *kindToolSpy) Invoke(string, toolgateway.Request) error { s.calls++; return nil }

func workerControlClaimToken(claimID string) string {
	sum := sha256.Sum256([]byte("agenova-worker-control:" + claimID))
	return "sha256:" + hex.EncodeToString(sum[:])
}
