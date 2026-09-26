// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/adapters/bundled"
	"github.com/wunderforge/agenova/internal/console"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/toolbackend"
)

func resolvedTools(t *testing.T) (*platform.ResolvedPlatform, *adapterregistry.Registry) {
	t.Helper()
	registry, err := bundled.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := adapterregistry.NewLifecycle(registry, adapterregistry.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	input := &v0.Platform{APIVersion: v0.PlatformAPIVersion, Kind: v0.PlatformKind, Metadata: v0.ObjectMeta{Name: "tools"}, Spec: v0.PlatformSpec{InitialPolicyRef: &v0.PlatformPolicyReference{ID: "reference-default-deny", Version: "1"}}}
	for _, id := range []string{bundled.KubernetesDeploymentID, bundled.AgentSandboxRuntimeID, bundled.OpenAICompatibleModelID, bundled.MCPHTTPToolID} {
		fragment, err := lifecycle.Init(id, "")
		if err != nil {
			t.Fatal(err)
		}
		input.Spec.Adapters = append(input.Spec.Adapters, fragment.Spec.Adapters...)
		if infra := fragment.Spec.Infrastructure; infra != nil {
			if infra.Deployment != nil {
				input.Spec.Infrastructure.Deployment = infra.Deployment
			}
			input.Spec.Infrastructure.RuntimeBackends = append(input.Spec.Infrastructure.RuntimeBackends, infra.RuntimeBackends...)
			input.Spec.Infrastructure.RuntimeProfiles = append(input.Spec.Infrastructure.RuntimeProfiles, infra.RuntimeProfiles...)
		}
		if services := fragment.Spec.Services; services != nil {
			input.Spec.Services.ModelBackends = append(input.Spec.Services.ModelBackends, services.ModelBackends...)
			input.Spec.Services.ModelProfiles = append(input.Spec.Services.ModelProfiles, services.ModelProfiles...)
			input.Spec.Services.ToolBackends = append(input.Spec.Services.ToolBackends, services.ToolBackends...)
			input.Spec.Services.ToolProfiles = append(input.Spec.Services.ToolProfiles, services.ToolProfiles...)
		}
	}
	resolved, _, failure := platform.Resolve(input, registry)
	if failure != nil {
		t.Fatal(failure)
	}
	return resolved, registry
}

type doubleRegistry struct {
	*adapterregistry.Registry
	provider toolbackend.Provider
}

func (r doubleRegistry) Construct(id, version string, capability platform.Capability) (any, error) {
	if capability == platform.CapabilityTool {
		return doubleFactory{provider: r.provider}, nil
	}
	return r.Registry.Construct(id, version, capability)
}

type doubleFactory struct{ provider toolbackend.Provider }

func (f doubleFactory) NewToolProvider(_ map[string]any, _ []map[string]any) (toolbackend.Provider, error) {
	return f.provider, nil
}

type installedDouble struct {
	calls    int
	received toolbackend.Invocation
}

func (p *installedDouble) Invoke(_ context.Context, call toolbackend.Invocation) (toolbackend.Result, error) {
	p.calls++
	p.received = call
	return toolbackend.Result{Text: "independent provider double", ResultRef: "artifact:readme"}, nil
}

func TestInstalledToolBuilderUsesResolvedRoutesAndProviderFactory(t *testing.T) {
	resolved, registry := resolvedTools(t)
	provider := &installedDouble{}
	tools, err := buildInstalledTools(resolved, doubleRegistry{Registry: registry, provider: provider})
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 0 {
		t.Fatal("configuration contacted provider")
	}
	authority := &v0.EffectiveAuthority{Tools: []string{"repo.read"}, ResourceScopes: []string{"repo:agenova/e16-fixture"}, ModelProfile: "model", Runtime: v0.EffectiveAuthorityRuntime{ProfileRef: "runtime"}}
	if err := validateInstalledAuthority(authority, map[string]string{"model": "fixture"}, map[string]bool{"runtime": true}, tools); err == nil {
		t.Fatal("unfinished installed execution was admitted")
	} else if submission, ok := err.(*console.SubmissionError); !ok || submission.Code != "tool_transport_unavailable" {
		t.Fatalf("wrong stage failure: %v", err)
	}
	authority.Tools = nil
	if err := validateInstalledAuthority(authority, map[string]string{"model": "fixture"}, map[string]bool{"runtime": true}, tools); err != nil {
		t.Fatal("model-only Work was blocked", err)
	}
	for _, operation := range []string{"git.read", "repo.write"} {
		authority.Tools = []string{operation}
		if validateInstalledAuthority(authority, map[string]string{"model": "fixture"}, map[string]bool{"runtime": true}, tools) == nil {
			t.Fatal("uninstalled grant accepted")
		}
	}
	// The installed catalog is detached from subsequently edited input data.
	resolved.ToolRoutes[0].Tool.AllowedValues[0] = "outside"
	for i := range resolved.Profiles {
		if resolved.Profiles[i].Capability == platform.CapabilityTool {
			resolved.Profiles[i].Config["parameter-allowed-values"] = "outside"
		}
	}
	result, err := tools.Invoke(context.Background(), toolbackend.Invocation{ID: "host-invocation", ClaimID: "claim-a", Operation: "repo.read", ResourceScope: "repo:agenova/e16-fixture", Parameters: map[string]string{"file": "README.md"}})
	if err != nil || provider.calls != 1 || !result.Untrusted || provider.received.ID != "host-invocation" {
		t.Fatalf("result=%+v calls=%d err=%v", result, provider.calls, err)
	}
}
func TestInstalledToolBuilderFailsClosedAndNeverSubstitutesMock(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*platform.ResolvedPlatform)
	}{
		{"tampered route", func(p *platform.ResolvedPlatform) { p.ToolRoutes[0].Tool.Operation = "git.read" }},
		{"missing routes", func(p *platform.ResolvedPlatform) { p.ToolRoutes = nil }},
		{"unknown adapter", func(p *platform.ResolvedPlatform) {
			for i := range p.Adapters {
				if p.Adapters[i].ID == bundled.MCPHTTPToolID {
					p.Adapters[i].Version = "99.0.0"
				}
			}
		}},
		{"invalid config", func(p *platform.ResolvedPlatform) {
			for i := range p.Instances {
				if p.Instances[i].Category == platform.CapabilityTool {
					p.Instances[i].Config["endpoint"] = "http://outside.invalid/mcp"
				}
			}
		}},
		{"wrong profile reference", func(p *platform.ResolvedPlatform) {
			for i := range p.Profiles {
				if p.Profiles[i].Capability == platform.CapabilityTool {
					p.Profiles[i].BackendRef = "missing"
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved, registry := resolvedTools(t)
			tc.change(resolved)
			if _, err := buildInstalledTools(resolved, registry); err == nil {
				t.Fatal("invalid installed config accepted")
			}
		})
	}
	resolved, registry := resolvedTools(t)
	tools, err := buildInstalledTools(resolved, registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tools.Invoke(context.Background(), toolbackend.Invocation{ID: "call", ClaimID: "claim", Operation: "repo.read", ResourceScope: "repo:agenova/e16-fixture", Parameters: map[string]string{"file": "README.md"}}); !errors.Is(err, toolbackend.ErrUnavailable) {
		t.Fatal("configured unfinished transport fell back")
	}
	empty, err := buildInstalledTools(&platform.ResolvedPlatform{}, registry)
	if err != nil || empty != nil {
		t.Fatal("legacy fixture selection changed")
	}
}
