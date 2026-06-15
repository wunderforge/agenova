//go:build integration

// Package e2e contains integration tests for the Phase 2 Agent Sandbox adapter.
// Run against kind-agenova-k8s-lab (requires cluster + upstream controller):
//
//	go test -v -tags integration -timeout 5m ./harness/phase-2-agent-sandbox-runtime/e2e/
package e2e

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/donozhang1992/agenova/api/v1alpha1"
	"github.com/donozhang1992/agenova/internal/runtime/agentsandbox"
)

var (
	kubeContext = flag.String("kube-context", "kind-agenova-k8s-lab", "kubectl context for the test cluster")
	namespace   = flag.String("namespace", "default", "kubernetes namespace for test resources")
)

const (
	// testImage is a real, publicly pullable image for spike e2e tests.
	// It exits cleanly after a short sleep, letting the controller observe completion.
	testImage   = "busybox:stable"
	testCommand = `["sh", "-c", "echo agenova-e2e-start && sleep 10 && echo agenova-e2e-done"]`
)

func TestClaimLifecycle_CreateBindStartSucceed(t *testing.T) {
	requireKindCluster(t)
	cleanupResources(t)
	t.Cleanup(func() { cleanupResources(t) })

	adapter := agentsandbox.New(*kubeContext, *namespace)

	// 1. AddTemplate
	template := v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "busybox-e2e-v1"},
		Spec: v1alpha1.AgentSandboxTemplateSpec{
			Image:   testImage,
			Command: []string{"sh", "-c", "echo agenova-e2e-start && sleep 10 && echo agenova-e2e-done"},
		},
	}
	if err := adapter.AddTemplate(template); err != nil {
		t.Fatalf("AddTemplate: %v", err)
	}
	t.Log("AddTemplate: ok")

	// 2. AddWarmPool
	pool := v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "busybox-e2e-pool"},
		Spec: v1alpha1.SandboxWarmPoolSpec{
			TemplateRef: "busybox-e2e-v1",
			Replicas:    1,
		},
	}
	if err := adapter.AddWarmPool(pool); err != nil {
		t.Fatalf("AddWarmPool: %v", err)
	}
	t.Log("AddWarmPool: ok")

	// Wait briefly for the controller to pre-create idle sandboxes.
	time.Sleep(3 * time.Second)

	// 3. AddClaim
	claim := v1alpha1.SandboxClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "e2e-smoke-run"},
		Spec:     v1alpha1.SandboxClaimSpec{PoolRef: "busybox-e2e-pool"},
	}
	if err := adapter.AddClaim(claim); err != nil {
		t.Fatalf("AddClaim: %v", err)
	}
	assertClaimPhase(t, adapter, "e2e-smoke-run", v1alpha1.ClaimPhasePending)
	t.Log("AddClaim: Pending ok")

	// 4. BindClaim - blocks until the upstream controller assigns a sandbox.
	t.Log("BindClaim: waiting for controller to assign sandbox...")
	if err := adapter.BindClaim("e2e-smoke-run"); err != nil {
		// Record blocker if binding times out.
		t.Logf("BLOCKER: BindClaim failed: %v", err)
		logKubectlState(t)
		t.Fatalf("BindClaim: %v", err)
	}
	bound := assertClaimPhase(t, adapter, "e2e-smoke-run", v1alpha1.ClaimPhaseBound)
	if bound.Status.SandboxID == "" {
		t.Fatal("Bound claim should record sandboxID from upstream")
	}
	t.Logf("BindClaim: Bound ok, sandboxID=%s", bound.Status.SandboxID)

	// 5. StartClaim - blocks until sandbox pod is ready.
	t.Log("StartClaim: waiting for sandbox pod ready...")
	if err := adapter.StartClaim("e2e-smoke-run"); err != nil {
		t.Logf("BLOCKER: StartClaim failed: %v", err)
		logKubectlState(t)
		t.Fatalf("StartClaim: %v", err)
	}
	assertClaimPhase(t, adapter, "e2e-smoke-run", v1alpha1.ClaimPhaseRunning)
	t.Log("StartClaim: Running ok")

	// 6. SucceedClaim - deletes upstream claim; controller cleans up sandbox.
	if err := adapter.SucceedClaim("e2e-smoke-run"); err != nil {
		t.Fatalf("SucceedClaim: %v", err)
	}
	succeeded := assertClaimPhase(t, adapter, "e2e-smoke-run", v1alpha1.ClaimPhaseSucceeded)
	if !succeeded.Status.SandboxReplaced {
		t.Fatal("Succeeded claim should record SandboxReplaced=true")
	}
	t.Logf("SucceedClaim: Succeeded ok, SandboxReplaced=true")

	logKubectlState(t)
}

