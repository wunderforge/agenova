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
