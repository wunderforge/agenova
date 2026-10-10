// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package connectedclient

import (
	"slices"
	"strings"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
)

func memoryOperation(operation string) bool {
	return operation == "memory.read" || operation == "memory.write" || operation == "memory.invalid"
}

// Memory facts carry only stable metadata, never free-form diagnostic content.
func validMemoryFact(f facts.Fact) bool {
	if f.Reason != "" || f.Decision != nil || f.Authority != nil || len(f.AuthorityChanges) != 0 || f.BackendIdentity != nil || f.PolicyRef == nil {
		return false
	}
	switch f.Kind {
	case "MemoryDecision":
		if !memoryOperation(f.Operation) || f.Memory != nil || f.ProviderStatus != "" {
			return false
		}
		if f.Result == v0.DecisionResultAllow {
			return f.Operation != "memory.invalid" && strings.TrimSpace(f.Target) != "" && f.ReasonCode == "memory-allowed"
		}
		return f.Result == v0.DecisionResultDeny && slices.Contains([]string{
			"memory-denied", "memory-context-mismatch", "memory-claim-inactive", "memory-context-cancelled", "memory-invalid-input",
			"memory-operation-not-granted", "memory-scope-not-granted", "memory-ownership-denied",
		}, f.ReasonCode)
	case "ProviderAttempt":
		return f.Operation != "memory.invalid" && f.Memory == nil && f.Result == "" && f.ProviderStatus == "Attempted" && f.ReasonCode == "memory-allowed"
	case "ProviderOutcome":
		if f.Result != "" || !facts.ValidMemoryOutcome(f.Memory, f.Operation, f.ProviderStatus) {
			return false
		}
		reason := "memory-" + strings.ToLower(f.Memory.Status)
		switch f.Memory.Status {
		case "Written", "Found", "Empty":
			reason = "memory-completed"
		case "WriteUncertain":
			reason = "memory-write-uncertain"
		}
		return f.ReasonCode == reason
	default:
		return false
	}
}

func lateMemoryOutcome(f facts.Fact) bool {
	return f.Kind == "ProviderOutcome" && f.Memory != nil &&
		(f.Operation == "memory.write" || f.Operation == "memory.read" && (f.Memory.Status == "Cancelled" || f.Memory.Status == "Timeout"))
}
