package operator

import (
	"testing"

	"github.com/donozhang1992/agenova/api/v1alpha1"
	"github.com/donozhang1992/agenova/internal/sandbox"
)

func TestSucceededClaimAutoReplacesSandboxWithoutGrowingActivePool(t *testing.T) {
	runtime := newRuntime(t)

	claim := v1alpha1.SandboxClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "research-run"},
		Spec: v1alpha1.SandboxClaimSpec{
			PoolRef: "python-agent-pool",
			Input:   map[string]string{"task": "research-run", "topic": "agent lifecycle"},
		},
	}
	if err := runtime.AddClaim(claim); err != nil {
		t.Fatalf("add claim: %v", err)
	}

	assertPoolStatus(t, runtime, "python-agent-pool", v1alpha1.SandboxWarmPoolStatus{IdleSandboxes: 1})

	if err := runtime.BindClaim("research-run"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	bound := assertClaimPhase(t, runtime, "research-run", v1alpha1.ClaimPhaseBound)
	if bound.Status.SandboxID == "" {
		t.Fatal("bound claim should record sandbox id")
	}
	assertPoolStatus(t, runtime, "python-agent-pool", v1alpha1.SandboxWarmPoolStatus{BoundClaims: 1})

	if err := runtime.StartClaim("research-run"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	assertClaimPhase(t, runtime, "research-run", v1alpha1.ClaimPhaseRunning)
	assertPoolStatus(t, runtime, "python-agent-pool", v1alpha1.SandboxWarmPoolStatus{RunningClaims: 1})

	if err := runtime.SucceedClaim("research-run"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}

	succeeded := assertClaimPhase(t, runtime, "research-run", v1alpha1.ClaimPhaseSucceeded)
	if succeeded.Status.SandboxID != bound.Status.SandboxID {
		t.Fatalf("succeeded claim should keep original sandbox id, got %q want %q", succeeded.Status.SandboxID, bound.Status.SandboxID)
	}
	if !succeeded.Status.SandboxReplaced {
		t.Fatal("succeeded claim should record the sandbox-replaced resource fact")
	}
	assertPoolStatus(t, runtime, "python-agent-pool", v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})

	sandboxes, ok := runtime.PoolSandboxes("python-agent-pool")
	if !ok {
		t.Fatal("expected pool sandboxes")
	}
	if len(sandboxes) != 1 {
		t.Fatalf("expected active pool to stay at replicas=1, got %d", len(sandboxes))
	}
	assertSandboxAbsent(t, sandboxes, bound.Status.SandboxID)
	replacementID := ""
	for _, candidate := range sandboxes {
		if candidate.Phase == sandbox.PhaseIdle {
			replacementID = candidate.ID
		}
	}
	if replacementID == "" {
		t.Fatal("expected an idle replacement sandbox")
	}
	if replacementID == bound.Status.SandboxID {
		t.Fatal("replacement sandbox should have a new id")
	}
}

func TestFailedClaimAutoReplacesSandboxAndLeavesPoolClaimable(t *testing.T) {
	runtime := newRuntime(t)
	addPendingClaim(t, runtime, "fail-once")

	if err := runtime.BindClaim("fail-once"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	bound := assertClaimPhase(t, runtime, "fail-once", v1alpha1.ClaimPhaseBound)
	if err := runtime.StartClaim("fail-once"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	if err := runtime.FailClaim("fail-once", "agent exited 1"); err != nil {
		t.Fatalf("fail claim: %v", err)
	}

	failed := assertClaimPhase(t, runtime, "fail-once", v1alpha1.ClaimPhaseFailed)
	if failed.Status.Error != "agent exited 1" {
		t.Fatalf("expected failure summary, got %q", failed.Status.Error)
	}
	if !failed.Status.SandboxReplaced {
		t.Fatal("failed claim should record the sandbox-replaced resource fact")
	}
	assertPoolStatus(t, runtime, "python-agent-pool", v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})
	sandboxes, ok := runtime.PoolSandboxes("python-agent-pool")
	if !ok {
		t.Fatal("expected pool sandboxes")
	}
	if len(sandboxes) != 1 {
		t.Fatalf("expected active pool to stay at replicas=1, got %d", len(sandboxes))
	}
	assertSandboxAbsent(t, sandboxes, bound.Status.SandboxID)

	addPendingClaim(t, runtime, "after-failure")
	if err := runtime.BindClaim("after-failure"); err != nil {
		t.Fatalf("pool should remain claimable after failed claim replacement: %v", err)
	}
	afterFailure := assertClaimPhase(t, runtime, "after-failure", v1alpha1.ClaimPhaseBound)
	if afterFailure.Status.SandboxID == "" {
		t.Fatal("replacement sandbox should bind to the next claim")
	}
	if afterFailure.Status.SandboxID == bound.Status.SandboxID {
		t.Fatal("failed sandbox should not be reused for the next claim")
	}
}

func TestBoundClaimCanFailBeforeStartAndReplacesSandbox(t *testing.T) {
	runtime := newRuntime(t)
	addPendingClaim(t, runtime, "lost-before-start")

	if err := runtime.BindClaim("lost-before-start"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	bound := assertClaimPhase(t, runtime, "lost-before-start", v1alpha1.ClaimPhaseBound)

	if err := runtime.FailClaim("lost-before-start", "sandbox lost before start"); err != nil {
		t.Fatalf("fail bound claim: %v", err)
	}

	failed := assertClaimPhase(t, runtime, "lost-before-start", v1alpha1.ClaimPhaseFailed)
	if failed.Status.Error != "sandbox lost before start" {
		t.Fatalf("expected failure summary, got %q", failed.Status.Error)
	}
	if !failed.Status.SandboxReplaced {
		t.Fatal("bound-failed claim should record the sandbox-replaced resource fact")
	}
	assertPoolStatus(t, runtime, "python-agent-pool", v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})
	sandboxes, ok := runtime.PoolSandboxes("python-agent-pool")
	if !ok {
		t.Fatal("expected pool sandboxes")
	}
	assertSandboxAbsent(t, sandboxes, bound.Status.SandboxID)
}

func TestPendingClaimCanExpireWithoutBindingSandbox(t *testing.T) {
	runtime := newRuntime(t)
	addPendingClaim(t, runtime, "expire-once")

	if err := runtime.ExpireClaim("expire-once", "ttl elapsed"); err != nil {
		t.Fatalf("expire claim: %v", err)
	}

	expired := assertClaimPhase(t, runtime, "expire-once", v1alpha1.ClaimPhaseExpired)
	if expired.Status.SandboxID != "" {
		t.Fatalf("expired pending claim should not bind a sandbox, got %q", expired.Status.SandboxID)
	}
	if expired.Status.SandboxReplaced {
		t.Fatal("expired pending claim never bound a sandbox, so none should be replaced")
	}
	assertPoolStatus(t, runtime, "python-agent-pool", v1alpha1.SandboxWarmPoolStatus{IdleSandboxes: 1})
}

func TestDuplicateClaimIsRejected(t *testing.T) {
	runtime := newRuntime(t)
	addPendingClaim(t, runtime, "duplicate")

	err := runtime.AddClaim(v1alpha1.SandboxClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "duplicate"},
		Spec: v1alpha1.SandboxClaimSpec{
			PoolRef: "python-agent-pool",
		},
	})
	if err == nil {
		t.Fatal("expected duplicate claim to be rejected")
	}
}

