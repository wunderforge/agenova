// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"errors"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/toolgateway"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

// providerToolAdapter is a consumer of the neutral provider contract. Only
// the Tool Gateway calls it, after successful decision recording.
type providerToolAdapter struct {
	ctx          context.Context
	claims       app.ClaimAuthorityReader
	appendFact   func(facts.Fact) (facts.Fact, error)
	ref, claimID string
	tools        *toolbackend.Set
	results      map[string]workerprotocol.Reply
}

func (a *providerToolAdapter) Invoke(id string, req toolgateway.Request) error {
	if req.ClaimID != a.claimID || a.ctx.Err() != nil {
		return errors.New("tool session inactive")
	}
	if err := app.RequireRunningClaim(a.claims, a.claimID); err != nil {
		return err
	}
	operation := req.Tool + "." + req.Action
	if err := a.tools.Catalog().Validate(operation, req.ResourceScope, req.Parameters); err != nil {
		return err
	}
	target := operation // Same logical target for both facts; never a provider URL/reference.
	fact := facts.Fact{Kind: "ProviderAttempt", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, Operation: "tool.invoke", Target: target, ReasonCode: "configured-tool", ProviderStatus: "Attempted"}
	if _, err := a.appendFact(fact); err != nil {
		return err
	}
	result, invokeErr := a.tools.Invoke(a.ctx, toolbackend.Invocation{ID: id, ClaimID: a.claimID, Operation: operation, ResourceScope: req.ResourceScope, Parameters: req.Parameters})
	fact.Kind = "ProviderOutcome"
	fact.ProviderStatus = "Succeeded"
	if invokeErr != nil {
		fact.ProviderStatus = "Failed"
		fact.ReasonCode = "tool-provider-failed"
		fact.Reason = "Configured tool provider failed; no fallback was used."
		switch {
		case errors.Is(invokeErr, toolbackend.ErrUnavailable):
			fact.ReasonCode = "tool-transport-unavailable"
			fact.Reason = "Configured tool transport is unavailable in this build."
		case errors.Is(invokeErr, toolbackend.ErrTimeout), errors.Is(invokeErr, context.DeadlineExceeded):
			fact.ReasonCode = "tool-timeout"
			fact.Reason = "Configured tool provider exceeded its deadline."
		case errors.Is(invokeErr, toolbackend.ErrResponseTooLarge):
			fact.ReasonCode = "tool-response-too-large"
			fact.Reason = "Configured tool response exceeded the byte limit."
		case errors.Is(invokeErr, toolbackend.ErrProtocol):
			fact.ReasonCode = "tool-protocol-error"
			fact.Reason = "Configured tool provider returned an unsupported or malformed response."
		}
	}
	if _, err := a.appendFact(fact); err != nil {
		return errors.New("tool outcome recording failed")
	}
	if invokeErr != nil {
		return invokeErr
	}
	// ResultRef remains separate in the neutral result; evidence projection is
	// Slice 2 work and must update the shared CLI/API/Portal decoders together.
	prefix := "[UNTRUSTED TOOL DATA]\n"
	if result.Truncated {
		prefix += "[TRUNCATED]\n"
	}
	a.results[id] = workerprotocol.Reply{Allowed: true, Text: prefix + result.Text, Untrusted: true, Truncated: result.Truncated}
	return nil
}
