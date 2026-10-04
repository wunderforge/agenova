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

// errToolEvidence reports that a tool decision, attempt or outcome could not
// be recorded. After an attempt it means the external effect is unknown, so
// it must never read as an ordinary tool failure or as zero activity.
var errToolEvidence = errors.New("tool evidence recording failed")

func (a *providerToolAdapter) Invoke(id string, req toolgateway.Request) error {
	// A mismatched claim cannot be attributed safely, so nothing is recorded.
	// The service binds both sides, so this is a correlation guard only.
	if req.ClaimID != a.claimID {
		return errors.New("tool session inactive")
	}
	operation := req.Tool + "." + req.Action
	target := operation // Same logical target for both facts; never a provider URL/reference.
	fact := facts.Fact{Kind: "ProviderAttempt", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, Operation: "tool.invoke", Target: target, ReasonCode: "configured-tool", ProviderStatus: "Attempted"}
	// Checks that can still fail after the Gateway allowed the call (a
	// cancellation or termination race, or an argument the service did not
	// pre-validate) complete the invocation with an attempt/outcome pair, so it
	// never stays half-recorded. The provider is not called.
	reject := func(status, code, reason string, cause error) error {
		if _, err := a.appendFact(fact); err != nil {
			return errToolEvidence
		}
		outcome := fact
		outcome.Kind, outcome.ProviderStatus, outcome.ReasonCode, outcome.Reason = "ProviderOutcome", status, code, reason
		if _, err := a.appendFact(outcome); err != nil {
			return errToolEvidence
		}
		return cause
	}
	if err := a.ctx.Err(); err != nil {
		return reject("Cancelled", "tool-session-inactive", "The Work stopped before the configured tool was called.", err)
	}
	if err := app.RequireRunningClaim(a.claims, a.claimID); err != nil {
		return reject("Cancelled", "tool-claim-not-running", "The claim was no longer running when the configured tool would have been called.", err)
	}
	if err := a.tools.Catalog().Validate(operation, req.ResourceScope, req.Parameters); err != nil {
		return reject("Failed", "tool-arguments-rejected", "The tool arguments are not configured for this operation and resource; no call was made.", err)
	}
	if _, err := a.appendFact(fact); err != nil {
		return errToolEvidence
	}
	result, invokeErr := a.tools.Invoke(a.ctx, toolbackend.Invocation{ID: id, ClaimID: a.claimID, Operation: operation, ResourceScope: req.ResourceScope, Parameters: req.Parameters})
	fact.Kind = "ProviderOutcome"
	fact.ProviderStatus = "Succeeded"
	if invokeErr == nil && facts.ValidResultRef(result.ResultRef) {
		fact.ResultRef = result.ResultRef
	}
	if invokeErr == nil {
		fact.Truncated = result.Truncated
	}
	if invokeErr != nil {
		fact.ProviderStatus = "Failed"
		fact.ReasonCode = "tool-provider-failed"
		fact.Reason = "Configured tool provider failed; no fallback was used."
		switch {
		case errors.Is(invokeErr, toolbackend.ErrUnavailable):
			fact.ReasonCode = "tool-transport-unavailable"
			fact.Reason = "Configured tool server is unavailable; no fallback was used."
		case errors.Is(invokeErr, toolbackend.ErrTimeout), errors.Is(invokeErr, context.DeadlineExceeded):
			fact.ReasonCode = "tool-timeout"
			fact.Reason = "Configured tool provider exceeded its deadline."
		case errors.Is(invokeErr, toolbackend.ErrResponseTooLarge):
			fact.ReasonCode = "tool-response-too-large"
			fact.Reason = "Configured tool response exceeded the byte limit."
		case errors.Is(invokeErr, toolbackend.ErrProtocol):
			fact.ReasonCode = "tool-protocol-error"
			fact.Reason = "Configured tool provider returned an unsupported or malformed response."
		case errors.Is(invokeErr, toolbackend.ErrCredentialUnavailable):
			fact.ReasonCode = "tool-credential-unavailable"
			fact.Reason = "Configured tool credential could not be resolved; no request was sent and no fallback was used."
		case errors.Is(invokeErr, toolbackend.ErrCredentialRejected):
			fact.ReasonCode = "tool-credential-rejected"
			fact.Reason = "Configured tool server rejected the credential; no retry or fallback was used."
		}
	}
	if _, err := a.appendFact(fact); err != nil {
		return errToolEvidence
	}
	if invokeErr != nil {
		return invokeErr
	}
	prefix := "[UNTRUSTED TOOL DATA]\n"
	if result.Truncated {
		prefix += "[TRUNCATED]\n"
	}
	a.results[id] = workerprotocol.Reply{Allowed: true, Text: prefix + result.Text, Untrusted: true, Truncated: result.Truncated}
	return nil
}
