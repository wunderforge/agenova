// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/toolgateway"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"testing"
)

type incompleteProvider struct{}

func (incompleteProvider) Complete(_ context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	var schema map[string]any
	if json.Unmarshal(r.OutputSchema, &schema) != nil {
		return modelprovider.Result{}, errors.New("missing action schema")
	}
	properties := schema["properties"].(map[string]any)
	if properties["completion"] == nil {
		return modelprovider.Result{}, errors.New("missing completion schema")
	}
	return modelprovider.Result{Text: `{"action":"finish","answer":"No relevant web tool is available.","completion":"incomplete","reason":"missing-capability","target":"web.read"}`, Model: "test-model"}, nil
}

type incompleteExecutor struct{}

func (incompleteExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	reply, err := h(ctx, workerprotocol.Operation{Kind: "model", AgentProtocol: workerprotocol.ResearchProtocol, ClaimID: task.ClaimID, WorkloadToken: task.ModelToken, Profile: task.ModelProfile, Prompt: workerprotocol.PromptPrefix(task) + "Next action:"})
	if err != nil {
		return "", err
	}
	a, err := workerprotocol.ParseAgentAction(reply.Text, task.Tools, workerprotocol.ResearchProtocol)
	return a.Answer, err
}
func TestIncompleteOutcomePreservesAnswerAndMetadata(t *testing.T) {
	service, err := NewService(&verticalBackend{}, incompleteExecutor{}, incompleteProvider{}, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err = service.Submit(verticalRequest(t, "incomplete-result")); err != nil {
		t.Fatal(err)
	}
	view := awaitVertical(t, service, "incomplete-result")
	if view.Outcome.Status != "Succeeded" || view.Outcome.Text != "No relevant web tool is available." || view.Outcome.Completion == nil || view.Outcome.Completion.Status != "incomplete" {
		t.Fatalf("outcome %+v", view.Outcome)
	}
	for _, f := range view.Facts {
		if f.Kind == "ToolDecision" || f.Operation == "NativeHTTPSObservation" {
			t.Fatal("fabricated execution evidence")
		}
	}
}

func TestOptInToolFailureBecomesBoundedObservation(t *testing.T) {
	for _, loseEvidence := range []bool{false, true} {
		provider := &toolDouble{err: toolbackend.ErrTimeout}
		recorded := []facts.Fact{}
		adapter := &providerToolAdapter{recoverable: true, ctx: context.Background(), claims: runningToolClaims(), appendFact: func(f facts.Fact) (facts.Fact, error) {
			if loseEvidence && f.Kind == "ProviderOutcome" {
				return facts.Fact{}, errors.New("write failed")
			}
			recorded = append(recorded, f)
			return f, nil
		}, ref: "work", claimID: "claim", tools: boundTools(t, provider, "repo.read", "repo:example/a"), results: map[string]workerprotocol.Reply{}}
		err := adapter.Invoke("inv", toolgateway.Request{ClaimID: "claim", Tool: "repo", Action: "read", ResourceScope: "repo:example/a", Parameters: map[string]string{"file": "logs/timeout.log"}})
		if loseEvidence {
			if !errors.Is(err, errToolEvidence) {
				t.Fatal("lost evidence recovered")
			}
			continue
		}
		if err != nil || len(recorded) != 2 || adapter.results["inv"].Error != "tool-timeout" {
			t.Fatal(err, recorded, adapter.results)
		}
	}
}

type finiteResearchExecutor struct{}

func (finiteResearchExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	op := workerprotocol.Operation{Kind: "model", AgentProtocol: workerprotocol.ResearchProtocol, ClaimID: task.ClaimID, WorkloadToken: task.ModelToken, Profile: task.ModelProfile, Prompt: workerprotocol.PromptPrefix(task) + "Next action:"}
	if _, err := h(ctx, op); err != nil {
		return "", err
	}
	for _, tool := range task.Tools {
		for _, input := range tool.AllowedValues {
			if _, err := h(ctx, workerprotocol.Operation{Kind: "tool", ClaimID: task.ClaimID, WorkloadToken: task.ToolToken, Tool: tool.Operation, ResourceScope: tool.ResourceScope, Input: input}); err != nil {
				return "", err
			}
		}
	}
	r, err := h(ctx, op)
	if err != nil {
		return "", err
	}
	a, err := workerprotocol.ParseAgentAction(r.Text, task.Tools, workerprotocol.ResearchProtocol)
	return a.Answer, err
}

type finiteResearchProvider struct{ calls int }

func (p *finiteResearchProvider) Complete(_ context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	p.calls++
	if p.calls > 1 {
		var schema struct {
			Properties struct{ Action struct{ Enum []string } }
		}
		if err := json.Unmarshal(r.OutputSchema, &schema); err != nil {
			return modelprovider.Result{}, err
		}
		if len(schema.Properties.Action.Enum) != 1 || schema.Properties.Action.Enum[0] != "finish" {
			return modelprovider.Result{}, errors.New("consumed finite tools still advertised")
		}
	}
	return modelprovider.Result{Text: `{"action":"finish","answer":"Observed the finite inputs.","completion":"completed"}`, Model: "fixture"}, nil
}
func TestResearchFiniteInputsSwitchToFinishOnlySchema(t *testing.T) {
	p := &finiteResearchProvider{}
	s, err := NewService(&verticalBackend{}, finiteResearchExecutor{}, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "research-finite")); err != nil {
		t.Fatal(err)
	}
	v := awaitVertical(t, s, "research-finite")
	if v.Outcome.Status != "Succeeded" || p.calls != 2 {
		t.Fatalf("finite completion failed: %+v calls=%d", v.Outcome, p.calls)
	}
}
