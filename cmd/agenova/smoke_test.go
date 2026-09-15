// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLISmoke(t *testing.T) {
	bin := buildCLI(t)

	helpOut := runCLI(t, bin, 0, "--help")
	if !strings.Contains(helpOut, "Usage:") {
		t.Fatalf("help output: %q", helpOut)
	}

	versionOut := runCLI(t, bin, 0, "version")
	if !strings.Contains(versionOut, "agenova ") || !strings.Contains(versionOut, "runtime-backend: memory") {
		t.Fatalf("version output: %q", versionOut)
	}

	unknown := runCLI(t, bin, 2, "not-a-command")
	if !strings.Contains(unknown, `unknown command "not-a-command"`) {
		t.Fatalf("unknown command output: %q", unknown)
	}

	invalid := runCLI(t, bin, 2, "--backend", "kubernetes", "version")
	if !strings.Contains(invalid, `unknown runtime backend "kubernetes"`) {
		t.Fatalf("invalid backend output: %q", invalid)
	}

	missingValue := runCLI(t, bin, 2, "--backend", "--version")
	if !strings.Contains(missingValue, "flag --backend requires a value") {
		t.Fatalf("missing backend value output: %q", missingValue)
	}
}

func TestRunSubmissionSmoke(t *testing.T) {
	bin := buildCLI(t)
	fixture := claimRequestFixture(t, "valid-team-a-engineer.yaml")

	allow := runCLI(t, bin, 0, "run", "-f", fixture)
	if !strings.Contains(allow, "request: fix-payment-timeout") || !strings.Contains(allow, "decision: Allow") || !strings.Contains(allow, "allocated: true") || !strings.Contains(allow, "phase: Succeeded") {
		t.Fatalf("allow output: %q", allow)
	}

	deny := runCLIEnv(t, bin, 1, []string{"AGENOVA_LOCAL_PRINCIPAL=team-b"}, "run", "-f", fixture)
	if !strings.Contains(deny, "decision: Deny") || !strings.Contains(deny, "allocated: false") {
		t.Fatalf("deny output: %q", deny)
	}

	secret := runCLI(t, bin, 2, "run", "-f", claimRequestFixture(t, "invalid-secret-value.json"))
	if !strings.Contains(secret, "secret-value") && !strings.Contains(secret, "secrets") {
		t.Fatalf("secret rejection: %q", secret)
	}

	authority := runCLI(t, bin, 2, "run", "-f", fixture, "--repo=acme/payments")
	if !strings.Contains(authority, "does not grant authority through CLI flags") {
		t.Fatalf("authority flag: %q", authority)
	}

	missing := runCLI(t, bin, 2, "run")
	if !strings.Contains(missing, "run requires -f") {
		t.Fatalf("missing file: %q", missing)
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "agenova"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = moduleDir(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/agenova: %v\n%s", err, out)
	}
	return bin
}

func claimRequestFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "harness", "fixtures", "contract", "v0", "inputs", "claim-request", name)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(moduleDir(t), "..", ".."))
}

func runCLIEnv(t *testing.T, bin string, wantExit int, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	return finishCLI(t, cmd, wantExit, args)
}

func runCLI(t *testing.T, bin string, wantExit int, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	return finishCLI(t, cmd, wantExit, args)
}

func finishCLI(t *testing.T, cmd *exec.Cmd, wantExit int, args []string) string {
	t.Helper()
	out, err := cmd.CombinedOutput()
	got := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run %v: %v\n%s", args, err, out)
		}
		got = ee.ExitCode()
	}
	if got != wantExit {
		t.Fatalf("run %v: exit %d, want %d\n%s", args, got, wantExit, out)
	}
	return string(out)
}

func moduleDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return dir
}
