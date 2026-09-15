// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration && controlled

package agentsandbox

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/runtime"
	runtimeagentsandbox "github.com/wunderforge/agenova/internal/runtime/agentsandbox"
)

func TestGovernedApplicationRun_Kind(t *testing.T) {
	if *kubeContext == "" {
		t.Fatal("explicit -kube-context required; no cluster was contacted")
	}
	runNamespace := strings.TrimSpace(*namespace)
	if runNamespace == "" || runNamespace == "default" {
		t.Fatal("explicit disposable non-default -namespace required; no cluster was contacted")
	}
	if out, err := kubectlForContext("cluster-info"); err != nil {
		t.Fatalf("context %q unavailable: %v %s", *kubeContext, err, out)
	}
	if out, err := kubectlForContext("get", "namespace", runNamespace, "--ignore-not-found=true", "-o", "name"); err != nil {
		t.Fatalf("check namespace ownership for %q: %v %s", runNamespace, err, out)
	} else if len(strings.TrimSpace(string(out))) != 0 {
		t.Fatalf("refuse to reuse namespace %q; provide a new disposable name: %s", runNamespace, out)
	}
	if out, err := kubectlForContext("create", "namespace", runNamespace); err != nil {
		t.Fatalf("create owned namespace %q: %v %s", runNamespace, err, out)
	}
	cleanupConfirmed := false
	t.Cleanup(func() {
		if !cleanupConfirmed {
			t.Logf("retain namespace %q for diagnosis because application cleanup was not confirmed", runNamespace)
			return
		}
		if out, err := kubectlForContext("delete", "namespace", runNamespace, "--wait=true"); err != nil {
			t.Errorf("delete integration-owned namespace %q: %v %s", runNamespace, err, out)
		}
	})

	adapter := runtimeagentsandbox.NewControlled(*kubeContext, runNamespace)
	const poolName = "reference-engineer-pool"
	if err := adapter.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: app.ReferenceRuntimeTemplateRef},
		Spec: v1alpha1.AgentSandboxTemplateSpec{
			Image: "agenova-testworker:kind", Command: []string{"/agenova-workerctl", "serve"},
		},
	}); err != nil {
		t.Fatalf("AddTemplate: %v", err)
	}
	if err := adapter.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: poolName},
		Spec: v1alpha1.SandboxWarmPoolSpec{
			TemplateRef: app.ReferenceRuntimeTemplateRef,
			Replicas:    1,
		},
	}); err != nil {
		t.Fatalf("AddWarmPool: %v", err)
	}

	recorder := &recordingRuntimeBackend{RuntimeBackend: adapter}
	requestPath := filepath.Join("..", "..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", "valid-team-a-engineer.yaml")

	denied, err := app.SubmitClaimRequestFile(requestPath, recorder, app.ReferencePrincipalTeamB)
	if err != nil {
		t.Fatalf("Team B submission: %v", err)
	}
	if denied.Decision != v1alpha1.DecisionResultDeny || denied.Allocated || recorder.callCount() != 0 {
		t.Fatalf("Team B reached backend: result=%+v calls=%s", denied, recorder.calls)
	}
	if claims := namesFor(t, "sandboxclaims"); len(claims) != 0 {
		t.Fatalf("Team B denial left Kubernetes claims: %v", claims)
	}
	t.Logf("denied before allocation: request=%s principal=%s decision=%s backendCalls=0", denied.RequestRef, denied.Principal, denied.Decision)

	recorder.afterStart = func(claimID string, identity v1alpha1.SandboxClaimBackendIdentity) error {
		claimToken := workerControlClaimToken(claimID)
		out, err := kubectl("exec", "pod/"+identity.WorkerID, "-c", "agent", "--", "/agenova-workerctl", "status", claimToken)
		if err != nil {
			return fmt.Errorf("read controlled result: %w: %s", err, out)
		}
		status := strings.TrimSpace(string(out))
		if !strings.HasPrefix(status, "state=running claim="+claimToken+" result=probe-") {
			return fmt.Errorf("unexpected controlled result %q", status)
		}
		recorder.workerResult = status
		return nil
	}
	allowed, err := app.SubmitClaimRequestFile(requestPath, recorder, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatalf("Team A submission: %v", err)
	}
	if allowed.Decision != v1alpha1.DecisionResultAllow || !allowed.Allocated || allowed.ClaimID == "" || allowed.Phase != v1alpha1.ClaimPhaseSucceeded {
		t.Fatalf("Team A result: %+v", allowed)
	}
	if err := recorder.assertCompleted(allowed.ClaimID); err != nil {
		t.Fatal(err)
	}
	if claims := namesFor(t, "sandboxclaims"); len(claims) != 0 {
		t.Fatalf("claim resources remain after application cleanup: %v", claims)
	}
	for _, resource := range []string{
		"sandbox/" + recorder.allocation.Identity.WorkerID,
		"pod/" + recorder.allocation.Identity.WorkerID,
	} {
		assertAbsentEventuallyInNamespace(t, runNamespace, resource)
	}
	cleanupConfirmed = true
	t.Logf("governed kind run complete: request=%s decision=%s claim=%s worker=%s phase=%s", allowed.RequestRef, allowed.Decision, allowed.ClaimID, recorder.allocation.Identity.WorkerID, allowed.Phase)
	t.Logf("controlled worker result: %s", recorder.workerResult)
	t.Logf("backend calls: %s", strings.Join(recorder.calls, " -> "))
}

