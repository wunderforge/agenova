// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
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

	allow := runCLI(t, bin, 0, "--backend", "memory", "run", "-f", fixture)
	if !strings.Contains(allow, "request: fix-payment-timeout") || !strings.Contains(allow, "decision: Allow") || !strings.Contains(allow, "allocated: true") || !strings.Contains(allow, "phase: Succeeded") {
		t.Fatalf("allow output: %q", allow)
	}

	deny := runCLIEnv(t, bin, 1, []string{"AGENOVA_LOCAL_PRINCIPAL=team-b"}, "--backend", "memory", "run", "-f", fixture)
	if !strings.Contains(deny, "decision: Deny") || !strings.Contains(deny, "allocated: false") {
		t.Fatalf("deny output: %q", deny)
	}

	secret := runCLI(t, bin, 2, "--backend", "memory", "run", "-f", claimRequestFixture(t, "invalid-secret-value.json"))
	if !strings.Contains(secret, "secret-value") && !strings.Contains(secret, "secrets") {
		t.Fatalf("secret rejection: %q", secret)
	}

	authority := runCLI(t, bin, 2, "--backend", "memory", "run", "-f", fixture, "--repo=acme/payments")
	if !strings.Contains(authority, "does not grant authority through CLI flags") {
		t.Fatalf("authority flag: %q", authority)
	}

	missing := runCLI(t, bin, 2, "--backend", "memory", "run")
	if !strings.Contains(missing, "run requires -f") {
		t.Fatalf("missing file: %q", missing)
	}
}

func TestRunSubmissionPrintsSharedEvidenceJSON(t *testing.T) {
	bin := buildCLI(t)
	fixture := claimRequestFixture(t, "valid-team-a-engineer.yaml")
	for _, tc := range []struct {
		preset string
		exit   int
		result v0.DecisionResult
	}{
		{"team-a", 0, v0.DecisionResultAllow}, {"team-b", 1, v0.DecisionResultDeny},
	} {
		out := runCLIEnv(t, bin, tc.exit, []string{"AGENOVA_LOCAL_PRINCIPAL=" + tc.preset}, "--backend", "memory", "run", "-f", fixture, "--json")
		var view evidence.View
		if err := json.Unmarshal([]byte(out), &view); err != nil {
			t.Fatal(err)
		}
		if view.Version != "agenova.evidence/v0" || view.Request == nil || view.State == nil || view.Outcome == nil || len(view.Facts) < 2 {
			t.Fatalf("incomplete shared view: %s", out)
		}
		if err := v0.ValidateIssuedState(view.State); err != nil {
			t.Fatal(err)
		}
		if view.State.Decision.Result != tc.result {
			t.Fatal("wrong decision")
		}
		if tc.result == v0.DecisionResultDeny {
			if view.State.Claim != nil || view.State.EffectiveAuthority != nil {
				t.Fatal("denial fabricated authority")
			}
		} else {
			if view.State.Claim.Phase != v0.ClaimPhaseSucceeded {
				t.Fatal("missing terminal state")
			}
			cleanup, final := false, false
			for _, fact := range view.Facts {
				cleanup = cleanup || fact.Operation == "CleanupSucceeded"
				final = final || fact.Kind == "RunOutcome"
			}
			if !cleanup || !final {
				t.Fatal("CLI omitted lifecycle/outcome facts")
			}
		}
	}
}

func TestAdapterLifecycleSmoke(t *testing.T) {
	bin := buildCLI(t)
	state := t.TempDir()
	reference := "agenova.io/runtime/agent-sandbox@0.1.0"

	catalog := runCLI(t, bin, 0, "adapters", "catalog", "--json", "--state-dir", state)
	if !strings.Contains(catalog, `"id":"agenova.io/deployment/kubernetes"`) || !strings.Contains(catalog, `"id":"agenova.io/model/openai-compatible"`) {
		t.Fatalf("catalog output: %s", catalog)
	}
	first := runCLI(t, bin, 0, "adapters", "install", reference, "--json", "--state-dir", state)
	if !strings.Contains(first, `"changed":true`) {
		t.Fatalf("first install output: %s", first)
	}
	second := runCLI(t, bin, 0, "adapters", "install", reference, "--json", "--state-dir", state)
	if !strings.Contains(second, `"changed":false`) {
		t.Fatalf("idempotent install output: %s", second)
	}
	list := runCLI(t, bin, 0, "adapters", "list", "--json", "--state-dir", state)
	if strings.Count(list, "agent-sandbox") != 1 {
		t.Fatalf("list output: %s", list)
	}
	fragment := runCLI(t, bin, 0, "adapters", "init", reference, "--name", "primary-runtime", "--state-dir", state)
	if !strings.Contains(fragment, "runtimeBackends:") || !strings.Contains(fragment, "mode: in-cluster") || strings.Contains(fragment, "credential") {
		t.Fatalf("init output: %s", fragment)
	}
	unknown := runCLI(t, bin, 2, "adapters", "inspect", "agenova.io/runtime/missing@0.1.0", "--state-dir", state)
	if !strings.Contains(unknown, "is not available") {
		t.Fatalf("unknown adapter output: %s", unknown)
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
