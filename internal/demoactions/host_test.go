// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package demoactions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wunderforge/agenova/internal/toolgateway"
)

type countingRunner struct{ calls int }

func (r *countingRunner) Run(context.Context, string, string, ...string) (string, error) {
	r.calls++
	return "", nil
}

type unavailableSandboxRunner struct{ calls []string }

func (r *unavailableSandboxRunner) Run(_ context.Context, _ string, name string, _ ...string) (string, error) {
	r.calls = append(r.calls, name)
	return "", errors.New("sandbox unavailable")
}

func TestMissingIsolatedImageBlocksPRBeforeClone(t *testing.T) {
	runner := &unavailableSandboxRunner{}
	host, err := NewHost(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.Invoke(context.Background(), "inv-123456789012", toolgateway.Request{Tool: "github", Action: "pr.create", ResourceScope: DemoRepoScope, Parameters: map[string]string{"input": "package retry\n"}})
	var safe failure
	if !errors.As(err, &safe) || safe.SafeCode() != "test-sandbox-unavailable" {
		t.Fatalf("missing test container = %v, want safe preflight failure", err)
	}
	if !slices.Equal(runner.calls, []string{"docker"}) {
		t.Fatalf("external operations after unavailable test sandbox: %v", runner.calls)
	}
}

func TestValidGoSourceMayStartWithLicenseComment(t *testing.T) {
	runner := &unavailableSandboxRunner{}
	host, err := NewHost(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	input := "// Copyright 2026 contributors\n// SPDX-License-Identifier: Apache-2.0\n\npackage retry\n\nfunc ShouldRetry() bool { return false }\n"
	_, err = host.Invoke(context.Background(), "inv-123456789012", toolgateway.Request{Tool: "github", Action: "pr.create", ResourceScope: DemoRepoScope, Parameters: map[string]string{"input": input}})
	var safe failure
	if !errors.As(err, &safe) || safe.SafeCode() != "test-sandbox-unavailable" {
		t.Fatalf("commented valid Go source was rejected before sandbox preflight: %v", err)
	}
}

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return "ok", nil
}

func TestModelAuthoredTestsAreIsolatedFromHost(t *testing.T) {
	runner := &recordingRunner{}
	host, err := NewHost(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(t.TempDir(), "repository")
	if _, err := host.testReplacement(context.Background(), checkout); err != nil {
		t.Fatal(err)
	}
	if runner.name != "docker" {
		t.Fatalf("untrusted tests ran with %q, not Docker", runner.name)
	}
	for _, required := range []string{"none", "--read-only", "ALL", "no-new-privileges", "65532:65532", "GOPROXY=off", "CGO_ENABLED=0", "type=bind,source=" + checkout + ",target=/src,readonly", isolatedGoImage} {
		if !slices.Contains(runner.args, required) {
			t.Errorf("isolated test omitted %q", required)
		}
	}
	if !slices.Equal(runner.args[len(runner.args)-6:], []string{isolatedGoImage, "go", "test", "-p", "1", "./..."}) {
		t.Errorf("unexpected test entrypoint: %q", runner.args[len(runner.args)-6:])
	}
}

func TestHostRejectsUnsafeTargetsBeforeExternalCommand(t *testing.T) {
	runner := &countingRunner{}
	host, err := NewHost(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	cases := []toolgateway.Request{
		{Tool: "git", Action: "read", ResourceScope: "repo:other/repo", Parameters: map[string]string{"file": "README.md"}},
		{Tool: "git", Action: "read", ResourceScope: DemoRepoScope, Parameters: map[string]string{"file": "../private.txt"}},
		{Tool: "github", Action: "pr.create", ResourceScope: "repo:other/repo", Parameters: map[string]string{"input": "package retry"}},
		{Tool: "kubernetes", Action: "rollback", ResourceScope: "k8s:other", Parameters: map[string]string{"input": "rollback"}},
		{Tool: "kubernetes", Action: "rollback", ResourceScope: DemoKubeScope},
	}
	for _, request := range cases {
		if _, err := host.Invoke(context.Background(), "inv-123456789012", request); err == nil {
			t.Fatalf("unsafe request accepted: %s.%s", request.Tool, request.Action)
		}
	}
	invalid, err := host.Invoke(context.Background(), "inv-123456789012", toolgateway.Request{Tool: "github", Action: "pr.create", ResourceScope: DemoRepoScope, Parameters: map[string]string{"input": "please fix it"}})
	if err != nil || invalid.Reply.Allowed || invalid.FailureCode != "replacement-invalid" {
		t.Fatalf("prose was not safely rejected as model-authored Go source: %#v, %v", invalid, err)
	}
	if runner.calls != 0 {
		t.Fatalf("external runner invoked %d times for rejected requests", runner.calls)
	}
}

func TestHostReadsOnlyDedicatedArtifact(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("synthetic incident"), 0600); err != nil {
		t.Fatal(err)
	}
	host, err := NewHost(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := host.Invoke(context.Background(), "inv-123456789012", toolgateway.Request{Tool: "git", Action: "read", ResourceScope: DemoRepoScope, Parameters: map[string]string{"file": "README.md"}})
	if err != nil || !result.Reply.Allowed || result.Reply.Text != "synthetic incident" || result.AuditTarget != DemoRepo+"/README.md" {
		t.Fatalf("bounded artifact read failed: %#v, %v", result, err)
	}
}

func TestBoundedOutputNeverBuffersPastLimit(t *testing.T) {
	b := &boundedOutput{limit: 4}
	if n, err := b.Write([]byte("abcdef")); n != 6 || err != nil || string(b.data) != "abcd" {
		t.Fatalf("bounded writer = %d, %v, %q", n, err, b.data)
	}
}
