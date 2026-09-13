// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"sort"
	"strings"
)

// reservedCredentialKeys is the documented, exact set of `Parameters` keys
// reserved for credential material. Matching is deterministic: a key is
// normalized once (lowercase, then `-` and `_` removed) and compared for
// equality against this set. There is no substring or value inspection, and
// the gateway does not claim to detect arbitrary secrets — provider
// credentials originate only behind adapters.
var reservedCredentialKeys = map[string]struct{}{
	"token":         {},
	"githubtoken":   {},
	"accesstoken":   {},
	"refreshtoken":  {},
	"apikey":        {},
	"accesskey":     {},
	"secretkey":     {},
	"privatekey":    {},
	"password":      {},
	"secret":        {},
	"credential":    {},
	"credentials":   {},
	"authorization": {},
}

// NormalizeParameterKey applies the single documented normalization rule:
// lowercase, then remove "-" and "_".
func NormalizeParameterKey(key string) string {
	lowered := strings.ToLower(key)
	return strings.NewReplacer("-", "", "_", "").Replace(lowered)
}

// ReservedCredentialKey reports whether a request parameter key is one of the
// documented reserved credential names (for example "githubToken", matching
// the frozen claim-request.invalid.secret-value fixture).
func ReservedCredentialKey(key string) bool {
	_, reserved := reservedCredentialKeys[NormalizeParameterKey(key)]
	return reserved
}

// FindReservedCredentialKey returns the reserved credential key that a
// rejection reports. Go map iteration is unordered, so a request carrying more
// than one reserved key would otherwise name a different key — and produce a
// different Reason — on identical input. The matching keys are sorted and the
// first is returned, making the rejection deterministic.
func FindReservedCredentialKey(params map[string]string) (string, bool) {
	var matched []string
	for key := range params {
		if ReservedCredentialKey(key) {
			matched = append(matched, key)
		}
	}
	if len(matched) == 0 {
		return "", false
	}
	sort.Strings(matched)
	return matched[0], true
}

// AmbiguousResourceScope reports whether a resource scope fails to name one
// bounded resource: empty scopes and wildcard scopes cannot be authorized
// against an exact grant.
func AmbiguousResourceScope(scope string) bool {
	trimmed := strings.TrimSpace(scope)
	return trimmed == "" || strings.Contains(trimmed, "*")
}
