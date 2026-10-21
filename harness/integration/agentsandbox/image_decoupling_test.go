// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build integration && controlled

package agentsandbox

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/registration"
	runtimebackend "github.com/wunderforge/agenova/internal/runtime/agentsandbox"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

var secondImage = flag.String("second-worker-image", "", "second independently built compatible image; opt-in image-decoupling smoke")
var incompatibleImage = flag.String("incompatible-worker-image", "", "image lacking controlled-v1; required with second-worker-image")

type imageRegistry struct{ *registration.MemoryStore }

func (r imageRegistry) Lookup(name string) (*v0.AgentTemplate, error) { return r.Template(name) }

// Explicit synthetic model response isolates runtime/image behavior, not LLM quality.
type imageTestProvider struct{ calls atomic.Int32 }

func (p *imageTestProvider) Complete(context.Context, modelprovider.Request) (modelprovider.Result, error) {
	n := p.calls.Add(1)
	return modelprovider.Result{Text: `{"action":"finish","tool":"","resource":"","input":"","answer":"image contract verified"}`, Model: "synthetic-image-test", ResponseID: fmt.Sprint(n), InputTokens: 1, OutputTokens: 1}, nil
}

type imageObservation struct {
	Image   string
	Command []string
}
type imageRecordingExecutor struct {
	console.Executor
	mu      sync.Mutex
	workers map[string]imageObservation
}

func (e *imageRecordingExecutor) Execute(ctx context.Context, id v0.SandboxClaimBackendIdentity, task workerprotocol.Task, handle workerprotocol.Handler) (string, error) {
	data, err := kubectl("get", "pod/"+id.WorkerID, "-o", "json")
	if err != nil {
		return "", fmt.Errorf("read test-owned worker image: %w", err)
	}
	var pod struct {
		Spec struct {
			Containers []struct {
				Name, Image string
				Command     []string
			}
		}
	}
	if err := json.Unmarshal(data, &pod); err != nil {
		return "", err
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == "agent" {
			e.mu.Lock()
			e.workers[task.ClaimID] = imageObservation{c.Image, c.Command}
			e.mu.Unlock()
		}
	}
	return e.Executor.Execute(ctx, id, task, handle)
}

