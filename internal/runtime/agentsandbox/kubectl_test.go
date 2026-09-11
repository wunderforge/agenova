// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"
)

// fakeCommand replaces the kubectl executable while leaving argument
// construction and result classification in kubectlRunner under test.
type fakeCommand struct {
	calls  [][]string
	handle func(ctx context.Context, args []string) ([]byte, error)
}

func (f *fakeCommand) run(ctx context.Context, _ []byte, args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	return f.handle(ctx, args)
}

func newTestRunner(f *fakeCommand) *kubectlRunner {
	r := newKubectlRunner("kind-test", "agenova-test")
	r.command = f.run
	r.timeout = 200 * time.Millisecond
	return r
}

func TestExists_presentWhenNameReturned(t *testing.T) {
	f := &fakeCommand{handle: func(context.Context, []string) ([]byte, error) {
		return []byte("sandboxclaim.extensions.agents.x-k8s.io/agenova-claim-a\n"), nil
	}}
	present, err := newTestRunner(f).exists("sandboxclaims", "agenova-claim-a")
	if err != nil || !present {
		t.Fatalf("exists = %t, %v; want true, nil", present, err)
	}
	args := strings.Join(f.calls[0], " ")
	for _, want := range []string{"--context kind-test", "--namespace agenova-test", "get sandboxclaims agenova-claim-a", "--ignore-not-found=true", "-o name"} {
		if !strings.Contains(args, want) {
			t.Fatalf("args %q missing %q", args, want)
		}
	}
}

func TestExists_absentOnlyOnCleanExitWithEmptyOutput(t *testing.T) {
	f := &fakeCommand{handle: func(context.Context, []string) ([]byte, error) {
		return []byte("  \n"), nil
	}}
	present, err := newTestRunner(f).exists("sandboxclaims", "agenova-claim-a")
	if err != nil || present {
		t.Fatalf("exists = %t, %v; want false, nil", present, err)
	}
}

func TestExists_queryFailureIsAnErrorNotAbsence(t *testing.T) {
	failures := map[string][]byte{
		"credential plugin":  []byte(`error: exec plugin: executable file not found in $PATH`),
		"not found in diag":  []byte(`error: the server could not find the requested resource (NotFound: CRD missing)`),
		"connection refused": []byte(`The connection to the server 127.0.0.1:6443 was refused`),
	}
	for name, output := range failures {
		t.Run(name, func(t *testing.T) {
			f := &fakeCommand{handle: func(context.Context, []string) ([]byte, error) {
				return output, &exec.ExitError{}
			}}
			present, err := newTestRunner(f).exists("sandboxclaims", "agenova-claim-a")
			if err == nil {
				t.Fatalf("query failure classified as absence: exists=%t", present)
			}
			if present {
				t.Fatal("query failure must not report presence either")
			}
		})
	}
}

func TestRunner_appliesCommandDeadline(t *testing.T) {
	f := &fakeCommand{handle: func(ctx context.Context, _ []string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("command context carries no deadline")
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	r := newTestRunner(f)
	start := time.Now()
	_, err := r.exists("sandboxclaims", "slow")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("deadline not enforced: took %s", elapsed)
	}
}

// TestExecKubectl_killsHungSubprocess proves the production commandFunc
// terminates a real subprocess at the deadline rather than waiting on it.
func TestExecKubectl_killsHungSubprocess(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("shell helper is POSIX only")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "kubectl")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := execKubectl(ctx, nil, "get", "x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("hung subprocess not killed at deadline: took %s", elapsed)
	}
}
