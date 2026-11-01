// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/connectedclient"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type discoveryProvider struct {
	tool, scope string
	denied      bool
	calls       atomic.Int32
}

func (p *discoveryProvider) Complete(_ context.Context, _ modelprovider.Request) (modelprovider.Result, error) {
	n := p.calls.Add(1)
	a := workerprotocol.Action{Action: "tool", Tool: p.tool, Resource: p.scope, Input: "sample"}
	if n > 1 {
		a = workerprotocol.Action{Action: "finish", Answer: "Observed result", Completion: "completed"}
		if p.denied {
			a.Completion = "incomplete"
			a.Reason = "tool-denied"
			a.Target = p.tool
		}
	}
	b, _ := json.Marshal(a)
	return modelprovider.Result{Text: string(b), Model: "deterministic-test", InputTokens: 1, OutputTokens: 1}, nil
}

type discoveryExecutor struct {
	seen        *workerprotocol.Task
	tool, scope string
	budgetProbe bool
}

func (e discoveryExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, h workerprotocol.Handler) (string, error) {
	*e.seen = task
	model := workerprotocol.Operation{Kind: "model", ClaimID: task.ClaimID, WorkloadToken: task.ModelToken, Profile: task.ModelProfile, AgentProtocol: workerprotocol.DiscoveryProtocol, Prompt: workerprotocol.LoopPrompt(task, "")}
	if e.budgetProbe {
		for i := 0; i < 25; i++ {
			if _, err := h(ctx, model); err != nil {
				return "", err
			}
		}
		return "", errors.New("budget bypassed")
	}
	if _, err := h(ctx, model); err != nil {
		return "", err
	}
	if _, err := h(ctx, workerprotocol.Operation{Kind: "tool", ClaimID: task.ClaimID, WorkloadToken: task.ToolToken, Tool: e.tool, ResourceScope: e.scope, Input: "sample"}); err != nil {
		return "", err
	}
	reply, err := h(ctx, model)
	return reply.Text, err
}

func discoveryService(t *testing.T, e discoveryExecutor, p *discoveryProvider) (*Service, *toolDouble) {
	t.Helper()
	template := app.ReferenceTemplate()
	template.Spec.Entrypoint.Protocol = workerprotocol.DiscoveryProtocol
	twenty := 20
	template.Spec.CapabilityCeiling.MaxModelCalls = &twenty
	template.Spec.CapabilityCeiling.Tools = []string{"web.read", "crm.read"}
	template.Spec.CapabilityCeiling.ResourceScopes = []string{"web:public-pages", "crm:customer-retention"}
	loader := &policy.Loader{}
	if err := loader.Load(app.ReferencePolicy()); err != nil {
		t.Fatal(err)
	}
	backend := &toolDouble{}
	bindings := []toolbackend.Binding{}
	for _, pair := range [][2]string{{"web.read", "web:public-pages"}, {"crm.read", "crm:customer-retention"}, {"web.read", "web:outside"}} {
		bindings = append(bindings, toolbackend.Binding{Descriptor: toolbackend.Descriptor{Operation: pair[0], ResourceScope: pair[1], Description: "Test descriptor", Parameter: "input", MaxBytes: 64, AllowedValues: []string{"sample"}}, Backend: "test", Provider: backend, MaxConcurrentCalls: 1, MaxObservationBytes: 128})
	}
	tools, err := toolbackend.NewSet(bindings)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := app.NewReferencePrincipalSource(app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewServiceWithOptions(&verticalBackend{}, e, p, app.ReferencePrincipalTeamA, Options{ToolBackend: tools, Prepare: func(data []byte) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, principal, loader, advisorTemplate{template})
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, backend
}
func discoveryRequest(t *testing.T, name string, tools, scopes []string, limit int) []byte {
	t.Helper()
	var r v0.ClaimRequest
	_ = json.Unmarshal(verticalRequest(t, name), &r)
	r.Spec.RequestedAccess.Tools = tools
	r.Spec.RequestedAccess.ResourceScopes = scopes
	r.Spec.Runtime.MaxModelCalls = &limit
	b, _ := json.Marshal(r)
	return b
}

func TestTemplateDiscoveryDoesNotGrantWorkExecution(t *testing.T) {
	for _, operation := range []string{"web.read", "crm.read"} {
		for _, missing := range []string{"none", "tool", "scope", "all"} {
			t.Run(operation+"/"+missing, func(t *testing.T) {
				scope := "web:public-pages"
				otherScope := "crm:customer-retention"
				otherTool := "crm.read"
				if operation == "crm.read" {
					scope, otherScope = otherScope, scope
					otherTool = "web.read"
				}
				grantedTools := []string{operation}
				grantedScopes := []string{scope}
				if missing == "tool" {
					grantedTools = []string{otherTool}
				}
				if missing == "scope" {
					grantedScopes = []string{otherScope}
				}
				if missing == "all" {
					grantedTools = nil
					grantedScopes = nil
				}
				var task workerprotocol.Task
				p := &discoveryProvider{tool: operation, scope: scope, denied: missing != "none"}
				s, backend := discoveryService(t, discoveryExecutor{seen: &task, tool: operation, scope: scope}, p)
				ref := "discovery"
				if _, err := s.Submit(discoveryRequest(t, ref, grantedTools, grantedScopes, 6)); err != nil {
					t.Fatal(err)
				}
				v := awaitVertical(t, s, ref)
				if !task.Discovery || len(task.Tools) != 2 || task.MaxModelCalls != 6 {
					t.Fatalf("discovery changed with grants: %+v", task)
				}
				decisions, attempts := 0, 0
				for _, f := range v.Facts {
					if f.Kind == "ToolDecision" {
						decisions++
						want := v0.DecisionResultAllow
						if missing != "none" {
							want = v0.DecisionResultDeny
						}
						if f.Result != want || f.ActionRef == "" || f.Turn != 1 {
							t.Fatalf("missing actual decision/action correlation: %+v", f)
						}
					}
					if f.Kind == "ProviderAttempt" && f.Operation == "tool.invoke" {
						attempts++
					}
				}
				wantCalls := int32(1)
				if missing != "none" {
					wantCalls = 0
				}
				if decisions != 1 || backend.calls.Load() != wantCalls || attempts != int(wantCalls) || v.Outcome.Status != "Succeeded" {
					t.Fatalf("bad enforcement/outcome: %d %d %+v", decisions, backend.calls.Load(), v.Outcome)
				}
				if missing != "none" && (v.Outcome.Completion == nil || v.Outcome.Completion.Status != "incomplete") {
					t.Fatal("tool refusal should remain recoverable and truthful")
				}
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, v) }))
				defer server.Close()
				client := connectedclient.Client{Server: server.URL, HTTPClient: server.Client(), AccessToken: func(context.Context) (string, error) { return "test", nil }}
				if _, err := client.Show(ref); err != nil {
					t.Fatal("CLI rejected public action evidence", err)
				}
			})
		}
	}
}

