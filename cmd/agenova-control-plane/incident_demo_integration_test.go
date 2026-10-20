// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
//go:build integration

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/policy"
	runtimebackend "github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type incidentLog struct {
	sync.Mutex
	data bytes.Buffer
}

func (l *incidentLog) Write(b []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.data.Write(b)
}
func (l *incidentLog) entries() []map[string]any {
	l.Lock()
	defer l.Unlock()
	var result []map[string]any
	for _, line := range bytes.Split(l.data.Bytes(), []byte("\n")) {
		var item map[string]any
		if json.Unmarshal(line, &item) == nil {
			result = append(result, item)
		}
	}
	return result
}
func (l *incidentLog) calls() int {
	n := 0
	for _, e := range l.entries() {
		if e["event"] == "receipt" && e["rpcMethod"] == "tools/call" {
			n++
		}
	}
	return n
}

// Model replies are scripted, not inference. Actual worker and SDK/file I/O
// are separate child processes; allocation is the labelled memory reference.
type incidentScriptedModel struct {
	mu    sync.Mutex
	turns map[string]int
}

func (m *incidentScriptedModel) Complete(_ context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix, scope := "payments/INC-P1042/", "repo:demo/payments"
	files := []string{"logs.txt", "retry.txt"}
	if strings.Contains(r.Prompt, "INC-B2081") {
		prefix, scope = "billing/INC-B2081/", "repo:demo/billing"
		files = []string{"logs.txt", "job-config.txt"}
	}
	n := m.turns[scope]
	m.turns[scope]++
	a := workerprotocol.Action{Action: "finish", Answer: "Scripted integration check: both assigned evidence files were read."}
	if n < 2 {
		a = workerprotocol.Action{Action: "tool", Tool: "repo.read", Resource: scope, Input: prefix + files[n]}
	}
	b, _ := json.Marshal(a)
	return modelprovider.Result{Text: string(b), Model: "scripted-integration-not-inference", ResponseID: fmt.Sprintf("script-%s-%d", scope, n)}, nil
}

type incidentExecutor struct {
	binary   string
	log      *incidentLog
	mu       sync.Mutex
	tasks    []workerprotocol.Task
	handlers []workerprotocol.Handler
}

func (e *incidentExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, handle workerprotocol.Handler) (string, error) {
	if len(task.Tools) != 1 {
		return "", errors.New("worker catalog not narrowed")
	}
	e.mu.Lock()
	e.tasks = append(e.tasks, task)
	e.handlers = append(e.handlers, handle)
	e.mu.Unlock()
	own := task.Tools[0]
	otherScope, otherFile := "repo:demo/billing", "billing/INC-B2081/logs.txt"
	if own.ResourceScope == otherScope {
		otherScope, otherFile = "repo:demo/payments", "payments/INC-P1042/logs.txt"
	}
	before := e.log.calls()
	op := workerprotocol.Operation{ClaimID: task.ClaimID, WorkloadToken: task.ToolToken, Kind: "tool", Tool: "repo.read", ResourceScope: otherScope, Input: otherFile}
	reply, err := handle(ctx, op)
	if err != nil || reply.Allowed {
		return "", errors.New("Running cross-scope probe was not denied")
	}
	op.ResourceScope = own.ResourceScope
	reply, err = handle(ctx, op)
	if err == nil || reply.Allowed {
		return "", errors.New("disguised-path probe was not rejected")
	}
	if e.log.calls() != before {
		return "", errors.New("negative probe reached MCP")
	}
	cmd := exec.CommandContext(ctx, e.binary)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return "", err
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	encoder := json.NewEncoder(stdin)
	if err = encoder.Encode(task); err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	for n := 0; scanner.Scan() && n < workerprotocol.MaxOperations+1; n++ {
		var msg workerprotocol.Message
		if json.Unmarshal(scanner.Bytes(), &msg) != nil {
			return "", errors.New("invalid Worker message")
		}
		if msg.Operation != nil {
			r, callErr := handle(ctx, *msg.Operation)
			if callErr != nil {
				return "", callErr
			}
			if err = encoder.Encode(r); err != nil {
				return "", err
			}
			continue
		}
		if msg.Result != "" {
			_ = stdin.Close()
			if cmd.Wait() != nil {
				return "", errors.New("Worker exit failed")
			}
			return msg.Result, nil
		}
	}
	return "", errors.New("Worker returned no result")
}

