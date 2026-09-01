// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/cli"
)

func TestVersionHostsInMemoryReferenceBackend(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{"agenova", "version"}, &stdout, &stderr, app.NewRuntime)
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
	code := cli.Main([]string{"agenova", "--backend=agentsandbox", "version"}, &stdout, &stderr, app.NewRuntime)
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