func TestClaimLifecycle_ExpirePendingClaim(t *testing.T) {
	requireKindCluster(t)
	cleanupResources(t)
	t.Cleanup(func() { cleanupResources(t) })

	adapter := agentsandbox.New(*kubeContext, *namespace)

	if err := adapter.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "busybox-e2e-v1"},
		Spec:     v1alpha1.AgentSandboxTemplateSpec{Image: testImage, Command: []string{"sh", "-c", "sleep 1"}},
	}); err != nil {
		t.Fatalf("AddTemplate: %v", err)
	}
	if err := adapter.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "busybox-e2e-pool"},
		Spec:     v1alpha1.SandboxWarmPoolSpec{TemplateRef: "busybox-e2e-v1", Replicas: 1},
	}); err != nil {
		t.Fatalf("AddWarmPool: %v", err)
	}
	if err := adapter.AddClaim(v1alpha1.SandboxClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "e2e-expire-run"},
		Spec:     v1alpha1.SandboxClaimSpec{PoolRef: "busybox-e2e-pool"},
	}); err != nil {
		t.Fatalf("AddClaim: %v", err)
	}
	assertClaimPhase(t, adapter, "e2e-expire-run", v1alpha1.ClaimPhasePending)

	if err := adapter.ExpireClaim("e2e-expire-run", "ttl elapsed"); err != nil {
		t.Fatalf("ExpireClaim: %v", err)
	}
	expired := assertClaimPhase(t, adapter, "e2e-expire-run", v1alpha1.ClaimPhaseExpired)
	if expired.Status.SandboxID != "" {
		t.Fatal("Expired pending claim should not have a sandboxID")
	}
	if expired.Status.SandboxReplaced {
		t.Fatal("Expired pending claim should not record SandboxReplaced")
	}
	t.Log("ExpireClaim: Expired ok")
}

// --- helpers ---

func requireKindCluster(t *testing.T) {
	t.Helper()
	out, err := exec.Command("kubectl", "--context", *kubeContext, "cluster-info").CombinedOutput()
	if err != nil {
		t.Fatalf("kind cluster %q not reachable; e2e evidence cannot pass: %v\n%s", *kubeContext, err, out)
	}
}

func cleanupResources(t *testing.T) {
	t.Helper()
	resources := []string{
		fmt.Sprintf("sandboxclaims/agenova-claim-e2e-smoke-run --namespace=%s", *namespace),
		fmt.Sprintf("sandboxclaims/agenova-claim-e2e-expire-run --namespace=%s", *namespace),
		fmt.Sprintf("sandboxwarmpools/agenova-pool-busybox-e2e-pool --namespace=%s", *namespace),
		fmt.Sprintf("sandboxtemplates/agenova-tmpl-busybox-e2e-v1 --namespace=%s", *namespace),
	}
	for _, r := range resources {
		parts := strings.Fields(r)
		args := append([]string{"--context", *kubeContext, "delete", "--ignore-not-found=true"}, parts...)
		if out, err := exec.Command("kubectl", args...).CombinedOutput(); err != nil {
			t.Logf("cleanup %s: %v\n%s", r, err, out)
		}
	}
}

func assertClaimPhase(t *testing.T, a *agentsandbox.SpikeAdapter, name string, want v1alpha1.ClaimPhase) v1alpha1.SandboxClaim {
	t.Helper()
	claim, ok := a.Claim(name)
	if !ok {
		t.Fatalf("claim not found: %s", name)
	}
	if claim.Status.Phase != want {
		t.Fatalf("claim %s phase = %s, want %s", name, claim.Status.Phase, want)
	}
	return claim
}

func logKubectlState(t *testing.T) {
	t.Helper()
	cmds := [][]string{
		{"--context", *kubeContext, "--namespace", *namespace, "get", "sandboxtemplates,sandboxwarmpools,sandboxclaims", "-o", "wide"},
		{"--context", *kubeContext, "--namespace", *namespace, "get", "pods", "-o", "wide"},
		{"--context", *kubeContext, "--namespace", *namespace, "get", "events", "--sort-by=.lastTimestamp"},
	}
	for _, args := range cmds {
		out, err := exec.Command("kubectl", args...).CombinedOutput()
		if err != nil {
			t.Logf("kubectl %v: %v", args, err)
		} else {
			t.Logf("kubectl %v:\n%s", args[4:], out)
		}
	}
}
