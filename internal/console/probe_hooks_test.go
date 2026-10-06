// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build agenovaprobe

package console

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

// The probe bridge fails one tool-path append and must stop the call before
// the provider, while every other fact still reaches the real journal.
func TestProbeAppendWrapperFailsBeforeProviderCall(t *testing.T) {
	for _, kind := range []string{"ToolDecision", "ProviderAttempt"} {
		t.Run(kind, func(t *testing.T) {
			provider := &toolDouble{}
			service, err := NewServiceWithOptions(&verticalBackend{}, reactExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{ToolBackend: boundTools(t, provider, "git.read", "repo:acme/payments")})
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			var failed atomic.Int32
			if err := WrapToolAppendForProbe(service, func(next func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error) {
				return func(f facts.Fact) (facts.Fact, error) {
					if f.Kind == kind {
						failed.Add(1)
						return facts.Fact{}, errors.New("probe journal failure")
					}
					return next(f)
				}
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Submit(verticalRequest(t, "probe")); err != nil {
				t.Fatal(err)
			}
			view := awaitVertical(t, service, "probe")
			if provider.calls.Load() != 0 || failed.Load() != 1 || view.Outcome.Status != "Failed" {
				t.Fatalf("calls=%d failed=%d status=%q", provider.calls.Load(), failed.Load(), view.Outcome.Status)
			}
			outcome := ""
			for _, f := range view.Facts {
				if f.Operation == "tool.invoke" && (f.Kind == kind || f.Kind == "ProviderOutcome") {
					t.Fatalf("a failed append was recorded or the call completed: %+v", f)
				}
				if f.Kind == "RunOutcome" {
					outcome = f.ReasonCode
				}
			}
			if outcome != "tool-evidence-failed" {
				t.Fatalf("Work outcome reason %q, want tool-evidence-failed", outcome)
			}
		})
	}
}

func TestProbeAppendWrapperRefusedOnceWorkExists(t *testing.T) {
	service, err := NewServiceWithOptions(&verticalBackend{}, reactExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{ToolBackend: boundTools(t, &toolDouble{}, "git.read", "repo:acme/payments")})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Submit(verticalRequest(t, "probe")); err != nil {
		t.Fatal(err)
	}
	awaitVertical(t, service, "probe")
	identity := func(next func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error) { return next }
	if err := WrapToolAppendForProbe(service, identity); err == nil {
		t.Fatal("wrapper installed after Work existed")
	}
}

// ignoringExecutor swallows a tool error and finishes normally, as a worker
// that does not abort on handler errors might.
type ignoringExecutor struct{}

func (ignoringExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	model := workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: workerprotocol.LoopPrompt(task, "")}
	if _, err := h(ctx, model); err != nil {
		return "", err
	}
	_, _ = h(ctx, workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "tool", Tool: task.Tools[0].Operation, ResourceScope: task.Tools[0].ResourceScope, Input: "logs/timeout.log"})
	reply, err := h(ctx, model)
	return reply.Text, err
}

func TestLostToolOutcomeFailsWorkEvenIfWorkerContinues(t *testing.T) {
	provider := &toolDouble{}
	service, err := NewServiceWithOptions(&verticalBackend{}, ignoringExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{ToolBackend: boundTools(t, provider, "git.read", "repo:acme/payments")})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if err := WrapToolAppendForProbe(service, func(next func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error) {
		return func(f facts.Fact) (facts.Fact, error) {
			if f.Kind == "ProviderOutcome" && f.Operation == "tool.invoke" {
				return facts.Fact{}, errors.New("probe journal failure")
			}
			return next(f)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Submit(verticalRequest(t, "ignored")); err != nil {
		t.Fatal(err)
	}
	view := awaitVertical(t, service, "ignored")
	reason := ""
	for _, f := range view.Facts {
		if f.Kind == "RunOutcome" {
			reason = f.ReasonCode
		}
	}
	if provider.calls.Load() != 1 || view.Outcome.Status == "Succeeded" || reason != "tool-evidence-failed" {
		t.Fatalf("calls=%d status=%q reason=%q", provider.calls.Load(), view.Outcome.Status, reason)
	}
}
