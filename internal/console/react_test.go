// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"testing"
	"time"
)

type reactExecutor struct{ foreignScope bool }

func (e reactExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	model := workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: workerprotocol.LoopPrompt(task, "")}
	if _, err := h(ctx, model); err != nil {
		return "", err
	}
	scope := task.ResourceScope
	if e.foreignScope {
		scope = "repo:outside/private"
	}
	reply, err := h(ctx, workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "tool", Tool: "git.read", ResourceScope: scope, Input: "logs/timeout.log"})
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