type recordingRuntimeBackend struct {
	runtime.RuntimeBackend
	calls        []string
	allocation   runtime.Allocation
	afterStart   func(string, v1alpha1.SandboxClaimBackendIdentity) error
	workerResult string
}

func (r *recordingRuntimeBackend) Allocate(request runtime.AllocateRequest) (runtime.Allocation, error) {
	r.calls = append(r.calls, "Allocate")
	allocation, err := r.RuntimeBackend.Allocate(request)
	if err == nil {
		r.allocation = allocation
	}
	return allocation, err
}

func (r *recordingRuntimeBackend) Observe(identity v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	r.calls = append(r.calls, "Observe")
	return r.RuntimeBackend.Observe(identity)
}

func (r *recordingRuntimeBackend) Start(identity v1alpha1.SandboxClaimBackendIdentity) error {
	r.calls = append(r.calls, "Start")
	if err := r.RuntimeBackend.Start(identity); err != nil {
		return err
	}
	if r.afterStart != nil {
		return r.afterStart(r.allocation.ClaimID, identity)
	}
	return nil
}

func (r *recordingRuntimeBackend) Terminate(identity v1alpha1.SandboxClaimBackendIdentity) error {
	r.calls = append(r.calls, "Terminate")
	return r.RuntimeBackend.Terminate(identity)
}

func (r *recordingRuntimeBackend) Cleanup(identity v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	r.calls = append(r.calls, "Cleanup")
	return r.RuntimeBackend.Cleanup(identity)
}

func (r *recordingRuntimeBackend) callCount() int {
	return len(r.calls)
}

func (r *recordingRuntimeBackend) assertCompleted(claimID string) error {
	if r.allocation.ClaimID != claimID || r.allocation.Identity.Backend != runtimeagentsandbox.BackendName || r.allocation.Identity.WorkerID == "" {
		return fmt.Errorf("uncorrelated allocation: claim=%s allocation=%+v", claimID, r.allocation)
	}
	if r.workerResult == "" {
		return fmt.Errorf("controlled worker result was not observed")
	}
	wantSuffix := []string{"Start", "Terminate", "Cleanup"}
	if len(r.calls) < len(wantSuffix) {
		return fmt.Errorf("incomplete backend calls: %v", r.calls)
	}
	for index, want := range wantSuffix {
		got := r.calls[len(r.calls)-len(wantSuffix)+index]
		if got != want {
			return fmt.Errorf("backend call suffix=%v, want %v", r.calls, wantSuffix)
		}
	}
	return nil
}

func namesFor(t *testing.T, resource string) []string {
	t.Helper()
	out, err := kubectl("get", resource, "-o", "name")
	if err != nil {
		t.Fatalf("list %s: %v %s", resource, err, out)
	}
	fields := strings.Fields(string(out))
	return fields
}

func assertAbsentEventually(t *testing.T, resource string) {
	assertAbsentEventuallyInNamespace(t, *namespace, resource)
}

func assertAbsentEventuallyInNamespace(t *testing.T, targetNamespace, resource string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		out, err := kubectlInNamespace(targetNamespace, "get", resource, "--ignore-not-found=true", "-o", "name")
		if err != nil {
			t.Fatalf("query released %s: %v %s", resource, err, out)
		}
		if strings.TrimSpace(string(out)) == "" {
			return
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("released resource still present: %s %s", resource, out)
		}
		time.Sleep(time.Second)
	}
}
