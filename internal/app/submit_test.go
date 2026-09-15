// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

func TestSubmitPaymentTimeoutAllowsTeamAAndRunsOneClaim(t *testing.T) {
	spy := &runtimeSpy{}
	result, err := SubmitClaimRequestFile(canonicalRequestPath(t), spy, ReferencePrincipalTeamA)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.RequestRef != "fix-payment-timeout" || result.Decision != v1alpha1.DecisionResultAllow {
		t.Fatalf("result = %+v", result)
	}
	if result.Principal != "user:team-a-engineer" {
		t.Fatalf("principal = %q", result.Principal)
	}
	if !result.Allocated || spy.allocates != 1 || spy.starts != 1 || spy.terminates != 1 || spy.cleanups != 1 {
		t.Fatalf("result = %+v, runtime calls = %+v", result, spy)
	}
	if result.ClaimID == "" || result.Phase != v1alpha1.ClaimPhaseSucceeded {
		t.Fatalf("claim outcome = %+v", result)
	}
}

func TestSubmitPaymentTimeoutDeniesTeamBWithoutAllocation(t *testing.T) {
	spy := &runtimeSpy{}
	result, err := SubmitClaimRequestFile(canonicalRequestPath(t), spy, ReferencePrincipalTeamB)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Decision != v1alpha1.DecisionResultDeny {
		t.Fatalf("decision = %q, want Deny", result.Decision)
	}
	if result.Allocated || spy.allocates != 0 {
		t.Fatalf("deny allocated a backend: %+v calls=%d", result, spy.allocates)
	}
}

func TestSubmitRejectsInvalidFixturesBeforeAllocation(t *testing.T) {
	cases := []struct {
		name string
		file string
		want v1alpha1.ValidationCategory
	}{
		{"secret", "invalid-secret-value.json", v1alpha1.ValidationCategorySecretValue},
		{"principal", "invalid-self-asserted-principal.json", v1alpha1.ValidationCategorySelfAssertedPrincipal},
		{"template", "invalid-missing-template.json", v1alpha1.ValidationCategoryRequiredField},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &runtimeSpy{}
			path := filepath.Join("..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", tc.file)
			_, err := SubmitClaimRequestFile(path, spy, ReferencePrincipalTeamA)
			var validation *v1alpha1.ValidationError
			if !errors.As(err, &validation) || validation.Category != tc.want {
				t.Fatalf("error = %#v, want category %q", err, tc.want)
			}
			if spy.allocates != 0 {
				t.Fatalf("invalid request allocated: %d", spy.allocates)
			}
		})
	}
}

func TestPrincipalPresetFromEnv(t *testing.T) {
	t.Setenv(LocalPrincipalEnv, "")
	preset, err := PrincipalPresetFromEnv()
	if err != nil || preset != ReferencePrincipalTeamA {
		t.Fatalf("empty env: %q %v", preset, err)
	}
	t.Setenv(LocalPrincipalEnv, "team-b")
	preset, err = PrincipalPresetFromEnv()
	if err != nil || preset != ReferencePrincipalTeamB {
		t.Fatalf("team-b env: %q %v", preset, err)
	}
	t.Setenv(LocalPrincipalEnv, "team-c")
	if _, err := PrincipalPresetFromEnv(); err == nil {
		t.Fatal("unknown principal preset should fail closed")
	}
}

func TestSubmitRejectsMalformedYAMLBeforeAllocation(t *testing.T) {
	spy := &runtimeSpy{}
	path := filepath.Join(t.TempDir(), "broken.yaml")
	if err := os.WriteFile(path, []byte("apiVersion: [unterminated\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := SubmitClaimRequestFile(path, spy, ReferencePrincipalTeamA)
	var validation *v1alpha1.ValidationError
	if !errors.As(err, &validation) || validation.Category != v1alpha1.ValidationCategoryInvalidDocument {
		t.Fatalf("error = %#v", err)
	}
	if spy.allocates != 0 {
		t.Fatalf("malformed YAML allocated: %d", spy.allocates)
	}
}

func TestReferenceAgentTemplateMatchesCanonicalFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "harness", "fixtures", "contract", "v0", "inputs", "agent-template", "valid-engineer.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	want, validationErr := v1alpha1.ParseAgentTemplateYAML(data)
	if validationErr != nil {
		t.Fatalf("parse fixture: %v", validationErr)
	}
	got, err := referenceAgentTemplate("engineer")
	if err != nil {
		t.Fatalf("reference template: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reference template drifted from canonical fixture\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestReferenceAgentTemplateRejectsUnknownTemplate(t *testing.T) {
	if _, err := referenceAgentTemplate("researcher"); err == nil {
		t.Fatal("unknown template accepted")
	}
}

func canonicalRequestPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", "valid-team-a-engineer.yaml")
}

type runtimeSpy struct {
	allocates  int
	starts     int
	terminates int
	cleanups   int
	claimID    string
	identity   v1alpha1.SandboxClaimBackendIdentity
}

func (s *runtimeSpy) Allocate(request runtime.AllocateRequest) (runtime.Allocation, error) {
	s.allocates++
	s.claimID = request.ClaimID
	s.identity = v1alpha1.SandboxClaimBackendIdentity{Backend: "spy", WorkerID: "worker-1"}
	return runtime.Allocation{ClaimID: request.ClaimID, Identity: s.identity}, nil
}
func (s *runtimeSpy) Observe(identity v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	return runtime.Observation{ClaimID: s.claimID, Identity: identity, Ready: true}, nil
}
func (s *runtimeSpy) Start(v1alpha1.SandboxClaimBackendIdentity) error {
	s.starts++
	return nil
}
func (s *runtimeSpy) Terminate(v1alpha1.SandboxClaimBackendIdentity) error {
	s.terminates++
	return nil
}
func (s *runtimeSpy) Cleanup(identity v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	s.cleanups++
	return runtime.CleanupResult{Identity: identity, Released: true}, nil
}
