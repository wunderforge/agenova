// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package gatewaytest loads the frozen v0 contract fixtures for gateway
// tests, so governed-invocation evidence exercises the same Team A engineer
// scenario and secret-rejection case the rest of the contract suite uses.
// A design change that invalidates these inputs must update the shared
// fixture set in a reviewed change, not fork private copies here.
package gatewaytest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
)

// TeamAAuthority is the granted-authority slice of the frozen
// issued-state.valid.team-a-engineer fixture.
type TeamAAuthority struct {
	ClaimID        string
	Phase          v1alpha1.ClaimPhase
	Tools          []string
	ResourceScopes []string
	ModelProfile   string
}

// LoadTeamAAuthority reads the Team A engineer issued-state fixture through
// the system-issued v0 parser, so the gateway tests share the same claim and
// authority shape the SandboxClaim contract validates.
func LoadTeamAAuthority(tb testing.TB) TeamAAuthority {
	tb.Helper()

	state, validationErr := v1alpha1.ParseSystemIssuedState(readFixture(tb, "inputs/issued-state/valid-team-a-engineer.json"))
	if validationErr != nil {
		tb.Fatalf("team A issued-state fixture no longer parses as a valid system-issued state: %v", validationErr)
	}
	if state.Claim == nil || state.EffectiveAuthority == nil {
		tb.Fatal("team A issued-state fixture no longer carries a claim and effective authority; realign gateway tests with the shared fixture set")
	}
	authority := state.EffectiveAuthority
	if state.Claim.ID == "" || len(authority.Tools) == 0 || len(authority.ResourceScopes) == 0 || authority.ModelProfile == "" {
		tb.Fatal("team A issued-state fixture no longer carries claim id, tools, resource scopes, and model profile; realign gateway tests with the shared fixture set")
	}
	return TeamAAuthority{
		ClaimID:        state.Claim.ID,
		Phase:          state.Claim.Phase,
		Tools:          authority.Tools,
		ResourceScopes: authority.ResourceScopes,
		ModelProfile:   authority.ModelProfile,
	}
}

// SecretParameterKey returns a credential-bearing key from the frozen
// claim-request.invalid.secret-value fixture. The typed ClaimRequest cannot
// carry a secrets block by design, so the key is read from the raw document
// after confirming the v0 parser still rejects the fixture in the
// secret-value category the manifest expects.
func SecretParameterKey(tb testing.TB) string {
	tb.Helper()

	raw := readFixture(tb, "inputs/claim-request/invalid-secret-value.json")
	if _, validationErr := v1alpha1.ParseClaimRequestJSON(raw); validationErr == nil {
		tb.Fatal("claim-request secret-value fixture is no longer rejected by the v0 parser; realign gateway tests with the shared fixture set")
	} else if validationErr.Category != v1alpha1.ValidationCategorySecretValue {
		tb.Fatalf("claim-request secret-value fixture rejected as %q, want %q", validationErr.Category, v1alpha1.ValidationCategorySecretValue)
	}

	var doc struct {
		Spec struct {
			Secrets map[string]string `json:"secrets"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		tb.Fatalf("parse shared fixture claim-request.invalid.secret-value: %v", err)
	}
	for key := range doc.Spec.Secrets {
		return key
	}
	tb.Fatal("claim-request secret-value fixture no longer carries a secrets entry; realign gateway tests with the shared fixture set")
	return ""
}

func fixturePath(tb testing.TB, rel string) string {
	tb.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("cannot locate gatewaytest source for fixture resolution")
	}
	// self = <repo>/internal/gateway/gatewaytest/fixtures.go
	repoRoot := filepath.Join(filepath.Dir(self), "..", "..", "..")
	return filepath.Join(repoRoot, "harness", "fixtures", "contract", "v0", rel)
}

func readFixture(tb testing.TB, rel string) []byte {
	tb.Helper()
	path := fixturePath(tb, rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read shared fixture %s: %v", path, err)
	}
	return raw
}