func TestModelBudgetStopsIgnoringAgentWithoutExtraInference(t *testing.T) {
	for _, limit := range []int{2, 8, 20} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			var task workerprotocol.Task
			p := &discoveryProvider{tool: "crm.read", scope: "crm:customer-retention"}
			s, _ := discoveryService(t, discoveryExecutor{seen: &task, budgetProbe: true}, p)
			if _, err := s.Submit(discoveryRequest(t, "budget", nil, nil, limit)); err != nil {
				t.Fatal(err)
			}
			v := awaitVertical(t, s, "budget")
			if p.calls.Load() != int32(limit) || v.Outcome.Completion == nil || v.Outcome.Completion.ReportedBy != "platform" || v.Outcome.Completion.Reason != "model-call-limit" {
				t.Fatalf("budget not truthfully enforced: %d %+v", p.calls.Load(), v.Outcome)
			}
			reserved, denied := 0, 0
			for _, f := range v.Facts {
				if f.Kind == "ModelBudgetReservation" {
					reserved++
				}
				if f.Kind == "ModelDecision" && f.ReasonCode == "model-call-limit" {
					denied++
				}
				if f.Operation == "FinalAnswer" {
					t.Fatal("invented final Agent answer")
				}
			}
			if reserved != limit || denied != 1 {
				t.Fatal("missing authoritative accounting", reserved, denied)
			}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, v) }))
			defer server.Close()
			client := connectedclient.Client{Server: server.URL, HTTPClient: server.Client(), AccessToken: func(context.Context) (string, error) { return "test", nil }}
			if _, err := client.Show("budget"); err != nil {
				t.Fatal("CLI rejected exhausted budget evidence", err)
			}
			for i, f := range v.Facts {
				if f.Kind == "ModelBudgetReservation" {
					v.Facts = append(v.Facts[:i], v.Facts[i+1:]...)
					break
				}
			}
			if _, err := client.Show("budget"); err == nil {
				t.Fatal("CLI accepted model dispatch without its required reservation")
			}
		})
	}
}

func TestDiscoveryMaterializesOnlyInstalledTemplateScopesAndLegacyStaysFiltered(t *testing.T) {
	backend := &toolDouble{}
	bindings := []toolbackend.Binding{}
	for _, scope := range []string{"repo:acme/payments", "repo:acme/billing", "repo:other/private"} {
		bindings = append(bindings, toolbackend.Binding{Descriptor: toolbackend.Descriptor{Operation: "git.read", ResourceScope: scope, Description: "Test read", Parameter: "file", AllowedValues: []string{"README.md"}, MaxBytes: 64}, Provider: backend, Backend: "test", MaxObservationBytes: 64, MaxConcurrentCalls: 1})
	}
	tools, err := toolbackend.NewSet(bindings)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{tools: tools}
	template := app.ReferenceTemplate()
	template.Spec.Entrypoint.Protocol = workerprotocol.DiscoveryProtocol
	authority := &v0.EffectiveAuthority{Tools: []string{"git.read"}, ResourceScopes: []string{"repo:acme/payments"}}
	catalog, discovery := s.executionCatalog(template, authority)
	if !discovery || len(catalog) != 2 {
		t.Fatalf("wildcard discovery mismatch: %+v", catalog)
	}
	for _, tool := range catalog {
		if tool.ResourceScope == "repo:other/private" {
			t.Fatal("unrelated descriptor exposed")
		}
	}
	template.Spec.Entrypoint.Protocol = ""
	catalog, discovery = s.executionCatalog(template, authority)
	if discovery || len(catalog) != 1 || catalog[0].ResourceScope != "repo:acme/payments" {
		t.Fatal("legacy discovery widened")
	}
	if len(authority.ResourceScopes) != 1 {
		t.Fatal("discovery mutated Work grant")
	}
}
