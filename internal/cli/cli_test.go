// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/runtime"
)

func TestMemoryEvidencePrivacyAcrossCLIOutputs(t *testing.T) {
	view := evidence.View{Version: "agenova.evidence/v0", RequestRef: "private-test", Request: &v1alpha1.ClaimRequest{
		Spec: v1alpha1.ClaimRequestSpec{Task: &v1alpha1.ClaimRequestTask{
			Type: "investigation", Input: map[string]any{"objective": "private-input-sentinel"},
		}, RequestedAccess: v1alpha1.ClaimRequestedAccess{MemoryScopes: []string{"team-docs"}}},
	}, Outcome: &evidence.Outcome{Status: "Succeeded", Text: "private-output-sentinel"}}
	for _, value := range []any{view, &view, []evidence.View{view}} {
		var out, diagnostic bytes.Buffer
		if printJSON(&out, &diagnostic, value) != 0 || !json.Valid(out.Bytes()) {
			t.Fatal("CLI JSON output failed")
		}
		if strings.Contains(out.String(), "sentinel") || !strings.Contains(out.String(), `"input":{}`) || !strings.Contains(out.String(), "contentRedactions") {
			t.Fatal("CLI JSON leaked content or omitted projection markers")
		}
	}
	var out, diagnostic bytes.Buffer
	if err := printRunReport(&out, RunReport{Evidence: &view}, true); err != nil || strings.Contains(out.String(), "sentinel") {
		t.Fatal("CLI submission JSON leaked content")
	}
	out.Reset()
	services := Services{ShowConnected: func(string, string) (evidence.View, error) { return view, nil }}
	if printWork(&out, &diagnostic, parsedArgs{operands: []string{"show", view.RequestRef}}, services) != 0 || strings.Contains(out.String(), "sentinel") || !strings.Contains(out.String(), "content: withheld") {
		t.Fatal("CLI text leaked content or failed to show redaction")
	}
	if view.Outcome.Text == "" || len(view.Request.Spec.Task.Input) == 0 {
		t.Fatal("CLI mutated private data")
	}
}

func TestWorkPhaseApprovalRequiredMatchesPortal(t *testing.T) {
	view := evidence.View{State: &v1alpha1.IssuedState{Decision: v1alpha1.Decision{Result: v1alpha1.DecisionResultApprovalRequired}}, Outcome: &evidence.Outcome{Status: "ApprovalRequired"}}
	if got := workPhase(view); got != "Approval required" {
		t.Fatalf("approval phase = %q", got)
	}
}

func TestHelpAndVersion(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"agenova"},
		{"agenova", "--help"},
		{"agenova", "-h"},
		{"agenova", "help"},
	} {
		stdout, stderr, code := runCLI(t, args, nil)
		if code != 0 {
			t.Fatalf("%v: exit %d, stderr %q", args, code, stderr)
		}
		if stderr != "" {
			t.Fatalf("%v: unexpected stderr %q", args, stderr)
		}
		if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "version") || !strings.Contains(stdout, "run") {
			t.Fatalf("%v: help output missing usage: %q", args, stdout)
		}
		if strings.Contains(stdout, "\n  --repo") || strings.Contains(stdout, "\n  --tools") || strings.Contains(stdout, "\n  --model") {
			t.Fatalf("%v: help must not advertise authority flags: %q", args, stdout)
		}
		if !strings.Contains(stdout, "does not accept") {
			t.Fatalf("%v: help should say authority flags are not accepted: %q", args, stdout)
		}
	}

	stdout, stderr, code := runCLI(t, []string{"agenova", "version"}, memoryFactory)
	if code != 0 {
		t.Fatalf("version exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "agenova "+Version) {
		t.Fatalf("version output %q", stdout)
	}
	if !strings.Contains(stdout, "runtime-backend: memory") {
		t.Fatalf("version did not report hosted backend: %q", stdout)
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runCLI(t, []string{"agenova", "definitely-not-a-command"}, memoryFactory)
	if code != ExitUsage {
		t.Fatalf("exit %d, want %d", code, ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout should be empty, got %q", stdout)
	}
	if !strings.Contains(stderr, `unknown command "definitely-not-a-command"`) {
		t.Fatalf("stderr %q", stderr)
	}
	if !strings.Contains(stderr, "agenova --help") {
		t.Fatalf("stderr should be actionable: %q", stderr)
	}
}

func TestInvalidConfiguration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unknown backend",
			args: []string{"agenova", "--backend", "kubernetes", "version"},
			want: `unknown runtime backend "kubernetes"`,
		},
		{
			name: "empty backend",
			args: []string{"agenova", "--backend=", "version"},
			want: "flag --backend requires a value",
		},
		{
			name: "missing backend value",
			args: []string{"agenova", "--backend"},
			want: "flag --backend requires a value",
		},
		{
			name: "backend value is another flag",
			args: []string{"agenova", "--backend", "--version"},
			want: "flag --backend requires a value",
		},
		{
			name: "backend value is help flag",
			args: []string{"agenova", "--backend", "--help"},
			want: "flag --backend requires a value",
		},
		{
			name: "authority flag",
			args: []string{"agenova", "--repo=acme/payments"},
			want: "does not grant authority through CLI flags",
		},
		{
			name: "unknown flag",
			args: []string{"agenova", "--nope"},
			want: `unknown flag "--nope"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, code := runCLI(t, tc.args, memoryFactory)
			if code != ExitUsage {
				t.Fatalf("exit %d, want %d, stderr %q", code, ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout should be empty, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr %q, want substring %q", stderr, tc.want)
			}
		})
	}
}

func TestFactoryAcceptsTestDouble(t *testing.T) {
	t.Parallel()

	stub := &stubBackend{}
	factory := func(string) (runtime.RuntimeBackend, string, error) {
		return stub, "double", nil
	}

	stdout, stderr, code := runCLI(t, []string{"agenova", "--backend", "memory", "version"}, factory)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "runtime-backend: double") {
		t.Fatalf("injected double was not hosted: %q", stdout)
	}
}

func memoryFactory(name string) (runtime.RuntimeBackend, string, error) {
	if name == "" || name == "memory" {
		return &stubBackend{}, "memory", nil
	}
	return nil, "", errUnknownBackend(name)
}

func errUnknownBackend(name string) error {
	return errorString("unknown runtime backend \"" + name + "\"\nThis composition root supports \"memory\" (the in-memory reference backend).\nProvider backends are not selected from the CLI")
}

type errorString string

func (e errorString) Error() string { return string(e) }

func runCLI(t *testing.T, args []string, factory RuntimeFactory) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Main(args, &stdout, &stderr, factory, nil)
	return stdout.String(), stderr.String(), code
}

type stubBackend struct{}

func (s *stubBackend) Allocate(runtime.AllocateRequest) (runtime.Allocation, error) {
	return runtime.Allocation{}, nil
}
func (s *stubBackend) Observe(v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	return runtime.Observation{}, nil
}
func (s *stubBackend) Start(v1alpha1.SandboxClaimBackendIdentity) error     { return nil }
func (s *stubBackend) Terminate(v1alpha1.SandboxClaimBackendIdentity) error { return nil }
func (s *stubBackend) Cleanup(v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	return runtime.CleanupResult{}, nil
}
