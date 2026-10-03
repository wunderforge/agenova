// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build agenovaprobe

package main

// E16 Slice 3 acceptance probes (N1-N5, N11). They run the installed tool
// composition (buildInstalledTools, console.Service, Tool Gateway and the
// real MCP provider) and replace only the runtime (in-memory), the worker
// (a capturing executor) and the model (a finish-only double). Built only
// with the agenovaprobe tag; never part of the installed control plane.
//
// Environment:
//   AGENOVA_PROBE_PLATFORM  effective-platform.json from the installed revision
//   AGENOVA_PROBE_TEMPLATE  the registered E16 AgentTemplate YAML
//   AGENOVA_PROBE_POLICY    the registered E16 PolicyBundle YAML
//   AGENOVA_PROBE_PROVIDER  "double" for a local dry run only; never evidence
//                           (a source Platform .yaml is accepted only then)
//
// Each step prints one "E16_PROBE <json>" receipt to stdout. Server-side
// zero-call proof comes from the fixture's logs, matched by time window and
// invocation ID; the probe only states what it expected and observed.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

const (
	probeFixtureScope = "repo:agenova/e16-fixture"
	probeFaultsScope  = "repo:agenova/e16-faults"
	probeFile         = "README.md"
)

type probeReceipt struct {
	Case          string   `json:"case"`
	Step          string   `json:"step"`
	Provider      string   `json:"provider"`
	Requests      []string `json:"requests"`
	Claims        []string `json:"claims"`
	ClaimPhases   []string `json:"claimPhases,omitempty"`
	InvocationIDs []string `json:"invocationIds"`
	StartedAt     string   `json:"startedAt"`
	EndedAt       string   `json:"endedAt"`
	ExpectedPoint string   `json:"expectedPoint"`
	ExpectedCalls int      `json:"expectedToolsCalls"`
	ReplyAllowed  bool     `json:"replyAllowed"`
	ObservedError string   `json:"observedError,omitempty"`
	ToolFacts     []string `json:"toolFacts"`
	ReasonCodes   []string `json:"reasonCodes"`
	// InjectedFailures counts journal faults the probe actually triggered;
	// InjectedFact is the fact that was refused (kind:result:status).
	InjectedFailures int32  `json:"injectedFailures,omitempty"`
	InjectedFact     string `json:"injectedFact,omitempty"`
	// DoubleCalls is the local dry-run provider count; nil against MCP,
	// where the server logs are the only call evidence.
	DoubleCalls *int32 `json:"doubleCalls,omitempty"`
	Pass        bool   `json:"pass"`
}

type probeEnv struct {
	resolved *platform.ResolvedPlatform
	template *v0.AgentTemplate
	bundle   policy.PolicyBundle
	tools    *toolbackend.Set
	provider string
	double   *probeDouble
}

