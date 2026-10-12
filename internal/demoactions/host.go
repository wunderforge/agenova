// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package demoactions is an opt-in host-side provider for one isolated,
// synthetic incident. It is not an arbitrary GitHub or Kubernetes adapter.
package demoactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/toolgateway"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

const (
	DemoRepo        = "wunderforge/agenova-payment-incident-demo"
	DemoRepoScope   = "repo:wunderforge/agenova-payment-incident-demo"
	DemoKubeScope   = "k8s:kind-agenova-k8s-lab/agenova-payment-demo/payment-api"
	DemoContext     = "kind-agenova-k8s-lab"
	DemoNamespace   = "agenova-payment-demo"
	DemoDeployment  = "payment-api"
	isolatedGoImage = "golang:1.26-alpine@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c"
)

var pullURL = regexp.MustCompile(`https://github\.com/wunderforge/agenova-payment-incident-demo/pull/[0-9]+`)
var safeInvocation = regexp.MustCompile(`^inv-[a-zA-Z0-9-]{12,80}$`)

type CommandRunner interface {
	Run(context.Context, string, string, ...string) (string, error)
}

type Host struct {
	repositoryPath string
	runner         CommandRunner
}

type failure string

func (f failure) Error() string    { return string(f) }
func (f failure) SafeCode() string { return string(f) }

func NewHost(repositoryPath string, runner CommandRunner) (*Host, error) {
	path, err := filepath.Abs(repositoryPath)
	if err != nil || path == "" {
		return nil, errors.New("absolute demo repository path is required")
	}
	if runner == nil {
		runner = execRunner{}
	}
	return &Host{repositoryPath: path, runner: runner}, nil
}

func (h *Host) Invoke(ctx context.Context, invocationID string, req toolgateway.Request) (console.ToolResult, error) {
	if h == nil || ctx.Err() != nil {
		return console.ToolResult{}, errors.New("demo tool host unavailable")
	}
	switch req.Tool + "." + req.Action {
	case "git.read":
		if req.ResourceScope != DemoRepoScope {
			return console.ToolResult{}, errors.New("repository target mismatch")
		}
		return h.read(req.Parameters["file"])
	case "github.pr.create":
		if req.ResourceScope != DemoRepoScope || !safeInvocation.MatchString(invocationID) {
			return console.ToolResult{}, errors.New("PR target or invocation mismatch")
		}
		return h.createPR(ctx, invocationID, req.Parameters["input"])
	case "kubernetes.rollback":
		if req.ResourceScope != DemoKubeScope || strings.TrimSpace(req.Parameters["input"]) == "" {
			return console.ToolResult{}, errors.New("rollback target or reason mismatch")
		}
		return h.rollback(ctx)
	default:
		return console.ToolResult{}, errors.New("unsupported demo operation")
	}
}

func (h *Host) read(name string) (console.ToolResult, error) {
	switch name {
	case "README.md", "logs/timeout.log", "src/retry.go", "src/retry_test.go":
	default:
		return console.ToolResult{}, errors.New("file outside the demo read allowlist")
	}
	content, err := os.ReadFile(filepath.Join(h.repositoryPath, filepath.FromSlash(name)))
	if err != nil || len(content) > 16<<10 {
		return console.ToolResult{}, errors.New("demo repository file unavailable or too large")
	}
	return console.ToolResult{Reply: workerprotocol.Reply{Allowed: true, Text: string(content)}, AuditTarget: DemoRepo + "/" + name}, nil
}