func TestInvalidTransitionRemainsRejected(t *testing.T) {
	runtime := newRuntime(t)
	addPendingClaim(t, runtime, "invalid")

	if err := runtime.StartClaim("invalid"); err == nil {
		t.Fatal("pending claim should not start before bind")
	}
	if err := runtime.SucceedClaim("invalid"); err == nil {
		t.Fatal("pending claim should not succeed")
	}
	if err := runtime.FailClaim("invalid", "no sandbox yet"); err == nil {
		t.Fatal("pending claim should not fail; it expires instead")
	}
	if err := runtime.BindClaim("invalid"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	if err := runtime.SucceedClaim("invalid"); err == nil {
		t.Fatal("bound claim should not succeed before running")
	}
	if err := runtime.StartClaim("invalid"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	if err := runtime.ExpireClaim("invalid", "too late"); err == nil {
		t.Fatal("running claim should not expire through pending transition")
	}
	if err := runtime.SucceedClaim("invalid"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}
	if err := runtime.SucceedClaim("invalid"); err == nil {
		t.Fatal("terminal claim should not transition again")
	}
}

func newRuntime(t *testing.T) *Runtime {
	t.Helper()

	runtime := NewRuntime()
	template := v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "python-agent-v1"},
		Spec: v1alpha1.AgentSandboxTemplateSpec{
			Image:   "example.local/agenova/python-agent:dev",
			Command: []string{"python", "/app/agent.py"},
		},
	}
	if err := runtime.AddTemplate(template); err != nil {
		t.Fatalf("add template: %v", err)
	}

	pool := v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "python-agent-pool"},
		Spec: v1alpha1.SandboxWarmPoolSpec{
			TemplateRef: "python-agent-v1",
			Replicas:    1,
		},
	}
	if err := runtime.AddWarmPool(pool); err != nil {
		t.Fatalf("add warm pool: %v", err)
	}
	return runtime
}

func addPendingClaim(t *testing.T, runtime *Runtime, name string) {
	t.Helper()

	claim := v1alpha1.SandboxClaim{
		Metadata: v1alpha1.ObjectMeta{Name: name},
		Spec: v1alpha1.SandboxClaimSpec{
			PoolRef: "python-agent-pool",
		},
	}
	if err := runtime.AddClaim(claim); err != nil {
		t.Fatalf("add claim: %v", err)
	}
}

func assertClaimPhase(t *testing.T, runtime *Runtime, name string, phase v1alpha1.ClaimPhase) v1alpha1.SandboxClaim {
	t.Helper()

	claim, ok := runtime.Claim(name)
	if !ok {
		t.Fatalf("claim not found: %s", name)
	}
	if claim.Status.Phase != phase {
		t.Fatalf("claim phase = %s, want %s", claim.Status.Phase, phase)
	}
	return claim
}

func assertPoolStatus(t *testing.T, runtime *Runtime, name string, want v1alpha1.SandboxWarmPoolStatus) {
	t.Helper()

	got, ok := runtime.PoolStatus(name)
	if !ok {
		t.Fatalf("pool not found: %s", name)
	}
	if got != want {
		t.Fatalf("pool status = %+v, want %+v", got, want)
	}
}

func assertSandboxAbsent(t *testing.T, sandboxes []sandbox.Sandbox, id string) {
	t.Helper()

	for _, candidate := range sandboxes {
		if candidate.ID == id {
			t.Fatalf("sandbox %s should not be present in active pool", id)
		}
	}
}
