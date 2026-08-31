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

func runCLI(t *testing.T, bin string, wantExit int, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
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
