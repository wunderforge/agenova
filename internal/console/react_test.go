// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"testing"
	"time"
)

type reactExecutor struct {
	foreignScope bool
	artifact     string
}

func (e reactExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	model := workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: workerprotocol.LoopPrompt(task, "")}
	if _, err := h(ctx, model); err != nil {
		return "", err
	}
	scope := task.ResourceScope
	if e.foreignScope {
		scope = "repo:outside/private"
	}
	artifact := e.artifact
	if artifact == "" {
		artifact = "logs/timeout.log"
	}
	reply, err := h(ctx, workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "tool", Tool: "git.read", ResourceScope: scope, Input: artifact})
	if err != nil {
		return "", err
	}
	if !reply.Allowed {
		return "", errors.New("tool denied")
	}
	model.Prompt = workerprotocol.LoopPrompt(task, reply.Text)
	reply, err = h(ctx, model)
	return reply.Text, err
}

type finishOnlyProvider struct{}

func (finishOnlyProvider) Complete(_ context.Context, req modelprovider.Request) (modelprovider.Result, error) {
	if string(req.OutputSchema) != workerprotocol.FinishSchema {
		return modelprovider.Result{}, errors.New("unavailable tools remain schema-valid")
	}
	return modelprovider.Result{Text: `{"action":"finish","tool":"","input":"","answer":"No tool access; answer using task input."}`}, nil
}

func TestNoToolAssignmentsUseFinishOnlySchema(t *testing.T) {
	for _, withoutTools := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-resource-scope", true: "no-read-tool"}[withoutTools], func(t *testing.T) {
			var req v0.ClaimRequest
			if err := json.Unmarshal(verticalRequest(t, "finish-only"), &req); err != nil {
				t.Fatal(err)
			}
			if withoutTools {
				req.Spec.RequestedAccess.Tools = []string{}
			} else {
				req.Spec.RequestedAccess.ResourceScopes = []string{}
			}
			body, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			s, err := NewService(&verticalBackend{}, &verticalExecutor{}, finishOnlyProvider{}, app.ReferencePrincipalTeamA)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err := s.Submit(body); err != nil {
				t.Fatal(err)
			}
			view := awaitVertical(t, s, "finish-only")
			if view.Outcome.Status != "Succeeded" {
				t.Fatalf("no-tool assignment failed: %+v", view.Outcome)
			}
			for _, f := range view.Facts {
				if f.Operation == "tool.invoke" {
					t.Fatal("no-tool assignment invoked a tool")
				}
			}
		})
	}
}

func TestMissingMockArtifactRetainsSpecificCorrelatedReason(t *testing.T) {
	s, err := NewService(&verticalBackend{}, reactExecutor{artifact: "unavailable.txt"}, &verticalProvider{}, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Submit(verticalRequest(t, "missing-artifact")); err != nil {
		t.Fatal(err)
	}
	view := awaitVertical(t, s, "missing-artifact")
	for _, f := range view.Facts {
		if f.Kind == "ProviderOutcome" && f.Operation == "tool.invoke" {
			if f.ProviderStatus != "Failed" || f.ReasonCode != "mock-artifact-not-found" || f.Reason != "mock artifact not found; choose README.md, logs/timeout.log or src/retry.txt" || f.InvocationID == "" || f.ClaimID != view.State.Claim.ID || f.RequestRef != view.RequestRef {
				t.Fatalf("missing actionable failure attribution: %+v", f)
			}
			return
		}
	}
	t.Fatal("missing tool outcome")
}
func TestReActConsoleRecordsGovernedMockAndTurns(t *testing.T) {
	for _, denied := range []bool{false, true} {
		s, err := NewService(&verticalBackend{}, reactExecutor{foreignScope: denied}, &verticalProvider{}, app.ReferencePrincipalTeamA)
		if err != nil {
			t.Fatal(err)
		}
		initial, err := s.Submit(verticalRequest(t, "react-run"))
		if err != nil {
			t.Fatal(err)
		}
		var viewFacts int
		var decision, attempt, observation, turns int
		deadline := time.Now().Add(5 * time.Second)
		for {
			v, err := s.QueryRequest(initial.RequestRef)
			if err != nil {
				t.Fatal(err)
			}
			if v.Outcome != nil {
				for _, f := range v.Facts {
					if f.ClaimID != "" && f.ClaimID != v.State.Claim.ID {
						t.Fatal("cross-claim attribution")
					}
					if f.Kind == "ToolDecision" {
						decision++
					}
					if f.Kind == "ProviderAttempt" && f.Operation == "tool.invoke" {
						attempt++
					}
					if f.Kind == "WorkerActivity" && f.Operation == "ObservationReceived" {
						observation++
					}
					if f.Kind == "WorkerActivity" && f.Operation == "TurnStarted" {
						turns++
					}
				}
				viewFacts = len(v.Facts)
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("no outcome")
			}
			time.Sleep(time.Millisecond)
		}
		s.Close()
		if decision != 1 || viewFacts == 0 {
			t.Fatal("missing tool policy evidence")
		}
		if denied {
			if attempt != 0 || observation != 0 {
				t.Fatal("denial reached mock adapter")
			}
		} else if attempt != 1 || observation != 1 || turns != 2 {
			t.Fatalf("attempt=%d observations=%d turns=%d", attempt, observation, turns)
		}
	}
}
