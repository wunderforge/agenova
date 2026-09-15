// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/runtime"
)

func TestRunRequiresFile(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runCLI(t, []string{"agenova", "run"}, memoryFactory)
	if code != ExitUsage {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout %q", stdout)
	}
	if !strings.Contains(stderr, "run requires -f") {
		t.Fatalf("stderr %q", stderr)
	}
}

func TestRunRejectsAuthorityFlags(t *testing.T) {
	t.Parallel()
	_, stderr, code := runCLI(t, []string{"agenova", "run", "-f", "request.yaml", "--repo=acme/payments"}, memoryFactory)
	if code != ExitUsage {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stderr, "does not grant authority through CLI flags") {
		t.Fatalf("stderr %q", stderr)
	}
}

func TestRunSubmitsCanonicalFile(t *testing.T) {
	t.Parallel()
	var gotPath string
	handler := func(path string, backend runtime.RuntimeBackend) (RunReport, error) {
		if backend == nil {
			t.Fatal("hosted backend was not supplied")
		}
		gotPath = path
		return RunReport{
			RequestRef: "fix-payment-timeout",
			Decision:   "Allow",
			Principal:  "user:team-a-engineer",
			Allocated:  true,
			ClaimID:    "claim:fix-payment-timeout:test",
			Phase:      "Succeeded",
		}, nil
	}
	var stdout, stderr strings.Builder
	code := Main([]string{"agenova", "run", "-f", filepath.Join("fixtures", "request.yaml")}, &stdout, &stderr, memoryFactory, handler)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if gotPath != filepath.Join("fixtures", "request.yaml") {
		t.Fatalf("submitted %q", gotPath)
	}
	out := stdout.String()
	if !strings.Contains(out, "request: fix-payment-timeout") || !strings.Contains(out, "decision: Allow") || !strings.Contains(out, "allocated: true") || !strings.Contains(out, "phase: Succeeded") {
		t.Fatalf("stdout %q", out)
	}
}

func TestRunFileFlagRequiresValue(t *testing.T) {
	t.Parallel()
	_, stderr, code := runCLI(t, []string{"agenova", "run", "-f", "--help"}, memoryFactory)
	if code != ExitUsage {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stderr, "flag -f requires a value") {
		t.Fatalf("stderr %q", stderr)
	}
}

func TestRunHelp(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runCLI(t, []string{"agenova", "run", "--help"}, memoryFactory)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr %q", stderr)
	}
	if !strings.Contains(stdout, "agenova run -f") {
		t.Fatalf("run help %q", stdout)
	}
	if strings.Contains(stdout, "\n  --repo") || strings.Contains(stdout, "\n  --tools") || strings.Contains(stdout, "\n  --model") {
		t.Fatalf("run help must not advertise authority flags: %q", stdout)
	}
}

func TestRunRejectsUnknownBackendBeforeSubmit(t *testing.T) {
	t.Parallel()
	called := false
	handler := func(string, runtime.RuntimeBackend) (RunReport, error) {
		called = true
		return RunReport{}, nil
	}
	var stdout, stderr strings.Builder
	code := Main([]string{"agenova", "--backend=kubernetes", "run", "-f", "request.yaml"}, &stdout, &stderr, memoryFactory, handler)
	if code != ExitUsage {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if called {
		t.Fatal("unknown backend reached the run handler")
	}
	if !strings.Contains(stderr.String(), `unknown runtime backend "kubernetes"`) {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestRunDenyExitsOne(t *testing.T) {
	t.Parallel()
	handler := func(string, runtime.RuntimeBackend) (RunReport, error) {
		return RunReport{RequestRef: "fix-payment-timeout", Decision: "Deny", Principal: "user:team-b-engineer"}, nil
	}
	var stdout, stderr strings.Builder
	code := Main([]string{"agenova", "run", "-f", "request.yaml"}, &stdout, &stderr, memoryFactory, handler)
	if code != 1 {
		t.Fatalf("exit %d, want 1, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "decision: Deny") {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestRunReportsApplicationAllocation(t *testing.T) {
	t.Parallel()
	handler := func(string, runtime.RuntimeBackend) (RunReport, error) {
		return RunReport{RequestRef: "fix-payment-timeout", Decision: "Allow", Allocated: true, ClaimID: "claim-1", Phase: "Succeeded"}, nil
	}
	var stdout, stderr strings.Builder
	code := Main([]string{"agenova", "run", "-f", "request.yaml"}, &stdout, &stderr, memoryFactory, handler)
	if code != 0 {
		t.Fatalf("exit %d, want 0, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "allocated: true") || !strings.Contains(stdout.String(), "phase: Succeeded") {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestRunPreservesTerminalReportOnOperationalFailure(t *testing.T) {
	t.Parallel()
	handler := func(string, runtime.RuntimeBackend) (RunReport, error) {
		return RunReport{
			RequestRef: "fix-payment-timeout",
			Decision:   "Allow",
			Principal:  "user:team-a-engineer",
			Allocated:  true,
			ClaimID:    "claim-1",
			Phase:      "Failed",
		}, errors.New("cleanup runtime: release unconfirmed")
	}
	var stdout, stderr strings.Builder
	code := Main([]string{"agenova", "run", "-f", "request.yaml"}, &stdout, &stderr, memoryFactory, handler)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "claim: claim-1") || !strings.Contains(stdout.String(), "phase: Failed") {
		t.Fatalf("terminal report lost: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "release unconfirmed") || strings.Contains(stderr.String(), "for usage") {
		t.Fatalf("operational error misclassified: %q", stderr.String())
	}
}

func TestRunTreatsEmptyReportErrorAsInvalidSubmission(t *testing.T) {
	t.Parallel()
	handler := func(string, runtime.RuntimeBackend) (RunReport, error) {
		return RunReport{}, errors.New("invalid ClaimRequest")
	}
	var stdout, stderr strings.Builder
	code := Main([]string{"agenova", "run", "-f", "request.yaml"}, &stdout, &stderr, memoryFactory, handler)
	if code != ExitUsage {
		t.Fatalf("exit %d, want %d", code, ExitUsage)
	}
	if stdout.String() != "" || !strings.Contains(stderr.String(), "for usage") {
		t.Fatalf("invalid submission output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
