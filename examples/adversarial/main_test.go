// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const teamBFixture = "testdata/team-b-unauthorized.yaml"

// TestDeniesUnauthorizedTeamBBeforeClaimCreation is the canonical negative
// case: a well-formed request submitted under the unauthorized Team B
// principal is denied with real Decision evidence and no claim is created.
func TestDeniesUnauthorizedTeamBBeforeClaimCreation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	store := &claimStore{}

	code := run([]string{"--task", teamBFixture}, &stdout, &stderr, store)

	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, exitFailure, stdout.String(), stderr.String())
	}
	if len(store.created) != 0 {
		t.Fatalf("claims created = %v, want none created on a denied submission", store.created)
	}

	out := stdout.String()
	for _, want := range []string{
		fixtureModeLabel,
		"[DENIED]",
		"principal=user:team-b-engineer",
		"team=team-b",
		"policy=reference-default-deny@1",
		"reason=",
		"claimsCreated=0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestMalformedInputExitsNonZeroWithoutClaim covers the malformed-input case:
// a request missing required fields exits non-zero with an actionable error,
// does not panic, and never reaches policy evaluation or claim creation.
func TestMalformedInputExitsNonZeroWithoutClaim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.yaml")
	// Missing spec.task, spec.runtime, and spec.templateRef.
	malformed := []byte("apiVersion: agenova.io/v1alpha1\nkind: ClaimRequest\nmetadata:\n  name: broken\nspec: {}\n")
	if err := os.WriteFile(path, malformed, 0o600); err != nil {
		t.Fatalf("write malformed fixture: %v", err)
	}

	var stdout, stderr bytes.Buffer
	store := &claimStore{}

	code := run([]string{"--task", path}, &stdout, &stderr, store)

	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if len(store.created) != 0 {
		t.Fatalf("claims created = %v, want none on malformed input", store.created)
	}
	if strings.Contains(stdout.String(), "[DENIED]") {
		t.Fatalf("malformed input must not reach policy evaluation:\n%s", stdout.String())
	}
	if strings.TrimSpace(stderr.String()) == "" {
		t.Fatal("malformed input must produce an actionable error on stderr")
	}
}

// TestMissingTaskFlagExitsUsage guards the required --task argument.
func TestMissingTaskFlagExitsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr, &claimStore{})
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "--task is required") {
		t.Fatalf("stderr missing usage guidance:\n%s", stderr.String())
	}
}
