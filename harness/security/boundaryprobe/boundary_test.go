// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// These are supplementary in-memory composition tests for #198. They use the
// exported console.Executor seam, so they do NOT exercise exchangeWorker,
// kubectl, the installed control plane, or real model-originated injection.
package boundaryprobe_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/operator"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type provider struct{ calls atomic.Int32 }

func (p *provider) Complete(context.Context, modelprovider.Request) (modelprovider.Result, error) {
	p.calls.Add(1)
	return modelprovider.Result{Text: `{"action":"finish","tool":"","input":"","answer":"synthetic answer"}`, Model: "synthetic"}, nil
}

type executor func(context.Context, v0.SandboxClaimBackendIdentity, workerprotocol.Task, workerprotocol.Handler) (string, error)

func (e executor) Execute(ctx context.Context, id v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	return e(ctx, id, task, h)
}

func service(t *testing.T, e executor) (*console.Service, *provider) {
	t.Helper()
	// Two idle workers remove capacity as an explanation for B queueing.
	backend := operator.NewRuntime()
	if err := backend.AddTemplate(v0.AgentSandboxTemplate{Metadata: v0.ObjectMeta{Name: app.ReferenceRuntimeTemplateRef}, Spec: v0.AgentSandboxTemplateSpec{Image: "example.local/e15/synthetic-worker:dev"}}); err != nil {
		t.Fatal(err)
	}
	if err := backend.AddWarmPool(v0.SandboxWarmPool{Metadata: v0.ObjectMeta{Name: "e15-probe-pool"}, Spec: v0.SandboxWarmPoolSpec{TemplateRef: app.ReferenceRuntimeTemplateRef, Replicas: 2}}); err != nil {
		t.Fatal(err)
	}
	p := new(provider)
	s, err := console.NewService(backend, e, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, p
}
func request(t *testing.T, name string, limit time.Duration) []byte {
	t.Helper()
	timeout := v0.Duration(limit)
	r := v0.ClaimRequest{APIVersion: v0.ClaimRequestAPIVersion, Kind: v0.ClaimRequestKind, Metadata: v0.ObjectMeta{Name: name}, Spec: v0.ClaimRequestSpec{TemplateRef: "engineer", ProjectRef: "payments", Task: &v0.ClaimRequestTask{Type: "investigation", Input: map[string]any{"objective": name}}, RequestedAccess: v0.ClaimRequestedAccess{Tools: []string{"git.read"}, ResourceScopes: []string{"repo:acme/payments"}, ModelProfile: "approved-coding-model"}, Runtime: &v0.ClaimRuntimeRequirements{ProfileRef: "standard-isolated", Timeout: &timeout}}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func view(t *testing.T, s *console.Service, ref string) evidence.View {
	t.Helper()
	v, err := s.QueryRequest(ref)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func terminal(t *testing.T, s *console.Service, ref string) evidence.View {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v := view(t, s, ref)
		if v.Outcome != nil {
			return v
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no terminal outcome")
	return evidence.View{}
}
func model(task workerprotocol.Task) workerprotocol.Operation {
	return workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: workerprotocol.LoopPrompt(task, "")}
}

type retained struct {
	h  workerprotocol.Handler
	op workerprotocol.Operation
}

func TestRetainedHandlerRefusesAllTerminalPhases(t *testing.T) {
	for _, phase := range []v0.ClaimPhase{v0.ClaimPhaseSucceeded, v0.ClaimPhaseFailed, v0.ClaimPhaseExpired} {
		t.Run(string(phase), func(t *testing.T) {
			captured := make(chan retained, 1)
			e := executor(func(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
				op := model(task)
				reply, err := h(ctx, op)
				if err != nil {
					return "", err
				}
				captured <- retained{h, op}
				switch phase {
				case v0.ClaimPhaseFailed:
					return "", errors.New("synthetic worker failure")
				case v0.ClaimPhaseExpired:
					<-ctx.Done()
					return "", ctx.Err()
				}
				return reply.Text, nil
			})
			s, p := service(t, e)
			limit := time.Minute
			if phase == v0.ClaimPhaseExpired {
				limit = time.Second
			}
			if _, err := s.Submit(request(t, string(phase), limit)); err != nil {
				t.Fatal(err)
			}
			before := terminal(t, s, string(phase))
			if before.State.Claim.Phase != phase {
				t.Fatalf("phase=%s want %s", before.State.Claim.Phase, phase)
			}
			var saved retained
			select {
			case saved = <-captured:
			default:
				t.Fatal("positive control did not reach executor")
			}
			if p.calls.Load() != 1 {
				t.Fatal("positive control must make exactly one provider call")
			}
			// A fresh context ensures refusal is not merely cancellation of the old
			// request context. The claim's authoritative terminal phase is checked.
			for _, kind := range []string{"model", "tool"} {
				op := saved.op
				if kind == "tool" {
					op = workerprotocol.Operation{ClaimID: op.ClaimID, Kind: "tool", Tool: "git.read", ResourceScope: "repo:acme/payments", Input: "README.md"}
				}
				reply, err := saved.h(context.Background(), op)
				if err == nil || reply.Allowed {
					t.Fatalf("late %s allowed: %+v / %v", kind, reply, err)
				}
			}
			after := view(t, s, string(phase))
			if p.calls.Load() != 1 || !reflect.DeepEqual(before.Facts, after.Facts) {
				t.Fatal("late request changed provider count or facts")
			}
			t.Logf("phase=%s lateModel=refused lateTool=refused providerDelta=0 decisionFactDelta=0 (missing rejection evidence; Go seam only)", phase)
		})
	}
}
func TestConsoleQueuesSecondClaimUntilFirstCompletes(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	e := executor(func(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
		reply, err := h(ctx, model(task))
		if err != nil {
			return "", err
		}
		started <- struct{}{}
		if task.Objective == "first" {
			select {
			case <-release:
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		return reply.Text, nil
	})
	s, p := service(t, e)
	if _, err := s.Submit(request(t, "first", time.Minute)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first did not start")
	}
	if _, err := s.Submit(request(t, "second", time.Minute)); err != nil {
		t.Fatal(err)
	}
	a, b := view(t, s, "first"), view(t, s, "second")
	if a.State.Claim.Phase != v0.ClaimPhaseRunning || b.State.Claim.Phase != v0.ClaimPhasePending || b.State.Claim.BackendIdentity != nil || p.calls.Load() != 1 {
		t.Fatal("queue observation not canonical")
	}
	select {
	case <-started:
		t.Fatal("second executed while first was held")
	default:
	}
	close(release)
	for _, ref := range []string{"first", "second"} {
		if v := terminal(t, s, ref); v.State.Claim.Phase != v0.ClaimPhaseSucceeded {
			t.Fatal("queued work did not finish")
		}
	}
	if p.calls.Load() != 2 {
		t.Fatal("expected one call per completed claim")
	}
	t.Log("same Service with two idle workers: A=Running B=Pending B.backendIdentity=nil; B ran after A completed (Go composition evidence)")
}
func TestGatewayDenyAvailableThroughExecutorSeamOnly(t *testing.T) {
	for _, kind := range []string{"profile", "scope"} {
		t.Run(kind, func(t *testing.T) {
			e := executor(func(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
				if _, err := h(ctx, model(task)); err != nil {
					return "", err
				}
				op := model(task)
				if kind == "profile" {
					op.Profile = "ungranted-profile"
				} else {
					op = workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "tool", Tool: "git.read", ResourceScope: "repo:other/private", Input: "README.md"}
				}
				reply, err := h(ctx, op)
				if err != nil {
					return "", err
				}
				if reply.Allowed {
					return "", errors.New("unexpected Allow")
				}
				return "", errors.New("worker stopped after Deny")
			})
			s, p := service(t, e)
			if _, err := s.Submit(request(t, kind, time.Minute)); err != nil {
				t.Fatal(err)
			}
			v := terminal(t, s, kind)
			if v.State.Claim.Phase != v0.ClaimPhaseFailed || p.calls.Load() != 1 {
				t.Fatal("negative operation reached provider or did not fail")
			}
			category := "model-profile-not-granted"
			decisionKind := "ModelDecision"
			if kind == "scope" {
				category = "resource-scope-not-granted"
				decisionKind = "ToolDecision"
			}
			denials := 0
			for _, f := range v.Facts {
				if f.Kind == decisionKind && f.Result == v0.DecisionResultDeny {
					denials++
					if f.ReasonCode != category || f.Reason == "" || f.PolicyRef == nil || f.PolicyRef.ID == "" || f.PolicyRef.Version == "" || f.ClaimID != v.State.Claim.ID {
						t.Fatalf("incomplete Deny: %+v", f)
					}
				}
				if f.Kind == "ProviderAttempt" && f.Operation == "tool.invoke" {
					t.Fatal("denied scope reached tool adapter")
				}
			}
			if denials != 1 {
				t.Fatalf("Deny count=%d", denials)
			}
			t.Logf("category=%s Deny has policy+reason providerDelta=0; injected Executor skips installed transport, not kind/A2 proof", category)
		})
	}
}