func (h *Host) createPR(ctx context.Context, invocationID, replacement string) (console.ToolResult, error) {
	replacement = strings.TrimSpace(replacement)
	if strings.HasSuffix(replacement, "```") {
		for _, prefix := range []string{"```go\n", "```\n"} {
			if strings.HasPrefix(replacement, prefix) {
				replacement = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(replacement, prefix), "```"))
				break
			}
		}
	}
	invalid := func(detail string) (console.ToolResult, error) {
		return console.ToolResult{Reply: workerprotocol.Reply{Allowed: false, Error: "PR input must be one complete Go source file in package retry, not a plan or patch. " + detail}, AuditTarget: DemoRepo + "/src/retry.go", FailureCode: "replacement-invalid", FailureNote: "The model output was not a complete valid Go source file; no branch or PR was pushed."}, nil
	}
	if len(replacement) == 0 || len(replacement) > 4096 || strings.Contains(replacement, "```") {
		return invalid("Keep the source within 4096 bytes and omit markdown fences.")
	}
	parsed, parseErr := parser.ParseFile(token.NewFileSet(), "retry.go", replacement, parser.ParseComments)
	if parseErr != nil {
		return invalid("Go syntax error: " + boundedTestFeedback(parseErr.Error()))
	}
	if parsed.Name.Name != "retry" {
		return invalid("The package declaration must be 'package retry'.")
	}
	if _, err := h.run(ctx, "", 10*time.Second, "docker", "image", "inspect", isolatedGoImage); err != nil {
		return console.ToolResult{}, failure("test-sandbox-unavailable")
	}
	parent, err := os.MkdirTemp("", "agenova-payment-pr-")
	if err != nil {
		return console.ToolResult{}, errors.New("temporary checkout unavailable")
	}
	defer os.RemoveAll(parent)
	checkout := filepath.Join(parent, "repository")
	if _, err := h.run(ctx, parent, 45*time.Second, "git", "clone", "--branch", "main", "--single-branch", "https://github.com/"+DemoRepo+".git", checkout); err != nil {
		return console.ToolResult{}, failure("repository-clone-failed")
	}
	branch := "agenova/claim-" + strings.TrimPrefix(invocationID, "inv-")[:12]
	if _, err := h.run(ctx, checkout, 15*time.Second, "git", "checkout", "-b", branch); err != nil {
		return console.ToolResult{}, failure("branch-create-failed")
	}
	if err := os.WriteFile(filepath.Join(checkout, "src", "retry.go"), []byte(replacement), 0644); err != nil {
		return console.ToolResult{}, failure("replacement-write-failed")
	}
	// Model-authored Go is untrusted. Running its tests on the host would let
	// package init and test code access the operator's credentials. The test
	// container has no network, credentials, writeable source, or privileges.
	if testOutput, err := h.testReplacement(ctx, checkout); err != nil {
		// Keep the bounded compiler/test diagnostic in operator logs for
		// rehearsal QA; never print source or local credentials.
		fmt.Fprintln(os.Stderr, "demo PR source failed go test:", boundedTestFeedback(testOutput))
		return console.ToolResult{
			Reply:       workerprotocol.Reply{Allowed: false, Error: "The model-authored source did not pass go test ./...:\n" + boundedTestFeedback(testOutput)},
			AuditTarget: DemoRepo + "/src/retry.go",
			FailureCode: "replacement-tests-failed",
			FailureNote: "The model-authored replacement failed go test; no branch or PR was pushed.",
		}, nil
	}
	if _, err := h.run(ctx, checkout, 15*time.Second, "git", "add", "--", "src/retry.go"); err != nil {
		return console.ToolResult{}, failure("pr-staging-failed")
	}
	if _, err := h.run(ctx, checkout, 15*time.Second, "git", "-c", "user.name=Agenova Demo Agent", "-c", "user.email=demo@users.noreply.github.com", "commit", "-m", "Fix payment retry deadline and idempotency"); err != nil {
		return console.ToolResult{}, failure("pr-commit-failed")
	}
	if _, err := h.run(ctx, checkout, 30*time.Second, "git", "push", "-u", "origin", branch); err != nil {
		return console.ToolResult{}, failure("pr-push-failed")
	}
	output, err := h.run(ctx, checkout, 30*time.Second, "gh", "pr", "create", "--repo", DemoRepo, "--base", "main", "--head", branch, "--title", "Fix payment retry deadline and idempotency", "--body", "Synthetic Agenova incident: model-authored fix through a governed claim-scoped tool. `go test ./...` passed before opening this PR.")
	if err != nil {
		return console.ToolResult{}, failure("pr-create-failed")
	}
	url := pullURL.FindString(output)
	if url == "" {
		return console.ToolResult{}, failure("pr-url-invalid")
	}
	return console.ToolResult{Reply: workerprotocol.Reply{Allowed: true, Text: "Created real pull request " + url + "; model-authored replacement passed go test ./..."}, AuditTarget: url}, nil
}