// One unchanged application service, one runtime instance, independent templates.
// This is local backend evidence, not production OIDC or real-provider evidence.
func TestIndependentAgentImages_Kind(t *testing.T) {
	if *secondImage == "" {
		t.Skip("opt in with -second-worker-image and -incompatible-worker-image")
	}
	networkMode := requireGovernedNetworkPrerequisite(t)
	if *kubeContext == "" || *namespace == "" || *namespace == "default" || *incompatibleImage == "" {
		t.Fatal("explicit context, disposable namespace and both image flags required")
	}
	if out, err := kubectlForContext("create", "namespace", *namespace); err != nil {
		t.Fatalf("refuse reused namespace: %v %s", err, out)
	}
	clean := false
	t.Cleanup(func() {
		if clean {
			if out, err := kubectlForContext("delete", "namespace", *namespace, "--wait=false"); err != nil {
				t.Errorf("owned namespace cleanup: %v %s", err, out)
				return
			}
			deadline := time.Now().Add(90 * time.Second)
			for time.Now().Before(deadline) {
				out, err := kubectlForContext("get", "namespace", *namespace, "--ignore-not-found=true", "-o", "name")
				if err != nil {
					t.Errorf("namespace release unknown: %v", err)
					return
				}
				if strings.TrimSpace(string(out)) == "" {
					return
				}
				time.Sleep(time.Second)
			}
			t.Error("owned namespace release not confirmed")
		}
	})
	a := runtimebackend.NewControlled(*kubeContext, *namespace)
	if err := a.ConfigureNetworkIsolation(networkMode); err != nil {
		t.Fatal(err)
	}
	registries := map[string]imageRegistry{}
	templates := map[string]*v0.AgentTemplate{}
	bundle := app.ReferencePolicy()
	bundle.Rules = nil
	images := []string{*controlledWorkerImage, *secondImage, *incompatibleImage}
	for i, image := range images {
		project := fmt.Sprintf("project-%d", i)
		registry := imageRegistry{registration.NewMemoryStore()}
		registries[project] = registry
		registrar := registration.Service{Store: registry, ValidateTemplate: app.ValidateControlledTemplate}
		template := app.ReferenceTemplate()
		template.Metadata.Name, template.Spec.Artifact.Image = "engineer", image
		template.Spec.Entrypoint.Command = []string{"/agenova-workerctl", "serve"}
		if i == 1 {
			template.Spec.Entrypoint.Command = []string{"/bin/sh", "-c", "exec /agenova-workerctl serve"}
		}
		if i == 2 {
			template.Spec.Entrypoint.Command = []string{"/bin/sh", "-c", "sleep 600"}
		}
		limit := v0.Duration(time.Minute)
		template.Spec.CapabilityCeiling.MaxTimeout = &limit
		template.Spec.Defaults.MemoryScopes = nil
		document, err := json.Marshal(template)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := registrar.ApplyTemplate(document); err != nil {
			t.Fatalf("independent registration %d: %v", i, err)
		}
		stored, err := registry.Template("engineer")
		if err != nil {
			t.Fatal(err)
		}
		templates[project] = stored
		// Prewarm every variant before the first Work. A name-only pool could
		// otherwise hand project 0 a ready Pod from project 1 or 2.
		if _, err := a.ConfigureAgentTemplate(stored); err != nil {
			t.Fatal(err)
		}
		// Immutable identity still cannot be repointed at another image.
		template.Spec.Artifact.Image = "different-image:must-conflict"
		changed, _ := json.Marshal(template)
		if _, err := registrar.ApplyTemplate(changed); !errors.Is(err, registration.ErrConflict) {
			t.Fatalf("immutable registration changed: %v", err)
		}
		bundle.Rules = append(bundle.Rules, policy.Rule{Team: "team-a", Action: "claim.create", Project: project, TemplateRef: "engineer"})
	}
	loader := &policy.Loader{}
	if err := loader.Load(bundle); err != nil {
		t.Fatal(err)
	}
	principal, err := app.NewReferencePrincipalSource(app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	provider := &imageTestProvider{}
	executor := &imageRecordingExecutor{Executor: a, workers: map[string]imageObservation{}}
	service, err := console.NewServiceWithOptions(a, executor, provider, app.ReferencePrincipalTeamA, console.Options{
		Prepare: func(data []byte) (app.PreparedAssignment, error) {
			request, err := v0.ParseClaimRequestYAML(data)
			if err != nil {
				return app.PreparedAssignment{}, err
			}
			registry, ok := registries[request.Spec.ProjectRef]
			if !ok {
				return app.PreparedAssignment{}, errors.New("unknown test project")
			}
			return app.PrepareAssignment(data, principal, loader, registry)
		},
		Configure: func(template *v0.AgentTemplate) (app.ResolvedLaunch, error) {
			if err := app.ValidateControlledTemplate(template); err != nil {
				return app.ResolvedLaunch{}, err
			}
			ref, err := a.ConfigureAgentTemplate(template)
			return app.ResolvedLaunch{TemplateRef: ref}, err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	workers := map[string]bool{}
	// Return to project 0 after project 1 to exercise an already prewarmed pool.
	for i, variant := range []int{0, 1, 0, 2} {
		name := fmt.Sprintf("image-contract-%d", i)
		project := fmt.Sprintf("project-%d", variant)
		image := images[variant]
		request := fmt.Sprintf(`{"apiVersion":"agenova.io/v1alpha1","kind":"ClaimRequest","metadata":{"name":"%s"},"spec":{"templateRef":"engineer","projectRef":"%s","task":{"type":"investigation","input":{"objective":"verify independent image execution"}},"requestedAccess":{"modelProfile":"approved-coding-model"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}}`, name, project)
		before := provider.calls.Load()
		if _, err := service.Submit([]byte(request)); err != nil {
			t.Fatalf("submit: %v", err)
		}
		var final evidence.View
		deadline := time.Now().Add(100 * time.Second)
		for time.Now().Before(deadline) {
			final, err = service.QueryRequest(name)
			if err != nil {
				t.Fatal(err)
			}
			if final.Outcome != nil {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if final.Outcome == nil || final.State == nil || final.State.Claim == nil || final.State.Claim.BackendIdentity == nil {
			t.Fatalf("incomplete image outcome: %+v", final.Outcome)
		}
		worker := final.State.Claim.BackendIdentity.WorkerID
		if workers[worker] {
			t.Fatal("independent claims shared one worker identity")
		}
		workers[worker] = true
		cleanup := false
		for _, fact := range final.Facts {
			if fact.Kind == "Runtime" && fact.Operation == "CleanupSucceeded" {
				cleanup = true
			}
		}
		if !cleanup {
			t.Fatal("resource release not confirmed; retain owned namespace")
		}
		if variant < 2 {
			if final.Outcome.Status != "Succeeded" || final.Outcome.Text != "image contract verified" || provider.calls.Load() <= before {
				t.Fatalf("compatible image failed: %+v", final.Outcome)
			}
			executor.mu.Lock()
			actual, observed := executor.workers[final.State.Claim.ID]
			executor.mu.Unlock()
			if !observed || actual.Image != image || !reflect.DeepEqual(actual.Command, templates[project].Spec.Entrypoint.Command) {
				t.Fatalf("same-name project template used wrong worker: project=%s image=%s command=%v", project, actual.Image, actual.Command)
			}
		} else if final.Outcome.Status != "Failed" || provider.calls.Load() != before || final.Diagnosis == nil || final.Diagnosis.Code != "runtime-worker-control-unavailable" {
			t.Fatalf("incompatible image did not fail before provider: %+v diagnosis=%+v", final.Outcome, final.Diagnosis)
		}
		t.Logf("project=%s template=engineer image=%s entrypoint-isolated=true status=%s providerCalls=%d cleanup=confirmed worker=%s", project, image, final.Outcome.Status, provider.calls.Load()-before, worker)
	}
	clean = true
}
