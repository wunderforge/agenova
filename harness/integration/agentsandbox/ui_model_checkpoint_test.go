// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration && controlled && live

package agentsandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/modelprovider"
	runtimeagentsandbox "github.com/wunderforge/agenova/internal/runtime/agentsandbox"
)

var liveModel = flag.Bool("live-model", false, "explicit authorization for existing local Ollama inference on synthetic input")

func TestUIModelCheckpoint_Kind(t *testing.T) {
	if !*liveModel || *kubeContext == "" || *namespace == "" || *namespace == "default" {
		t.Fatal("explicit -live-model, kube-context and disposable namespace required; no model/cluster calls made")
	}
	if out, err := kubectlForContext("get", "namespace", *namespace, "--ignore-not-found=true", "-o", "name"); err != nil || len(strings.TrimSpace(string(out))) != 0 {
		t.Fatalf("refuse existing/unverified namespace %q: %v %s", *namespace, err, out)
	}
	if out, err := kubectlForContext("create", "namespace", *namespace); err != nil {
		t.Fatalf("create owned namespace: %v %s", err, out)
	}
	cleanupConfirmed := false
	t.Cleanup(func() {
		if !cleanupConfirmed {
			t.Logf("retain owned namespace %q for diagnosis; cleanup not confirmed", *namespace)
			return
		}
		if out, err := kubectlForContext("delete", "namespace", *namespace, "--wait=true"); err != nil {
			t.Errorf("delete owned namespace: %v %s", err, out)
		}
	})
	adapter := runtimeagentsandbox.NewControlled(*kubeContext, *namespace)
	if err := adapter.AddTemplate(v0.AgentSandboxTemplate{Metadata: v0.ObjectMeta{Name: app.ReferenceRuntimeTemplateRef}, Spec: v0.AgentSandboxTemplateSpec{Image: "agenova-testworker:kind", Command: []string{"/agenova-workerctl", "serve"}}}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.AddWarmPool(v0.SandboxWarmPool{Metadata: v0.ObjectMeta{Name: "reference-engineer-pool"}, Spec: v0.SandboxWarmPoolSpec{TemplateRef: app.ReferenceRuntimeTemplateRef, Replicas: 1}}); err != nil {
		t.Fatal(err)
	}
	provider, err := modelprovider.New(modelprovider.Config{Endpoint: "http://127.0.0.1:11434/v1", Models: map[string]string{"approved-coding-model": "llama3.1:latest"}, MaxTokens: 96, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	counter := &checkpointProvider{Client: provider}
	recorder := &recordingRuntimeBackend{RuntimeBackend: adapter}
	denied, err := console.NewService(recorder, adapter, counter, app.ReferencePrincipalTeamB)
	if err != nil {
		t.Fatal(err)
	}
	deniedServer := httptest.NewServer(console.Handler(denied))
	defer deniedServer.Close()
	defer denied.Close()
	deniedView := postCheckpoint(t, deniedServer.URL, checkpointRequest(t, "checkpoint-denied"), 200)
	if deniedView.State == nil || deniedView.State.Decision.Result != v0.DecisionResultDeny || deniedView.State.Claim != nil || counter.calls.Load() != 0 || recorder.callCount() != 0 {
		t.Fatal("preclaim denial reached allocation/provider")
	}
	for _, f := range deniedView.Facts {
		if f.ClaimID != "" {
			t.Fatal("denial fabricated claim identity")
		}
	}
	t.Log("Team B HTTP denial: claim=absent backendCalls=0 providerCalls=0")
	allowed, err := console.NewService(recorder, adapter, counter, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(console.Handler(allowed))
	defer server.Close()
	defer allowed.Close()
	initial := postCheckpoint(t, server.URL, checkpointRequest(t, "checkpoint-model"), 202)
	if initial.State == nil || initial.State.Claim == nil {
		t.Fatal("submission did not issue a canonical claim")
	}
	deadline := time.Now().Add(3 * time.Minute)
	var final evidence.View
	for {
		response, err := http.Get(server.URL + "/api/requests/checkpoint-model/evidence")
		if err != nil {
			t.Fatal(err)
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&final)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if final.Outcome != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("checkpoint deadline; no completed outcome")
		}
		time.Sleep(250 * time.Millisecond)
	}
	if final.State.Claim.Phase != v0.ClaimPhaseSucceeded || final.Outcome.Failure != "" || strings.TrimSpace(final.Outcome.Text) == "" || final.Outcome.Model == nil || counter.calls.Load() != 1 {
		t.Fatalf("real model checkpoint failed: state=%+v outcome=%+v providerCalls=%d", final.State.Claim, final.Outcome, counter.calls.Load())
	}
	if final.Outcome.Model.Model != "llama3.1:latest" || final.Outcome.Model.InputTokens <= 0 || final.Outcome.Model.OutputTokens <= 0 {
		t.Fatalf("missing real provider metadata: %+v", final.Outcome.Model)
	}
	grant := final.State.EffectiveAuthority
	if len(grant.Tools) != 1 || grant.Tools[0] != "git.read" || time.Duration(grant.Runtime.Timeout) != 30*time.Minute {
		t.Fatalf("not narrowed: %+v", grant)
	}
	var decision, attempt, outcome, cleanup bool
	inv := final.Outcome.Model.InvocationID
	for _, f := range final.Facts {
		if f.ClaimID != "" && f.ClaimID != final.State.Claim.ID {
			t.Fatal("cross-claim fact")
		}
		switch f.Kind {
		case "ModelDecision":
			decision = f.InvocationID == inv && f.Result == v0.DecisionResultAllow
		case "ProviderAttempt":
			attempt = f.InvocationID == inv
		case "ProviderOutcome":
			outcome = f.InvocationID == inv && f.ProviderStatus == "Succeeded"
		case "Runtime":
			if f.Operation == "CleanupSucceeded" {
				cleanup = true
			}
		}
	}
	if !decision || !attempt || !outcome || !cleanup {
		t.Fatal("missing same-invocation lifecycle/evidence")
	}
	id := final.State.Claim.BackendIdentity
	for _, resource := range []string{"pod/" + id.WorkerID, "sandbox/" + id.WorkerID} {
		assertAbsentEventuallyInNamespace(t, *namespace, resource)
	}
	if claims := namesFor(t, "sandboxclaims"); len(claims) != 0 {
		t.Fatalf("claims remain: %v", claims)
	}
	cleanupConfirmed = true
	t.Logf("UI API -> RunService -> kind worker -> Model Gateway -> Ollama -> worker result -> cleanup: request=%s claim=%s worker=%s invocation=%s", final.RequestRef, final.State.Claim.ID, id.WorkerID, inv)
	t.Logf("Actual response: %s", final.Outcome.Text)
	t.Logf("Provider metadata: model=%s response=%s inputTokens=%d outputTokens=%d; facts=%d; cleanup confirmed", final.Outcome.Model.Model, final.Outcome.Model.ResponseID, final.Outcome.Model.InputTokens, final.Outcome.Model.OutputTokens, len(final.Facts))
}

type checkpointProvider struct {
	modelprovider.Client
	calls atomic.Int32
}

func (p *checkpointProvider) Complete(ctx context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	p.calls.Add(1)
	return p.Client.Complete(ctx, r)
}
func checkpointRequest(t *testing.T, name string) []byte {
	t.Helper()
	timeout := v0.Duration(45 * time.Minute)
	r := v0.ClaimRequest{APIVersion: v0.ClaimRequestAPIVersion, Kind: v0.ClaimRequestKind, Metadata: v0.ObjectMeta{Name: name}, Spec: v0.ClaimRequestSpec{TemplateRef: "engineer", ProjectRef: "payments", Task: &v0.ClaimRequestTask{Type: "repository-change", Input: map[string]any{"objective": "Explain in one short sentence how bounded retries prevent runaway resource usage.", "repository": "acme/payments"}}, RequestedAccess: v0.ClaimRequestedAccess{Tools: []string{"git.read", "shell.exec"}, ResourceScopes: []string{"repo:acme/payments"}, ModelProfile: "approved-coding-model"}, Runtime: &v0.ClaimRuntimeRequirements{ProfileRef: "standard-isolated", Timeout: &timeout}}}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func postCheckpoint(t *testing.T, base string, data []byte, status int) evidence.View {
	t.Helper()
	response, err := http.Post(base+"/api/requests", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		t.Fatalf("HTTP submission=%d", response.StatusCode)
	}
	var view evidence.View
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&view); err != nil {
		t.Fatal(err)
	}
	return view
}
