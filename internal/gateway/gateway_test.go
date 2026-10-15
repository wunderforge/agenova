// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"os"
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

// Map iteration is unordered, so a request carrying several reserved keys must
// still name the same key — and therefore report the same Reason — every time.
func TestFindReservedCredentialKeyIsDeterministic(t *testing.T) {
	params := map[string]string{
		"repository":  "acme/payments",
		"githubToken": "x",
		"apiKey":      "y",
		"password":    "z",
	}
	for i := 0; i < 64; i++ {
		key, found := FindReservedCredentialKey(params)
		if !found {
			t.Fatal("FindReservedCredentialKey missed a reserved key")
		}
		if key != "apiKey" {
			t.Fatalf("iteration %d reported %q, want apiKey: the reported key must not vary between identical requests", i, key)
		}
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

func TestParseResourceScopeAcceptsCanonicalPilotResources(t *testing.T) {
	fixture := loadE21ResourceScopeFixture(t)
	for _, value := range fixture.Valid {
		t.Run(value, func(t *testing.T) {
			scope, err := ParseResourceScope(value)
			if err != nil {
				t.Fatalf("ParseResourceScope(%q): %v", value, err)
			}
			if got := scope.String(); got != value {
				t.Fatalf("round trip = %q, want %q", got, value)
			}
		})
	}
}

func TestParseResourceScopeRejectsMalformedOrUnsupportedResources(t *testing.T) {
	fixture := loadE21ResourceScopeFixture(t)
	for _, value := range fixture.Invalid {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseResourceScope(value); err == nil {
				t.Fatalf("ParseResourceScope(%q) succeeded", value)
			}
		})
	}
}

func TestE21OperationResourceMatrix(t *testing.T) {
	fixture := loadE21OperationResourceFixture(t)
	for _, test := range fixture.Allowed {
		t.Run("allow/"+test.Operation+"/"+test.Resource, func(t *testing.T) {
			scope, err := ParseResourceScope(test.Resource)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateToolOperationResource(test.Operation, scope); err != nil {
				t.Fatalf("allowed pair rejected: %v", err)
			}
		})
	}
	for _, test := range fixture.Mismatched {
		t.Run("deny/"+test.Operation+"/"+test.Resource, func(t *testing.T) {
			scope, err := ParseResourceScope(test.Resource)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateToolOperationResource(test.Operation, scope); err == nil {
				t.Fatal("mismatched operation/resource pair was accepted")
			}
		})
	}
	for _, operation := range fixture.Unsupported {
		t.Run("unsupported/"+operation, func(t *testing.T) {
			if !ForbiddenToolOperation(operation) {
				t.Fatal("forbidden operation was not identified")
			}
			scope, err := ParseResourceScope("repo:example/payments")
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateToolOperationResource(operation, scope); err == nil {
				t.Fatal("unsupported operation/resource pair was accepted")
			}
		})
	}
}

func TestLegacyCustomOperationRemainsGovernedByExactAuthority(t *testing.T) {
	if SupportedToolOperation("plan.compose") || ForbiddenToolOperation("plan.compose") {
		t.Fatal("legacy custom operation must remain outside the E21 read-only and forbidden vocabularies")
	}
}

type e21ResourceScopeFixture struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

type e21OperationResourcePair struct {
	Operation string `json:"operation"`
	Resource  string `json:"resource"`
}

type e21OperationResourceFixture struct {
	Allowed     []e21OperationResourcePair `json:"allowed"`
	Mismatched  []e21OperationResourcePair `json:"mismatched"`
	Unsupported []string                   `json:"unsupported"`
}

func loadE21ResourceScopeFixture(t *testing.T) e21ResourceScopeFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/e21-resource-scopes.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture e21ResourceScopeFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Valid) == 0 || len(fixture.Invalid) == 0 {
		t.Fatal("E21 resource-scope fixture requires valid and invalid cases")
	}
	return fixture
}

func loadE21OperationResourceFixture(t *testing.T) e21OperationResourceFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/e21-operation-resources.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture e21OperationResourceFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Allowed) == 0 || len(fixture.Mismatched) == 0 || len(fixture.Unsupported) == 0 {
		t.Fatal("E21 operation/resource fixture requires allowed, mismatched, and unsupported cases")
	}
	return fixture
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