func (h *Host) testReplacement(ctx context.Context, checkout string) (string, error) {
	return h.run(ctx, "", 90*time.Second, "docker", "run", "--rm", "--pull=never",
		"--network", "none", "--read-only", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges", "--user", "65532:65532",
		"--memory", "512m", "--cpus", "1", "--pids-limit", "64",
		"--tmpfs", "/tmp:rw,exec,nosuid,size=128m,mode=1777",
		"--mount", "type=bind,source="+checkout+",target=/src,readonly",
		"--workdir", "/src", "--env", "GOCACHE=/tmp/gocache",
		"--env", "GOPATH=/tmp/gopath", "--env", "GOTOOLCHAIN=local",
		"--env", "GOPROXY=off", "--env", "CGO_ENABLED=0",
		"--env", "GOMAXPROCS=2",
		isolatedGoImage, "go", "test", "-p", "1", "./...")
}

func (h *Host) rollback(ctx context.Context) (console.ToolResult, error) {
	version, revision, err := h.deploymentState(ctx)
	if err != nil || version != "v2.7" || revision != "2" {
		return console.ToolResult{}, errors.New("demo Deployment is not at expected v2.7 revision 2")
	}
	base := []string{"--context", DemoContext, "-n", DemoNamespace}
	if _, err := h.run(ctx, "", 30*time.Second, "kubectl", append(base, "rollout", "undo", "deployment/"+DemoDeployment, "--to-revision=1")...); err != nil {
		return console.ToolResult{}, errors.New("demo Deployment rollback command failed")
	}
	if _, err := h.run(ctx, "", 90*time.Second, "kubectl", append(base, "rollout", "status", "deployment/"+DemoDeployment, "--timeout=80s")...); err != nil {
		return console.ToolResult{}, errors.New("demo Deployment rollback did not become Ready")
	}
	version, revision, err = h.deploymentState(ctx)
	if err != nil || version != "v2.6" || revision == "2" {
		return console.ToolResult{}, errors.New("demo Deployment rollback could not be verified")
	}
	target := fmt.Sprintf("%s/%s revision %s (%s)", DemoNamespace, DemoDeployment, revision, version)
	return console.ToolResult{Reply: workerprotocol.Reply{Allowed: true, Text: "Real controlled rollback completed: " + target}, AuditTarget: target}, nil
}

func (h *Host) deploymentState(ctx context.Context) (string, string, error) {
	output, err := h.run(ctx, "", 15*time.Second, "kubectl", "--context", DemoContext, "-n", DemoNamespace, "get", "deployment", DemoDeployment, "-o", "json")
	if err != nil {
		return "", "", err
	}
	var state struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Name string `json:"name"`
						Env  []struct {
							Name  string `json:"name"`
							Value string `json:"value"`
						} `json:"env"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
	}
	if json.Unmarshal([]byte(output), &state) != nil {
		return "", "", errors.New("invalid Deployment response")
	}
	for _, container := range state.Spec.Template.Spec.Containers {
		if container.Name == "payment-api" {
			for _, env := range container.Env {
				if env.Name == "PAYMENT_VERSION" {
					return env.Value, state.Metadata.Annotations["deployment.kubernetes.io/revision"], nil
				}
			}
		}
	}
	return "", "", errors.New("demo Deployment version unavailable")
}

func (h *Host) run(parent context.Context, dir string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return h.runner.Run(ctx, dir, name, args...)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output := &boundedOutput{limit: 16 << 10}
	cmd.Stdout, cmd.Stderr = output, output
	err := cmd.Run()
	if err != nil {
		return string(output.data), err // Caller decides whether bounded output is safe to use.
	}
	return string(output.data), nil
}

func boundedTestFeedback(output string) string {
	if len(output) > 1800 {
		output = output[:1800]
	}
	var clean strings.Builder
	for _, r := range output {
		if r == '\n' || r == '\t' || (r >= ' ' && r <= '~') {
			clean.WriteRune(r)
		}
	}
	if clean.Len() == 0 {
		return "Go tests failed without a usable diagnostic; review src/retry_test.go."
	}
	return clean.String()
}

type boundedOutput struct {
	data  []byte
	limit int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(b.data) < b.limit {
		remaining := b.limit - len(b.data)
		b.data = append(b.data, p[:min(len(p), remaining)]...)
	}
	return len(p), nil
}

var _ io.Writer = (*boundedOutput)(nil)
