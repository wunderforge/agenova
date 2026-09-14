// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

func TestSubmitPaymentTimeoutAllowsTeamAWithoutAllocation(t *testing.T) {
	spy := &allocateSpy{}
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
	if result.Allocated || spy.allocates != 0 {
		t.Fatalf("allocated = %t, allocate calls = %d", result.Allocated, spy.allocates)
	}
}

func TestSubmitPaymentTimeoutDeniesTeamBWithoutAllocation(t *testing.T) {
	spy := &allocateSpy{}
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
			spy := &allocateSpy{}
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
	spy := &allocateSpy{}
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

func canonicalRequestPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", "valid-team-a-engineer.yaml")
}

type allocateSpy struct {
	allocates int
}

func (s *allocateSpy) Allocate(runtime.AllocateRequest) (runtime.Allocation, error) {
	s.allocates++
	return runtime.Allocation{}, errors.New("allocate must not be called by run -f")
}
func (s *allocateSpy) Observe(v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	return runtime.Observation{}, nil
}
func (s *allocateSpy) Start(v1alpha1.SandboxClaimBackendIdentity) error     { return nil }
func (s *allocateSpy) Terminate(v1alpha1.SandboxClaimBackendIdentity) error { return nil }
func (s *allocateSpy) Cleanup(v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	return runtime.CleanupResult{}, nil
}
