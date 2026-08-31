// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

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
		if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "version") {
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
	code := Main(args, &stdout, &stderr, factory)
	return stdout.String(), stderr.String(), code
}

type stubBackend struct{}

func (s *stubBackend) AddTemplate(v1alpha1.AgentSandboxTemplate) error { return nil }
func (s *stubBackend) AddWarmPool(v1alpha1.SandboxWarmPool) error      { return nil }
func (s *stubBackend) AddClaim(v1alpha1.SandboxClaim) error            { return nil }
func (s *stubBackend) BindClaim(string) error                          { return nil }
func (s *stubBackend) StartClaim(string) error                         { return nil }
func (s *stubBackend) SucceedClaim(string) error                       { return nil }
func (s *stubBackend) FailClaim(string, string) error                  { return nil }
func (s *stubBackend) ExpireClaim(string, string) error                { return nil }
func (s *stubBackend) Claim(string) (v1alpha1.SandboxClaim, bool) {
	return v1alpha1.SandboxClaim{}, false
}
func (s *stubBackend) PoolStatus(string) (v1alpha1.SandboxWarmPoolStatus, bool) {
	return v1alpha1.SandboxWarmPoolStatus{}, false
}
