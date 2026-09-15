// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"strings"

	"github.com/wunderforge/agenova/api/v1alpha1"
)

// NormalizeParameterKey applies the single documented normalization rule:
// lowercase, then remove "-" and "_".
func NormalizeParameterKey(key string) string {
	return v1alpha1.NormalizeCredentialFieldName(key)
}

// ReservedCredentialKey reports whether a request parameter key is one of the
// documented reserved credential names (for example "githubToken", matching
// the frozen claim-request.invalid.secret-value fixture).
func ReservedCredentialKey(key string) bool {
	return v1alpha1.ReservedCredentialFieldName(key)
}

// FindReservedCredentialKey returns the reserved credential key that a
// rejection reports. Go map iteration is unordered, so a request carrying more
// than one reserved key would otherwise name a different key — and produce a
// different Reason — on identical input. The matching keys are sorted and the
// first is returned, making the rejection deterministic.
func FindReservedCredentialKey(params map[string]string) (string, bool) {
	return v1alpha1.FindReservedCredentialFieldName(params)
}

// AmbiguousResourceScope reports whether a resource scope fails to name one
// bounded resource: empty scopes and wildcard scopes cannot be authorized
// against an exact grant.
func AmbiguousResourceScope(scope string) bool {
	trimmed := strings.TrimSpace(scope)
	return trimmed == "" || strings.Contains(trimmed, "*")
}