type incidentAllocations struct {
	runtimebackend.RuntimeBackend
	count int
}

func (a *incidentAllocations) Allocate(req runtimebackend.AllocateRequest) (runtimebackend.Allocation, error) {
	a.count++
	return a.RuntimeBackend.Allocate(req)
}

// Preserve the underlying reference runtime's simulated network lifecycle.
// Embedding only RuntimeBackend would otherwise erase this required capability.
func (a *incidentAllocations) ActivateNetwork(id v0.SandboxClaimBackendIdentity) (runtimebackend.NetworkEvidence, error) {
	network, ok := a.RuntimeBackend.(runtimebackend.NetworkBackend)
	if !ok {
		return runtimebackend.NetworkEvidence{}, runtimebackend.ErrUnsupported
	}
	return network.ActivateNetwork(id)
}

func (a *incidentAllocations) RevokeNetwork(id v0.SandboxClaimBackendIdentity) (runtimebackend.NetworkEvidence, error) {
	network, ok := a.RuntimeBackend.(runtimebackend.NetworkBackend)
	if !ok {
		return runtimebackend.NetworkEvidence{}, runtimebackend.ErrUnsupported
	}
	return network.RevokeNetwork(id)
}

func incidentBuild(t *testing.T, dir, pkg, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	output := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", output, pkg)
	cmd.Dir = dir
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build %s failed: %s", name, b)
	}
	return output
}
func incidentServer(t *testing.T) (*incidentLog, string, string) {
	t.Helper()
	root, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	binary := incidentBuild(t, filepath.Join(root, "harness", "integration", "mcpfixture"), ".", "mcpfixture")
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	log := &incidentLog{}
	tokenPath := filepath.Join(t.TempDir(), "synthetic-fixture-auth")
	if e := os.WriteFile(tokenPath, []byte("synthetic-demo-token-not-a-real-secret"), 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(binary)
	cmd.Env = []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "FIXTURE_") && !strings.HasPrefix(value, "POD_NAME=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "FIXTURE_ADDR="+addr, "FIXTURE_DATA_DIR="+filepath.Join(root, "harness", "demo", "incident-mcp", "data"), "FIXTURE_TOKEN_FILE="+tokenPath)
	cmd.Stdout = log
	cmd.Stderr = io.Discard
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	for deadline := time.Now().Add(10 * time.Second); ; {
		response, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("local SDK fixture not healthy")
		}
		time.Sleep(30 * time.Millisecond)
	}
	// Local TLS bridge with a trusted test CA, not InsecureSkipVerify or a
	// relaxed endpoint allowlist. The SDK sees its real loopback server Host.
	target, _ := url.Parse("http://" + addr)
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) { director(r); r.Host = target.Host }
	front := httptest.NewTLSServer(proxy)
	t.Cleanup(front.Close)
	original := http.DefaultTransport
	transport := original.(*http.Transport).Clone()
	transport.TLSClientConfig = front.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original; transport.CloseIdleConnections() })
	return log, root, front.URL + "/mcp"
}
func incidentHTTP(t *testing.T, endpoint, method, path string, data []byte) evidence.View {
	t.Helper()
	req, e := http.NewRequest(method, endpoint+path, bytes.NewReader(data))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Content-Type", "application/json")
	response, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("API returned %d", response.StatusCode)
	}
	var view evidence.View
	if json.NewDecoder(response.Body).Decode(&view) != nil {
		t.Fatal("invalid evidence response")
	}
	return view
}
func TestIncidentDemoLocalSDKAndWorker(t *testing.T) {
	log, root, endpoint := incidentServer(t)
	parsed, validation := v0.ParsePlatformYAML(incidentRead(t, "platform.yaml"))
	if validation != nil {
		t.Fatal(validation)
	}
	parsed.Spec.Services.ToolBackends[0].Config["endpoint"] = endpoint
	registry, e := bundled.NewRegistry()
	if e != nil {
		t.Fatal(e)
	}
	resolved, _, failure := platform.Resolve(parsed, registry)
	if failure != nil {
		t.Fatal(failure)
	}
	tools, e := buildInstalledTools(resolved, registry, nil)
	if e != nil {
		t.Fatal(e)
	}
	backend, _, e := app.NewRuntime("memory")
	if e != nil {
		t.Fatal(e)
	}
	allocations := &incidentAllocations{RuntimeBackend: backend}
	executor := &incidentExecutor{binary: incidentBuild(t, root, "./cmd/agenova-demo-worker", "worker"), log: log}
	service, e := console.NewServiceWithOptions(allocations, executor, &incidentScriptedModel{turns: map[string]int{}}, app.ReferencePrincipalTeamA, console.Options{ToolBackend: tools, Prepare: func(data []byte) (app.PreparedAssignment, error) {
		// Reuse the immutable policy/template and real resolver, not issued-state edits.
		template, validation := v0.ParseAgentTemplateYAML(incidentRead(t, "template.yaml"))
		if validation != nil {
			return app.PreparedAssignment{}, validation
		}
		bundle, err := policy.ParseDocumentYAML(incidentRead(t, "policy.yaml"))
		if err != nil {
			return app.PreparedAssignment{}, err
		}
		loader := &policy.Loader{}
		if err = loader.Load(bundle); err != nil {
			return app.PreparedAssignment{}, err
		}
		principal, err := app.NewReferencePrincipalSource(app.ReferencePrincipalTeamA)
		if err != nil {
			return app.PreparedAssignment{}, err
		}
		return app.PrepareAssignment(data, principal, loader, incidentTemplate{template})
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer service.Close()
	api := httptest.NewServer(console.Handler(service))
	defer api.Close()
	var views []evidence.View
	for _, file := range []string{"work-payments.yaml", "work-billing.yaml"} {
		request, err := v0.ParseClaimRequestYAML(incidentRead(t, file))
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(request)
		view := incidentHTTP(t, api.URL, "POST", "/api/requests", data)
		for deadline := time.Now().Add(20 * time.Second); view.Outcome == nil; {
			if time.Now().After(deadline) {
				t.Fatal("local Work timeout")
			}
			time.Sleep(20 * time.Millisecond)
			view = incidentHTTP(t, api.URL, "GET", "/api/requests/"+view.RequestRef+"/evidence", nil)
		}
		if view.Outcome.Status != "Succeeded" {
			for _, fact := range view.Facts {
				if fact.Kind == "ProviderOutcome" {
					t.Logf("provider outcome: %s %s %s", fact.Operation, fact.ProviderStatus, fact.ReasonCode)
				}
			}
			t.Logf("safe SDK receipts: %+v", log.entries())
			t.Fatalf("Work %s: %+v", file, view.Outcome)
		}
		views = append(views, view)
		decisions, rejected, calls, networkActive, networkRevoked := 0, 0, 0, 0, 0
		for _, f := range view.Facts {
			if f.Kind == "Runtime" && f.Operation == app.EventNetworkActive && strings.Contains(f.Reason, "Simulated") {
				networkActive++
			}
			if f.Kind == "Runtime" && f.Operation == app.EventNetworkRevoked && strings.Contains(f.Reason, "Simulated") {
				networkRevoked++
			}
			if f.Kind == "ToolDecision" && f.Result == v0.DecisionResultDeny {
				decisions++
			}
			if f.Kind == "WorkerActivity" && f.ReasonCode == "tool-arguments-rejected" {
				rejected++
			}
			if f.Kind == "ProviderAttempt" && f.Operation == "tool.invoke" {
				calls++
				matched := false
				for _, entry := range log.entries() {
					if entry["event"] == "tool" && entry["correlation"] == f.InvocationID && entry["outcome"] == "ok" {
						matched = true
					}
				}
				if !matched {
					t.Fatal("missing independent SDK read receipt")
				}
			}
		}
		if decisions != 1 || rejected != 1 || calls != 2 {
			t.Fatalf("decisions=%d parameterRejects=%d actualReads=%d", decisions, rejected, calls)
		}
		if networkActive != 1 || networkRevoked != 1 || len(view.State.EffectiveAuthority.NetworkProfiles) != 0 {
			t.Fatalf("expected one simulated offline activation/revocation: active=%d revoked=%d", networkActive, networkRevoked)
		}
	}
	if views[0].State.Principal != views[1].State.Principal || views[0].State.PolicyRef != views[1].State.PolicyRef {
		t.Fatal("caller/policy changed")
	}
	if log.calls() != 4 {
		t.Fatalf("MCP tools/call=%d, want 4", log.calls())
	}
	for i, task := range executor.tasks {
		before := log.calls()
		op := workerprotocol.Operation{ClaimID: task.ClaimID, WorkloadToken: task.ToolToken, Kind: "tool", Tool: "repo.read", ResourceScope: task.Tools[0].ResourceScope, Input: task.Tools[0].AllowedValues[0]}
		if reply, err := executor.handlers[i](context.Background(), op); err == nil && reply.Allowed {
			t.Fatal("terminal replay allowed")
		}
		if log.calls() != before {
			t.Fatal("terminal replay reached MCP")
		}
	}
	before := allocations.count
	request, err := v0.ParseClaimRequestYAML(incidentRead(t, "work-denied.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(request)
	denied := incidentHTTP(t, api.URL, "POST", "/api/requests", data)
	if denied.State.Decision.Result != v0.DecisionResultDeny || denied.State.Claim != nil || allocations.count != before || log.calls() != 4 {
		t.Fatal("admission denial allocated/dispatched")
	}
	t.Log("PASS: same principal/template/policy; disjoint authority; 4 real SDK reads; Running cross-scope/path probes and terminal replay add zero MCP calls; denied admission allocates nothing. Runtime=memory reference, Worker=real local process, Model=scripted NOT inference.")
}

type incidentTestCredential struct{ value string }

func (r incidentTestCredential) ReadSecretKey(_ context.Context, name, key string) ([]byte, error) {
	if name != "incident-fixture" || key != "token" {
		return nil, errors.New("unexpected synthetic reference")
	}
	return []byte(r.value), nil
}
func TestIncidentDemoCredentials(t *testing.T) {
	log, _, endpoint := incidentServer(t)
	for _, tc := range []struct {
		name, value string
		want        error
		delta       int
	}{
		{"valid", "synthetic-demo-token-not-a-real-secret", nil, 1},
		{"missing", "", toolbackend.ErrCredentialUnavailable, 0},
		{"wrong", "synthetic-wrong-demo-token", toolbackend.ErrCredentialRejected, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, v := v0.ParsePlatformYAML(incidentRead(t, "platform.yaml"))
			if v != nil {
				t.Fatal(v)
			}
			parsed.Spec.Services.ToolBackends[0].Config["endpoint"] = strings.TrimSuffix(endpoint, "/mcp") + "/mcp-token"
			parsed.Spec.Services.ToolBackends[0].Config["provisional-token-secret"] = "incident-fixture/token"
			registry, err := bundled.NewRegistry()
			if err != nil {
				t.Fatal(err)
			}
			resolved, _, failure := platform.Resolve(parsed, registry)
			if failure != nil {
				t.Fatal(failure)
			}
			var reader bundled.ProvisionalSecretReader
			if tc.value != "" {
				reader = incidentTestCredential{tc.value}
			}
			tools, err := buildInstalledTools(resolved, registry, reader)
			if err != nil {
				t.Fatal(err)
			}
			beforeCalls, beforeEntries := log.calls(), len(log.entries())
			result, err := tools.Invoke(context.Background(), toolbackend.Invocation{ID: "incident-credential-" + tc.name, ClaimID: "synthetic-credential-probe", Operation: "repo.read", ResourceScope: "repo:demo/payments", Parameters: map[string]string{"file": "payments/INC-P1042/logs.txt"}})
			if !errors.Is(err, tc.want) {
				t.Fatalf("unexpected credential error category: %v", err)
			}
			if tc.want == nil && !strings.Contains(result.Text, "SYNTHETIC DEMO DATA") {
				t.Fatal("valid credential did not read file")
			}
			if log.calls()-beforeCalls != tc.delta {
				t.Fatal("unexpected downstream tools/call delta")
			}
			if tc.name == "missing" && len(log.entries()) != beforeEntries {
				t.Fatal("missing credential made network requests")
			}
			encoded, _ := json.Marshal(log.entries())
			if bytes.Contains(encoded, []byte("synthetic-demo-token-not-a-real-secret")) || strings.Contains(result.Text, "synthetic-demo-token-not-a-real-secret") {
				t.Fatal("synthetic credential escaped trusted side")
			}
		})
	}
	t.Log("PASS: valid synthetic server-side token reads; missing sends no HTTP; wrong fails before tools/call; token absent from receipts/results. Not a production Secret-store or Pod inspection test.")
}
