// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"errors"
	"strings"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/toolgateway"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

// ToolProvider is a trusted, host-side provider. The Tool Gateway calls it
// only after the running claim and exact operation/resource grant are checked.
// It receives no external credential from the agent or ClaimRequest.
type ToolProvider interface {
	Invoke(context.Context, string, toolgateway.Request) (ToolResult, error)
}

type ToolResult struct {
	Reply       workerprotocol.Reply
	AuditTarget string // Bounded public reference, such as a PR URL or Deployment.
	FailureCode string // Bounded provider-owned reason; never raw command output.
	FailureNote string // Safe public summary, not model-authored or secret text.
}

type realToolAdapter struct {
	ctx          context.Context
	service      *Service
	ref, claimID string
	policy       v0.PolicyReference
	provider     ToolProvider
	results      map[string]workerprotocol.Reply
}

func (a *realToolAdapter) Invoke(id string, req toolgateway.Request) error {
	if req.ClaimID != a.claimID || a.ctx.Err() != nil || a.provider == nil {
		return errors.New("tool session inactive")
	}
	if err := app.RequireRunningClaim(a.service.runner, a.claimID); err != nil {
		return err
	}
	operation := req.Tool + "." + req.Action
	if _, err := a.service.journal.Append(facts.Fact{Kind: "ProviderAttempt", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, PolicyRef: &a.policy, Operation: "tool.invoke", Target: operation, ReasonCode: "real-provider-attempt", ProviderStatus: "Attempted"}); err != nil {
		return err // No external call after a failed pre-attempt record.
	}
	result, providerErr := a.provider.Invoke(a.ctx, id, req)
	status, code, target := "Succeeded", "real-provider-succeeded", operation
	reason := ""
	if providerErr != nil {
		status, code = "Failed", "real-provider-failed"
		result.Reply = workerprotocol.Reply{Allowed: false, Error: "tool provider failed; inspect the provider outcome"}
		var safe interface{ SafeCode() string }
		if errors.As(providerErr, &safe) {
			if label := safe.SafeCode(); label != "" && len(label) <= 64 {
				code = "real-provider-" + label
				reason = "The bounded demo provider reported " + label + "."
				result.Reply.Error = "tool provider failed: " + label
			}
		}
	} else if !result.Reply.Allowed {
		status, code = "Failed", "real-provider-rejected"
		if result.FailureCode != "" && len(result.FailureCode) <= 64 {
			code = "real-provider-" + result.FailureCode
		}
		if result.FailureNote != "" && len(result.FailureNote) <= 256 {
			reason = result.FailureNote
		}
	}
	if result.AuditTarget != "" && len(result.AuditTarget) <= 256 && !strings.ContainsAny(result.AuditTarget, "\r\n") {
		target = result.AuditTarget
	}
	if _, err := a.service.journal.Append(facts.Fact{Kind: "ProviderOutcome", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, PolicyRef: &a.policy, Operation: "tool.invoke", Target: target, ReasonCode: code, Reason: reason, ProviderStatus: status}); err != nil {
		return err
	}
	a.results[id] = result.Reply
	return nil
}
