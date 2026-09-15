// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/gateway/gatewaytest"
	"github.com/wunderforge/agenova/internal/modelgateway"
	"github.com/wunderforge/agenova/internal/toolgateway"
)

func TestRunningClaimEnforcesEffectiveAuthorityAcrossGateways(t *testing.T) {
	issued := gatewaytest.LoadTeamAIssuedState(t)
	issued.Claim.Phase = v1alpha1.ClaimPhasePending
	issued.Claim.BackendIdentity = nil
	issued.Evidence.RuntimeEvents = nil

	runtime := newSharedRuntime(t)
	runs, err := app.NewRunService(runtime, app.RunServiceOptions{})
	if err != nil {
		t.Fatalf("NewRunService: %v", err)
	}
	store := facts.NewStore()
	toolAdapter := &referenceToolAdapter{}
	modelAdapter := &referenceModelAdapter{}
	toolGateway := toolgateway.NewGateway(runs, nil, store, toolgateway.WithAdapter(toolAdapter))
	modelGateway := modelgateway.NewGateway(runs, nil, store, modelgateway.WithAdapter(modelAdapter))

	operation := strings.SplitN(issued.EffectiveAuthority.Tools[0], ".", 2)
	if len(operation) != 2 {
		t.Fatalf("canonical tool %q is not tool.action", issued.EffectiveAuthority.Tools[0])
	}
	allowedTool := toolgateway.Request{
		ClaimID:       issued.Claim.ID,
		Tool:          operation[0],
		Action:        operation[1],
		ResourceScope: issued.EffectiveAuthority.ResourceScopes[0],
	}
	allowedModel := modelgateway.Request{
		ClaimID: issued.Claim.ID,
		Profile: issued.EffectiveAuthority.ModelProfile,
	}

	result, runErr := runs.Run(issued, app.ResolvedLaunch{
		ProfileRef:  issued.EffectiveAuthority.Runtime.ProfileRef,
		TemplateRef: "agent-v1",
	}, func(ctx context.Context) error {
		toolDecision, invokeErr := toolGateway.Invoke(allowedTool)
		if invokeErr != nil || toolDecision.Result != gateway.ResultAllow {
			return fmt.Errorf("allowed tool decision=%+v error=%v", toolDecision, invokeErr)
		}
		wrongScope := allowedTool
		wrongScope.ResourceScope = "repo:acme/other"
		toolDecision, invokeErr = toolGateway.Invoke(wrongScope)
		if invokeErr != nil || toolDecision.Result != gateway.ResultDeny || toolDecision.Category != gateway.CategoryResourceNotGranted {
			return fmt.Errorf("wrong-scope decision=%+v error=%v", toolDecision, invokeErr)
		}

		modelDecision, invokeErr := modelGateway.Invoke(allowedModel)
		if invokeErr != nil || modelDecision.Result != gateway.ResultAllow {
			return fmt.Errorf("allowed model decision=%+v error=%v", modelDecision, invokeErr)
		}
		wrongModel := allowedModel
		wrongModel.Profile = "ungranted-premium-model"
		modelDecision, invokeErr = modelGateway.Invoke(wrongModel)
		if invokeErr != nil || modelDecision.Result != gateway.ResultDeny || modelDecision.Category != gateway.CategoryModelProfileNotGranted {
			return fmt.Errorf("wrong-model decision=%+v error=%v", modelDecision, invokeErr)
		}
		return nil
	})
	if runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}
	if result.Claim.Phase != v1alpha1.ClaimPhaseSucceeded {
		t.Fatalf("claim phase = %s, want Succeeded", result.Claim.Phase)
	}
	if toolAdapter.attempts != 1 || modelAdapter.attempts != 1 {
		t.Fatalf("adapter attempts = tool %d model %d, want one allowed call each", toolAdapter.attempts, modelAdapter.attempts)
	}
	toolFacts := store.ToolInvocations(issued.Claim.ID)
	modelFacts := store.ModelInvocations(issued.Claim.ID)
	if len(toolFacts) != 2 || toolFacts[0].Result != gateway.ResultAllow || toolFacts[1].Result != gateway.ResultDeny {
		t.Fatalf("tool facts = %+v, want Allow then Deny", toolFacts)
	}
	if len(modelFacts) != 2 || modelFacts[0].Result != gateway.ResultAllow || modelFacts[1].Result != gateway.ResultDeny {
		t.Fatalf("model facts = %+v, want Allow then Deny", modelFacts)
	}
}
