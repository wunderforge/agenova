// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"strings"
	"testing"
)

func TestNormalizeParameterKey(t *testing.T) {
	cases := map[string]string{
		"githubToken":  "githubtoken",
		"GITHUB_TOKEN": "githubtoken",
		"github-token": "githubtoken",
		"API_KEY":      "apikey",
		"repository":   "repository",
	}
	for input, want := range cases {
		if got := NormalizeParameterKey(input); got != want {
			t.Errorf("NormalizeParameterKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReservedCredentialKey(t *testing.T) {
	// Reserved names, in each spelling the single normalization rule folds.
	reserved := []string{"githubToken", "GITHUB_TOKEN", "github-token", "API_KEY", "password", "credential", "authorization"}
	for _, key := range reserved {
		if !ReservedCredentialKey(key) {
			t.Errorf("ReservedCredentialKey(%q) = false, want true", key)
		}
	}

	// Deterministic set membership only: names that merely contain a reserved
	// word are not rejected, because the contract does not claim to detect
	// arbitrary secrets.
	notReserved := []string{"repository", "objective", "branch", "timeout", "tokenizer", "passwordPolicyRef", "secretsManagerArn"}
	for _, key := range notReserved {
		if ReservedCredentialKey(key) {
			t.Errorf("ReservedCredentialKey(%q) = true, want false: matching must be exact after normalization", key)
		}
	}
}

func TestFindReservedCredentialKey(t *testing.T) {
	if key, found := FindReservedCredentialKey(map[string]string{"repository": "acme/payments", "githubToken": "x"}); !found || key != "githubToken" {
		t.Errorf("FindReservedCredentialKey = (%q, %v), want (githubToken, true)", key, found)
	}
	if _, found := FindReservedCredentialKey(map[string]string{"repository": "acme/payments"}); found {
		t.Error("FindReservedCredentialKey flagged credential-free params")
	}
	if _, found := FindReservedCredentialKey(nil); found {
		t.Error("FindReservedCredentialKey flagged nil params")
	}
}

func TestAmbiguousResourceScope(t *testing.T) {
	ambiguous := []string{"", "   ", "*", "repo:*", "repo:acme/*"}
	for _, scope := range ambiguous {
		if !AmbiguousResourceScope(scope) {
			t.Errorf("AmbiguousResourceScope(%q) = false, want true", scope)
		}
	}
	if AmbiguousResourceScope("repo:acme/payments") {
		t.Error("bounded scope repo:acme/payments flagged as ambiguous")
	}
}

func TestSequenceIDSource(t *testing.T) {
	ids := SequenceIDSource("inv-test")
	first, second := ids(), ids()
	if first != "inv-test-1" || second != "inv-test-2" {
		t.Errorf("sequence ids = %q, %q; want inv-test-1, inv-test-2", first, second)
	}
}

func TestRandomIDSource(t *testing.T) {
	ids := RandomIDSource()
	first, second := ids(), ids()
	if first == second {
		t.Errorf("random source issued duplicate id %q", first)
	}
	for _, id := range []string{first, second} {
		if !strings.HasPrefix(id, "inv-") || len(id) != len("inv-")+32 {
			t.Errorf("id %q is not in inv-<32 hex> form", id)
		}
	}
}

func TestAllowedOutcome(t *testing.T) {
	out := Allowed()
	if out.Result != ResultAllow || out.Category != "" || out.Reason != "" {
		t.Errorf("Allowed() = %+v, want bare Allow", out)
	}
}

// A policy that forgets to set Result must not produce an untyped decision:
// the contract has exactly three values and an unrecognised one fails closed.
func TestOutcomeNormalizeFailsClosed(t *testing.T) {
	for _, out := range []Outcome{
		{},
		{Category: "tool-not-granted", Reason: "capability not in grant"},
		{Result: DecisionResult("Maybe")},
	} {
		got := out.Normalize()
		if got.Result != ResultDeny {
			t.Errorf("Normalize(%+v).Result = %q, want Deny", out, got.Result)
		}
		if got.Category != CategoryInvalidPolicyOutcome {
			t.Errorf("Normalize(%+v).Category = %q, want %q", out, got.Category, CategoryInvalidPolicyOutcome)
		}
	}

	for _, result := range []DecisionResult{ResultAllow, ResultDeny, ResultApprovalRequired} {
		out := Outcome{Result: result, Category: "kept", Reason: "kept"}
		if got := out.Normalize(); got != out {
			t.Errorf("Normalize must not alter a valid %q outcome, got %+v", result, got)
		}
	}
}