func loadProbeEnv(t *testing.T) probeEnv {
	t.Helper()
	path := os.Getenv("AGENOVA_PROBE_PLATFORM")
	if path == "" {
		t.Skip("AGENOVA_PROBE_PLATFORM is not set; E16 probes run only inside the acceptance campaign")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var resolved platform.ResolvedPlatform
	if strings.HasSuffix(path, ".yaml") {
		// Local dry run from the source Platform; never the installed revision.
		if os.Getenv("AGENOVA_PROBE_PROVIDER") != "double" {
			t.Fatal("a source Platform is accepted only with AGENOVA_PROBE_PROVIDER=double")
		}
		input, verr := v0.ParsePlatformYAML(data)
		if verr != nil {
			t.Fatal(verr)
		}
		registry, err := bundled.NewRegistry()
		if err != nil {
			t.Fatal(err)
		}
		local, _, failure := platform.Resolve(input, registry)
		if failure != nil {
			t.Fatal(failure)
		}
		resolved = *local
	} else if err := json.Unmarshal(data, &resolved); err != nil || resolved.Revision == "" {
		t.Fatalf("installed effective Platform is unreadable: %v", err)
	}
	templateData, err := os.ReadFile(os.Getenv("AGENOVA_PROBE_TEMPLATE"))
	if err != nil {
		t.Fatal(err)
	}
	template, verr := v0.ParseAgentTemplateYAML(templateData)
	if verr != nil {
		t.Fatal(verr)
	}
	policyData, err := os.ReadFile(os.Getenv("AGENOVA_PROBE_POLICY"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := policy.ParseDocumentYAML(policyData)
	if err != nil {
		t.Fatal(err)
	}
	env := probeEnv{resolved: &resolved, template: template, bundle: bundle, provider: "mcp"}
	registry, err := bundled.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	var tools toolRegistry = registry
	if os.Getenv("AGENOVA_PROBE_PROVIDER") == "double" {
		env.provider, env.double = "double", &probeDouble{}
		tools = doubleRegistry{Registry: registry, provider: env.double}
	}
	if env.tools, err = buildInstalledTools(&resolved, tools); err != nil || env.tools == nil {
		t.Fatalf("installed tools: %v", err)
	}
	return env
}

// probeDouble stands in for the MCP server in a local dry run only.
type probeDouble struct{ calls atomic.Int32 }

func (*probeDouble) MaxConcurrentCalls() int { return 4 }
func (d *probeDouble) Invoke(_ context.Context, call toolbackend.Invocation) (toolbackend.Result, error) {
	d.calls.Add(1)
	return toolbackend.Result{Text: "local dry-run double for " + call.Parameters["file"]}, nil
}

// probeModel completes every model turn with a finish action, so a claim can
// reach Succeeded without a model server.
type probeModel struct{}

func (probeModel) Complete(context.Context, modelprovider.Request) (modelprovider.Result, error) {
	return modelprovider.Result{Text: `{"action":"finish","tool":"","input":"","answer":"probe complete"}`, Model: "probe-double", ResponseID: "probe"}, nil
}

type probeTemplates struct{ template *v0.AgentTemplate }

func (p probeTemplates) Lookup(name string) (*v0.AgentTemplate, error) {
	if name != p.template.Metadata.Name {
		return nil, fmt.Errorf("unknown template %q", name)
	}
	return p.template, nil
}

// probeExecutor hands the real per-Work operation handler to the case.
type probeExecutor struct {
	run func(context.Context, workerprotocol.Task, workerprotocol.Handler) (string, error)
}

func (e probeExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	return e.run(ctx, task, h)
}

// newProbeService mirrors configuredService's admission checks with the
// in-memory runtime. wrap, when set, installs the journal fault bridge.
func newProbeService(t *testing.T, env probeEnv, run func(context.Context, workerprotocol.Task, workerprotocol.Handler) (string, error), wrap func(func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error)) *console.Service {
	t.Helper()
	backend, _, err := app.NewRuntime("")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := app.NewReferencePrincipalSource(app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	models, runtimeProfiles := map[string]string{}, map[string]bool{}
	for _, profile := range env.resolved.Profiles {
		switch profile.Capability {
		case platform.CapabilityModel:
			model, _ := profile.Config["model"].(string)
			models[profile.Name] = model
		case platform.CapabilityRuntime:
			runtimeProfiles[profile.Name] = true
		}
	}
	service, err := console.NewServiceWithOptions(backend, probeExecutor{run: run}, probeModel{}, app.ReferencePrincipalTeamA, console.Options{
		ToolBackend: env.tools,
		Prepare: func(data []byte) (app.PreparedAssignment, error) {
			loader := &policy.Loader{}
			if err := loader.Load(env.bundle); err != nil {
				return app.PreparedAssignment{}, err
			}
			prepared, err := app.PrepareAssignment(data, principal, loader, probeTemplates{template: env.template})
			if err != nil {
				return prepared, err
			}
			if prepared.Issued != nil && prepared.Issued.Claim != nil {
				if err := validateInstalledAuthority(prepared.Issued.EffectiveAuthority, models, runtimeProfiles, env.tools); err != nil {
					return app.PreparedAssignment{}, err
				}
			}
			return prepared, nil
		},
		Configure: func(*v0.AgentTemplate) (app.ResolvedLaunch, error) {
			return app.ResolvedLaunch{TemplateRef: app.ReferenceRuntimeTemplateRef}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	if wrap != nil {
		if err := console.WrapToolAppendForProbe(service, wrap); err != nil {
			t.Fatal(err)
		}
	}
	return service
}

func probeRequest(t *testing.T, env probeEnv, name string, scopes []string, timeout time.Duration) []byte {
	t.Helper()
	tools := []string{"repo.read"}
	if len(scopes) == 0 {
		tools, scopes = []string{}, []string{}
	}
	d := v0.Duration(timeout)
	model := ""
	for _, profile := range env.template.Spec.CapabilityCeiling.ModelProfiles {
		model = profile
	}
	runtime := ""
	for _, profile := range env.template.Spec.CapabilityCeiling.RuntimeProfiles {
		runtime = profile
	}
	request := v0.ClaimRequest{APIVersion: v0.ClaimRequestAPIVersion, Kind: v0.ClaimRequestKind, Metadata: v0.ObjectMeta{Name: name},
		Spec: v0.ClaimRequestSpec{TemplateRef: env.template.Metadata.Name, ProjectRef: "payments",
			Task:            &v0.ClaimRequestTask{Type: "investigation", Input: map[string]any{"objective": "E16 acceptance probe " + name}},
			RequestedAccess: v0.ClaimRequestedAccess{Tools: tools, ResourceScopes: scopes, ModelProfile: model},
			Runtime:         &v0.ClaimRuntimeRequirements{ProfileRef: runtime, Timeout: &d}}}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func toolOp(claimID, scope, file string) workerprotocol.Operation {
	return workerprotocol.Operation{ClaimID: claimID, Kind: "tool", Tool: "repo.read", ResourceScope: scope, Input: file}
}

// toolFacts lists tool.invoke facts recorded for ref after sequence `after`.
func toolFacts(service *console.Service, ref string, after uint64) (summary []string, invocations []string, last uint64) {
	summary, invocations, _, last = toolFactDetails(service, ref, after)
	return summary, invocations, last
}

// toolFactDetails also returns the reason codes of those facts, so a probe
// asserts why a call was rejected, not only that it was.
func toolFactDetails(service *console.Service, ref string, after uint64) (summary []string, invocations []string, reasons []string, last uint64) {
	view, err := service.QueryRequest(ref)
	if err != nil {
		return []string{"query-failed"}, nil, nil, after
	}
	seen := map[string]bool{}
	for _, f := range view.Facts {
		if f.Sequence > last {
			last = f.Sequence
		}
		if f.Sequence <= after || f.Operation != "tool.invoke" {
			continue
		}
		parts := []string{f.Kind}
		for _, part := range []string{string(f.Result), f.ProviderStatus} {
			if part != "" {
				parts = append(parts, part)
			}
		}
		summary = append(summary, strings.Join(parts, ":"))
		if f.ReasonCode != "" {
			reasons = append(reasons, f.ReasonCode)
		}
		if f.InvocationID != "" && !seen[f.InvocationID] {
			seen[f.InvocationID] = true
			invocations = append(invocations, f.InvocationID)
		}
	}
	return summary, invocations, reasons, last
}

func emit(t *testing.T, r probeReceipt) {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("E16_PROBE %s\n", data)
	if !r.Pass {
		t.Errorf("%s/%s did not reach %s: %s %v", r.Case, r.Step, r.ExpectedPoint, r.ObservedError, r.ToolFacts)
	}
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// step runs one operation through a live handler and returns its receipt.
func step(env probeEnv, service *console.Service, ref string, h workerprotocol.Handler, ctx context.Context, op workerprotocol.Operation) (probeReceipt, workerprotocol.Reply, error) {
	_, _, before := toolFacts(service, ref, 0)
	r := probeReceipt{Requests: []string{ref}, Claims: []string{op.ClaimID}, StartedAt: now()}
	var calls int32
	if env.double != nil {
		calls = env.double.calls.Load()
	}
	reply, err := h(ctx, op)
	r.EndedAt = now()
	if env.double != nil {
		delta := env.double.calls.Load() - calls
		r.DoubleCalls = &delta
	}
	r.ReplyAllowed, r.ObservedError = reply.Allowed, errText(err)
	r.ToolFacts, r.InvocationIDs, r.ReasonCodes, _ = toolFactDetails(service, ref, before)
	if r.ToolFacts == nil {
		r.ToolFacts = []string{}
	}
	if r.ReasonCodes == nil {
		r.ReasonCodes = []string{}
	}
	if r.InvocationIDs == nil {
		r.InvocationIDs = []string{}
	}
	return r, reply, err
}

func hasOnly(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// runInWork submits one Work, runs body inside its live executor and waits
// for the Work outcome.
func runInWork(t *testing.T, env probeEnv, name string, scopes []string, wrap func(func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error), body func(context.Context, workerprotocol.Task, workerprotocol.Handler, *console.Service)) {
	t.Helper()
	var service *console.Service
	ready := make(chan struct{})
	service = newProbeService(t, env, func(ctx context.Context, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
		<-ready
		body(ctx, task, h, service)
		return "", errors.New("probe finished")
	}, wrap)
	if _, err := service.Submit(probeRequest(t, env, name, scopes, 2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	close(ready)
	awaitProbe(t, service, name)
}

func awaitProbe(t *testing.T, service *console.Service, ref string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if view, err := service.QueryRequest(ref); err == nil && view.Outcome != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s produced no outcome", ref)
}

// control is the positive control around each zero-call window: one granted
// read that must reach the server exactly once.
func control(t *testing.T, env probeEnv, caseName, label string) {
	t.Helper()
	runInWork(t, env, strings.ToLower(caseName+"-"+label), []string{probeFixtureScope}, nil, func(ctx context.Context, task workerprotocol.Task, h workerprotocol.Handler, service *console.Service) {
		r, reply, err := step(env, service, strings.ToLower(caseName+"-"+label), h, ctx, toolOp(task.ClaimID, probeFixtureScope, probeFile))
		r.Case, r.Step, r.Provider, r.ExpectedPoint, r.ExpectedCalls = caseName, label, env.provider, "provider success", 1
		r.Pass = err == nil && reply.Allowed && hasOnly(r.ToolFacts, "ToolDecision:Allow", "ProviderAttempt:Attempted", "ProviderOutcome:Succeeded") && (r.DoubleCalls == nil || *r.DoubleCalls == 1)
		emit(t, r)
	})
}

func TestE16Probes(t *testing.T) {
	env := loadProbeEnv(t)

	injected := map[string]*atomic.Int32{}
	refused := map[string]*atomic.Value{}
	failKind := func(kind string) func(func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error) {
		counter, fact := &atomic.Int32{}, &atomic.Value{}
		injected[kind], refused[kind] = counter, fact
		return func(next func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error) {
			return func(f facts.Fact) (facts.Fact, error) {
				if f.Kind == kind && f.Operation == "tool.invoke" {
					counter.Add(1)
					fact.Store(strings.Join([]string{f.Kind, string(f.Result), f.ProviderStatus}, ":"))
					return facts.Fact{}, errors.New("probe injected " + kind + " journal failure")
				}
				return next(f)
			}
		}
	}
	fired := func(kind string) (int32, string) {
		if counter := injected[kind]; counter != nil {
			summary, _ := refused[kind].Load().(string)
			return counter.Load(), summary
		}
		return 0, ""
	}
	const evidenceFailure = "tool evidence recording failed"

	// zeroCall runs one rejection inside a Work, framed by positive controls.
	zeroCall := func(t *testing.T, caseName string, scopes []string, fault string, op func(task workerprotocol.Task) workerprotocol.Operation, point string, pass func(probeReceipt) bool) {
		control(t, env, caseName, "control-before")
		ref := strings.ToLower(caseName)
		var wrap func(func(facts.Fact) (facts.Fact, error)) func(facts.Fact) (facts.Fact, error)
		if fault != "" {
			wrap = failKind(fault)
		}
		runInWork(t, env, ref, scopes, wrap, func(ctx context.Context, task workerprotocol.Task, h workerprotocol.Handler, service *console.Service) {
			r, _, _ := step(env, service, ref, h, ctx, op(task))
			r.Case, r.Step, r.Provider, r.ExpectedPoint, r.ExpectedCalls = caseName, "probe", env.provider, point, 0
			if fault != "" {
				r.InjectedFailures, r.InjectedFact = fired(fault)
			}
			r.Pass = pass(r) && (r.DoubleCalls == nil || *r.DoubleCalls == 0)
			emit(t, r)
		})
		control(t, env, caseName, "control-after")
	}

	t.Run("N1", func(t *testing.T) {
		zeroCall(t, "N1", nil, "", func(task workerprotocol.Task) workerprotocol.Operation {
			return toolOp(task.ClaimID, probeFixtureScope, probeFile)
		}, "Tool Gateway authority (no tool grant)", func(r probeReceipt) bool {
			return r.ObservedError == "" && !r.ReplyAllowed && hasOnly(r.ToolFacts, "ToolDecision:Deny") && hasOnly(r.ReasonCodes, gateway.CategoryToolNotGranted)
		})
	})
	t.Run("N2a", func(t *testing.T) {
		zeroCall(t, "N2a", []string{probeFixtureScope}, "", func(task workerprotocol.Task) workerprotocol.Operation {
			return toolOp(task.ClaimID, probeFaultsScope, "notes/incident-timeline.md")
		}, "Tool Gateway authority (ungranted configured scope)", func(r probeReceipt) bool {
			return r.ObservedError == "" && !r.ReplyAllowed && hasOnly(r.ToolFacts, "ToolDecision:Deny") && hasOnly(r.ReasonCodes, gateway.CategoryResourceNotGranted)
		})
	})
	t.Run("N2b", func(t *testing.T) {
		zeroCall(t, "N2b", []string{probeFixtureScope}, "", func(task workerprotocol.Task) workerprotocol.Operation {
			return toolOp(task.ClaimID, probeFixtureScope, "logs/slow.log")
		}, "installed route allowlist before the Gateway", func(r probeReceipt) bool {
			return r.ObservedError == toolbackend.ErrArguments.Error() && len(r.ToolFacts) == 0
		})
	})
	t.Run("N5a", func(t *testing.T) {
		zeroCall(t, "N5a", []string{probeFixtureScope}, "ToolDecision", func(task workerprotocol.Task) workerprotocol.Operation {
			return toolOp(task.ClaimID, probeFixtureScope, probeFile)
		}, "ToolDecision journal append", func(r probeReceipt) bool {
			return r.InjectedFailures == 1 && r.InjectedFact == "ToolDecision:Allow:" && r.ObservedError == evidenceFailure && len(r.ToolFacts) == 0
		})
	})
	t.Run("N5b", func(t *testing.T) {
		zeroCall(t, "N5b", []string{probeFixtureScope}, "ProviderAttempt", func(task workerprotocol.Task) workerprotocol.Operation {
			return toolOp(task.ClaimID, probeFixtureScope, probeFile)
		}, "ProviderAttempt journal append", func(r probeReceipt) bool {
			return r.InjectedFailures == 1 && r.InjectedFact == "ProviderAttempt::Attempted" && r.ObservedError == evidenceFailure && hasOnly(r.ToolFacts, "ToolDecision:Allow")
		})
	})
	t.Run("N3", func(t *testing.T) { probeCrossClaim(t, env) })
	for _, terminal := range []v0.ClaimPhase{v0.ClaimPhaseSucceeded, v0.ClaimPhaseFailed, v0.ClaimPhaseExpired} {
		t.Run("N4-"+string(terminal), func(t *testing.T) { probeTerminal(t, env, terminal) })
	}
	// N11 makes one real call, so it runs outside the zero-call windows.
	t.Run("N11", func(t *testing.T) {
		runInWork(t, env, "n11", []string{probeFixtureScope}, failKind("ProviderOutcome"), func(ctx context.Context, task workerprotocol.Task, h workerprotocol.Handler, service *console.Service) {
			r, _, _ := step(env, service, "n11", h, ctx, toolOp(task.ClaimID, probeFixtureScope, probeFile))
			r.Case, r.Step, r.Provider, r.ExpectedPoint, r.ExpectedCalls = "N11", "probe", env.provider, "ProviderOutcome journal append after the call", 1
			// The refused outcome must be the provider's success: a failed call
			// whose record is lost would prove nothing about N11.
			r.InjectedFailures, r.InjectedFact = fired("ProviderOutcome")
			r.Pass = r.InjectedFailures == 1 && r.InjectedFact == "ProviderOutcome::Succeeded" && r.ObservedError == evidenceFailure && hasOnly(r.ToolFacts, "ToolDecision:Allow", "ProviderAttempt:Attempted") && (r.DoubleCalls == nil || *r.DoubleCalls == 1)
			emit(t, r)
		})
	})
}

// probeCrossClaim holds claims A and B Running in two independent service
// instances and injects an operation nominating B into A's handler. Each
// runner owns its own state, so this proves only A's handler correlation
// check, not shared-service authorisation or authenticated isolation.
func probeCrossClaim(t *testing.T, env probeEnv) {
	control(t, env, "N3", "control-before")
	type running struct {
		service *console.Service
		claimID string
		handler workerprotocol.Handler
		ctx     context.Context
	}
	start := func(ref string, release <-chan struct{}) (chan running, *console.Service) {
		entered := make(chan running, 1)
		var service *console.Service
		service = newProbeService(t, env, func(ctx context.Context, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
			entered <- running{service: service, claimID: task.ClaimID, handler: h, ctx: ctx}
			<-release
			return "", errors.New("probe finished")
		}, nil)
		if _, err := service.Submit(probeRequest(t, env, ref, []string{probeFixtureScope}, 2*time.Minute)); err != nil {
			t.Fatal(err)
		}
		return entered, service
	}
	release := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(release) }) }
	defer stop()
	enteredA, serviceA := start("n3-a", release)
	enteredB, serviceB := start("n3-b", release)
	a, b := <-enteredA, <-enteredB
	phases := func() []string {
		out := []string{}
		for _, item := range []struct {
			service *console.Service
			ref     string
		}{{serviceA, "n3-a"}, {serviceB, "n3-b"}} {
			view, err := item.service.QueryRequest(item.ref)
			if err != nil || view.State == nil || view.State.Claim == nil {
				out = append(out, "unknown")
				continue
			}
			out = append(out, view.State.Claim.ID+"="+string(view.State.Claim.Phase))
		}
		return out
	}
	snapshot := phases()
	r, _, _ := step(env, serviceA, "n3-a", a.handler, a.ctx, toolOp(b.claimID, probeFixtureScope, probeFile))
	_, bFacts, _ := toolFacts(serviceB, "n3-b", 0)
	r.Case, r.Step, r.Provider, r.ExpectedPoint, r.ExpectedCalls = "N3", "probe", env.provider, "claim-ID check in A's operation handler", 0
	r.Requests, r.Claims, r.ClaimPhases = []string{"n3-a", "n3-b"}, []string{a.claimID, b.claimID}, snapshot
	r.Pass = strings.Contains(r.ObservedError, "worker session binding") && len(r.ToolFacts) == 0 && len(bFacts) == 0 && (r.DoubleCalls == nil || *r.DoubleCalls == 0) &&
		strings.HasSuffix(snapshot[0], "=Running") && strings.HasSuffix(snapshot[1], "=Running")
	emit(t, r)
	// Matched-claim controls: each handler accepts its own claim.
	for _, item := range []struct {
		label   string
		ref     string
		running running
	}{{"control-matched-a", "n3-a", a}, {"control-matched-b", "n3-b", b}} {
		r, reply, err := step(env, item.running.service, item.ref, item.running.handler, item.running.ctx, toolOp(item.running.claimID, probeFixtureScope, probeFile))
		r.Case, r.Step, r.Provider, r.ExpectedPoint, r.ExpectedCalls = "N3", item.label, env.provider, "provider success", 1
		r.Pass = err == nil && reply.Allowed && hasOnly(r.ToolFacts, "ToolDecision:Allow", "ProviderAttempt:Attempted", "ProviderOutcome:Succeeded")
		emit(t, r)
	}
	stop()
	awaitProbe(t, serviceA, "n3-a")
	awaitProbe(t, serviceB, "n3-b")
	control(t, env, "N3", "control-after")
}

// probeTerminal captures a Work's handler, drives its claim to the terminal
// phase, then calls the handler with a fresh live context. The rejection must
// come from the lifecycle check, not from the cancelled work context.
func probeTerminal(t *testing.T, env probeEnv, terminal v0.ClaimPhase) {
	caseName := "N4-" + string(terminal)
	ref := strings.ToLower(caseName)
	control(t, env, caseName, "control-before")
	captured := make(chan workerprotocol.Handler, 1)
	var claimID string
	timeout := 2 * time.Minute
	if terminal == v0.ClaimPhaseExpired {
		timeout = 3 * time.Second
	}
	service := newProbeService(t, env, func(ctx context.Context, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
		claimID = task.ClaimID
		captured <- h
		switch terminal {
		case v0.ClaimPhaseSucceeded:
			reply, err := h(ctx, workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: workerprotocol.LoopPrompt(task, "")})
			return reply.Text, err
		case v0.ClaimPhaseFailed:
			return "", errors.New("probe worker failure")
		default:
			<-ctx.Done()
			return "", ctx.Err()
		}
	}, nil)
	if _, err := service.Submit(probeRequest(t, env, ref, []string{probeFixtureScope}, timeout)); err != nil {
		t.Fatal(err)
	}
	h := <-captured
	awaitProbe(t, service, ref)
	view, err := service.QueryRequest(ref)
	if err != nil || view.State == nil || view.State.Claim == nil {
		t.Fatalf("terminal state unavailable: %v", err)
	}
	phase := view.State.Claim.Phase
	r, _, _ := step(env, service, ref, h, context.Background(), toolOp(claimID, probeFixtureScope, probeFile))
	r.Case, r.Step, r.Provider, r.ExpectedPoint, r.ExpectedCalls = caseName, "probe", env.provider, "app.RequireRunningClaim", 0
	r.ClaimPhases = []string{claimID + "=" + string(phase)}
	r.Pass = phase == terminal && strings.Contains(r.ObservedError, fmt.Sprintf("is not running (phase: %s)", terminal)) && len(r.ToolFacts) == 0 && (r.DoubleCalls == nil || *r.DoubleCalls == 0)
	emit(t, r)
	control(t, env, caseName, "control-after")
}
