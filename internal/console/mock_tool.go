// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"errors"
	"sort"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/toolgateway"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

var mockArtifacts = map[string]string{
	"README.md":        "SYNTHETIC MOCK: payment client retries transient errors. Total request deadline: 5 seconds. Retry budget must fit inside that deadline. Inspect logs/timeout.log or src/retry.txt for details.",
	"logs/timeout.log": "SYNTHETIC MOCK: attempt 1 upstream latency=4s; retry backoff=2s; attempt 2 started after total deadline=5s; final timeout at 10s. No idempotency key was reused.",
	"src/retry.txt":    "SYNTHETIC MOCK: retry loop allows 3 attempts with fixed 2s backoff. Each attempt starts a fresh 5s deadline instead of using remaining time. A retry has no stable idempotency key.",
}

// mockCatalog advertises the legacy synthetic git.read fixture for each
// granted resource scope, labelled so no prompt or evidence mistakes it for
// a live repository.
func mockCatalog(authority *v0.EffectiveAuthority) []workerprotocol.Tool {
	granted := false
	for _, tool := range authority.Tools {
		granted = granted || tool == "git.read"
	}
	if !granted {
		return nil
	}
	values := make([]string, 0, len(mockArtifacts))
	for name := range mockArtifacts {
		values = append(values, name)
	}
	sort.Strings(values)
	tools := []workerprotocol.Tool{}
	for _, scope := range authority.ResourceScopes {
		tools = append(tools, workerprotocol.Tool{Operation: "git.read", Description: "Read one synthetic mock artifact.", ResourceScope: scope, Parameter: "file", AllowedValues: values, Synthetic: true})
	}
	return tools
}

// Synthetic read adapter: never opens a file, repository, credential or network.
type mockReadAdapter struct {
	ctx          context.Context
	service      *Service
	ref, claimID string
	policy       v0.PolicyReference
	results      map[string]workerprotocol.Reply
}

func (a *mockReadAdapter) Invoke(id string, req toolgateway.Request) error {
	if req.ClaimID != a.claimID || a.ctx.Err() != nil {
		return errors.New("tool session inactive")
	}
	if err := app.RequireRunningClaim(a.service.runner, a.claimID); err != nil {
		return err
	}
	target := "Mock git.read · " + req.Parameters["file"]
	if _, err := a.service.appendTool(facts.Fact{Kind: "ProviderAttempt", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, Operation: "tool.invoke", Target: target, ReasonCode: "mock-tool", ProviderStatus: "Attempted"}); err != nil {
		return errToolEvidence
	}
	reply := workerprotocol.Reply{Allowed: true, Text: mockArtifacts[req.Parameters["file"]]}
	status := "Succeeded"
	code, reason := "mock-tool", ""
	if reply.Text == "" {
		status = "Failed"
		reply.Error = "mock artifact not found; choose README.md, logs/timeout.log or src/retry.txt"
		code, reason = "mock-artifact-not-found", reply.Error
	}
	if a.ctx.Err() != nil {
		return a.ctx.Err()
	}
	if err := app.RequireRunningClaim(a.service.runner, a.claimID); err != nil {
		return err
	}
	if _, err := a.service.appendTool(facts.Fact{Kind: "ProviderOutcome", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, Operation: "tool.invoke", Target: target, ReasonCode: code, Reason: reason, ProviderStatus: status}); err != nil {
		return errToolEvidence
	}
	a.results[id] = reply
	return nil
}
