// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/cli"
	"github.com/wunderforge/agenova/internal/runtime"
)

func TestVersionHostsInMemoryReferenceBackend(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{"agenova", "version"}, &stdout, &stderr, app.NewRuntime, nil)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "agenova "+cli.Version) {
		t.Fatalf("version output %q", out)
	}
	if !strings.Contains(out, "runtime-backend: memory") {
		t.Fatalf("executable did not host the in-memory backend: %q", out)
	}
}

func TestInvalidBackendUsesCompositionRootError(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{"agenova", "--backend=agentsandbox", "version"}, &stdout, &stderr, app.NewRuntime, nil)
	if code != cli.ExitUsage {
		t.Fatalf("exit %d, want %d, stderr %q", code, cli.ExitUsage, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown runtime backend "agentsandbox"`) {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestRunSubmitsCanonicalRequestThroughCompositionRoot(t *testing.T) {
	handler := func(path string, backend runtime.RuntimeBackend) (cli.RunReport, error) {
		result, err := app.SubmitClaimRequestFile(path, backend, app.ReferencePrincipalTeamA)
		if err != nil {
			return cli.RunReport{}, err
		}
		return cli.RunReport{
			RequestRef: result.RequestRef,
			Decision:   string(result.Decision),
			Principal:  result.Principal,
			Allocated:  result.Allocated,
		}, nil
	}

	path := filepath.Join("..", "..", "harness", "fixtures", "contract", "v0", "inputs", "claim-request", "valid-team-a-engineer.yaml")
	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{"agenova", "run", "-f", path}, &stdout, &stderr, app.NewRuntime, handler)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "request: fix-payment-timeout") || !strings.Contains(out, "decision: Allow") || !strings.Contains(out, "allocated: false") {
		t.Fatalf("stdout %q", out)
	}
}
