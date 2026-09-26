// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"gopkg.in/yaml.v3"
)

func toolPlatform(t *testing.T) (*v0.Platform, *adapterregistry.Registry) {
	t.Helper()
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := adapterregistry.NewLifecycle(registry, adapterregistry.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	d := mustInit(t, lifecycle, KubernetesDeploymentID, "deployment")
	r := mustInit(t, lifecycle, AgentSandboxRuntimeID, "runtime")
	m := mustInit(t, lifecycle, OpenAICompatibleModelID, "model")
	tool := mustInit(t, lifecycle, MCPHTTPToolID, "docs")
	input := &v0.Platform{APIVersion: v0.PlatformAPIVersion, Kind: v0.PlatformKind, Metadata: v0.ObjectMeta{Name: "tools"}, Spec: v0.PlatformSpec{InitialPolicyRef: &v0.PlatformPolicyReference{ID: "reference-default-deny", Version: "1"}, Infrastructure: v0.PlatformInfrastructure{Deployment: d.Spec.Infrastructure.Deployment, RuntimeBackends: r.Spec.Infrastructure.RuntimeBackends, RuntimeProfiles: r.Spec.Infrastructure.RuntimeProfiles}, Services: v0.PlatformServices{ModelBackends: m.Spec.Services.ModelBackends, ModelProfiles: m.Spec.Services.ModelProfiles, ToolBackends: tool.Spec.Services.ToolBackends, ToolProfiles: tool.Spec.Services.ToolProfiles}}}
	for _, fragment := range []adapterregistry.PlatformFragment{d, r, m, tool} {
		input.Spec.Adapters = append(input.Spec.Adapters, fragment.Spec.Adapters...)
	}
	return input, registry
}
func TestToolLifecycleInitInspectAndLockRoundTrip(t *testing.T) {
	input, registry := toolPlatform(t)
	data, err := yaml.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	parsed, validation := v0.ParsePlatformYAML(data)
	if validation != nil {
		t.Fatal(validation)
	}
	resolved, lock, failure := platform.Resolve(parsed, registry)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(resolved.ToolRoutes) != 1 || resolved.ToolRoutes[0].Tool.Operation != "repo.read" {
		t.Fatal("no neutral tool route")
	}
	encoded, _ := json.Marshal(resolved.ToolRoutes)
	if strings.Contains(string(encoded), "read_file") || strings.Contains(string(encoded), "endpoint") {
		t.Fatal("provider config leaked to tool descriptors")
	}
	if err := platform.VerifyResolvedLock(resolved, lock); err != nil {
		t.Fatal(err)
	}
	lockData, _ := json.Marshal(lock)
	var loaded platform.PlatformLock
	if err := json.Unmarshal(lockData, &loaded); err != nil {
		t.Fatal(err)
	}
	if err := platform.VerifyResolvedLock(resolved, &loaded); err != nil {
		t.Fatal(err)
	}
	// Tool adapter installation is persisted and idempotent across a store reload.
	path := t.TempDir()
	store, err := adapterregistry.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := adapterregistry.NewLifecycle(registry, store)
	if err != nil {
		t.Fatal(err)
	}
	first, err := lifecycle.Install(MCPHTTPToolID)
	if err != nil || !first.Changed {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	store, err = adapterregistry.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err = adapterregistry.NewLifecycle(registry, store)
	if err != nil {
		t.Fatal(err)
	}
	second, err := lifecycle.Install(MCPHTTPToolID)
	if err != nil || second.Changed {
		t.Fatalf("repeat=%+v err=%v", second, err)
	}
	inspected, err := lifecycle.Inspect(MCPHTTPToolID)
	if err != nil || !inspected.Installed {
		t.Fatal("inspect lost installed tool")
	}
	for _, schema := range []adapterregistry.ConfigSchema{inspected.Manifest.InstanceSchema, inspected.Manifest.ProfileSchema} {
		for _, field := range schema.Fields {
			if field.Kind != adapterregistry.ValueString || strings.Contains(field.Path, ".") {
				t.Fatalf("non-flat string schema: %+v", field)
			}
		}
	}
	if _, err := lifecycle.Init(MCPHTTPToolID, strings.Repeat("a", 57)); err == nil {
		t.Fatal("overlong generated profile accepted")
	}
	// No server exists/contact is needed by install, inspect, init or construction.
	value, err := registry.Construct(MCPHTTPToolID, ReferenceVersion, platform.CapabilityTool)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := value.(toolbackend.Factory).NewToolProvider(input.Spec.Services.ToolBackends[0].Config, []map[string]any{input.Spec.Services.ToolProfiles[0].Config})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Invoke(context.Background(), toolbackend.Invocation{}); !errors.Is(err, toolbackend.ErrUnavailable) {
		t.Fatal("unfinished transport did not fail explicitly")
	}
}
func TestToolConfigAndRoutesFailClosed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v0.Platform)
	}{
		{"numeric leaf", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["max-response-bytes"] = 65536 }},
		{"nested config", func(p *v0.Platform) {
			p.Spec.Services.ToolProfiles[0].Config["operations"] = map[string]any{"repo.read": "read_file"}
		}},
		{"unknown field", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["typo"] = "yes" }},
		{"fraction", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["max-concurrent-calls"] = "1.5" }},
		{"negative", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["max-concurrent-calls"] = "-1" }},
		{"exponent", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["max-response-bytes"] = "1e3" }},
		{"overflow", func(p *v0.Platform) {
			p.Spec.Services.ToolBackends[0].Config["max-response-bytes"] = "999999999999999999999"
		}},
		{"concurrency", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["max-concurrent-calls"] = "17" }},
		{"timeout", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["timeout"] = "31s" }},
		{"observation", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["max-response-bytes"] = "100" }},
		{"http outside", func(p *v0.Platform) {
			p.Spec.Services.ToolBackends[0].Config["endpoint"] = "http://untrusted.invalid/mcp"
		}},
		{"userinfo", func(p *v0.Platform) {
			p.Spec.Services.ToolBackends[0].Config["endpoint"] = "https://user:private@host.invalid/mcp"
		}},
		{"query", func(p *v0.Platform) {
			p.Spec.Services.ToolBackends[0].Config["endpoint"] = "https://host.invalid/mcp?secret=private"
		}},
		{"unsupported transport", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].Config["transport"] = "stdio" }},
		{"wrong capability", func(p *v0.Platform) { p.Spec.Services.ToolBackends[0].AdapterRef = "model" }},
		{"missing backend", func(p *v0.Platform) { p.Spec.Services.ToolProfiles[0].BackendRef = "absent" }},
		{"duplicate backend", func(p *v0.Platform) {
			p.Spec.Services.ToolBackends = append(p.Spec.Services.ToolBackends, p.Spec.Services.ToolBackends[0])
		}},
		{"duplicate profile", func(p *v0.Platform) {
			p.Spec.Services.ToolProfiles = append(p.Spec.Services.ToolProfiles, p.Spec.Services.ToolProfiles[0])
		}},
		{"duplicate route", func(p *v0.Platform) {
			second := p.Spec.Services.ToolProfiles[0]
			second.Name = "duplicate"
			p.Spec.Services.ToolProfiles = append(p.Spec.Services.ToolProfiles, second)
		}},
		{"traversal", func(p *v0.Platform) {
			p.Spec.Services.ToolProfiles[0].Config["parameter-allowed-values"] = "../private"
		}},
		{"duplicate values", func(p *v0.Platform) {
			p.Spec.Services.ToolProfiles[0].Config["parameter-allowed-values"] = "README.md\nREADME.md"
		}},
		{"empty line", func(p *v0.Platform) {
			p.Spec.Services.ToolProfiles[0].Config["parameter-allowed-values"] = "README.md\n"
		}},
		{"credential parameter", func(p *v0.Platform) { p.Spec.Services.ToolProfiles[0].Config["parameter-name"] = "api_key" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input, registry := toolPlatform(t)
			tc.mutate(input)
			_, _, failure := platform.Resolve(input, registry)
			if failure == nil {
				t.Fatal("invalid config resolved")
			}
			if strings.Contains(failure.Error(), "private@") || strings.Contains(failure.Error(), "secret=private") {
				t.Fatal("raw rejected configuration leaked")
			}
		})
	}
}
func TestToolRevisionsCanonicalizeAndPreserveOldLocks(t *testing.T) {
	input, registry := toolPlatform(t)
	first, lock, err := platform.Resolve(input, registry)
	if err != nil {
		t.Fatal(err)
	}
	input.Spec.Services.ToolBackends[0].Config["timeout"] = "5000ms"
	input.Spec.Services.ToolBackends[0].Config["max-response-bytes"] = "065536"
	input.Spec.Services.ToolProfiles[0].Config["parameter-allowed-values"] = "logs/timeout.log\n README.md "
	for i, j := 0, len(input.Spec.Adapters)-1; i < j; i, j = i+1, j-1 {
		input.Spec.Adapters[i], input.Spec.Adapters[j] = input.Spec.Adapters[j], input.Spec.Adapters[i]
	}
	second, secondLock, err := platform.Resolve(input, registry)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != second.Revision || !reflect.DeepEqual(lock, secondLock) {
		t.Fatal("equivalent tool config changed revision")
	}
	input.Spec.Services.ToolProfiles[0].Config["mcp-tool"] = "read_other"
	changed, _, err := platform.Resolve(input, registry)
	if err != nil || changed.Revision == first.Revision {
		t.Fatal("tool mapping was not revision bound")
	}
	first.ToolRoutes[0].Tool.AllowedValues[0] = "forged"
	if platform.VerifyResolvedLock(first, lock) == nil {
		t.Fatal("tampered tool catalog passed integrity check")
	}
	// Existing documents keep their wire payload (including absent toolRoutes).
	input.Spec.Services.ToolBackends = nil
	input.Spec.Services.ToolProfiles = nil
	for i, a := range input.Spec.Adapters {
		if a.ID == MCPHTTPToolID {
			input.Spec.Adapters = append(input.Spec.Adapters[:i], input.Spec.Adapters[i+1:]...)
			break
		}
	}
	old, oldLock, failure := platform.Resolve(input, registry)
	if failure != nil {
		t.Fatal(failure)
	}
	oldJSON, _ := json.Marshal(old)
	if strings.Contains(string(oldJSON), "toolRoutes") {
		t.Fatal("empty additive field changed legacy payload")
	}
	if err := platform.VerifyResolvedLock(old, oldLock); err != nil {
		t.Fatal(err)
	}
}

func TestToolRoutesAcrossResourcesAndBackends(t *testing.T) {
	input, registry := toolPlatform(t)
	backend := input.Spec.Services.ToolBackends[0]
	backend.Name = "second-backend"
	input.Spec.Services.ToolBackends = append(input.Spec.Services.ToolBackends, backend)
	profile := input.Spec.Services.ToolProfiles[0]
	profile.Name = "second-profile"
	profile.BackendRef = backend.Name
	profile.Config = map[string]any{}
	for k, v := range input.Spec.Services.ToolProfiles[0].Config {
		profile.Config[k] = v
	}
	input.Spec.Services.ToolProfiles = append(input.Spec.Services.ToolProfiles, profile)
	if _, _, err := platform.Resolve(input, registry); err == nil {
		t.Fatal("duplicate logical/resource route across backends accepted")
	}
	profile.Config["resource-scope"] = "repo:example/other"
	first, _, err := platform.Resolve(input, registry)
	if err != nil {
		t.Fatal(err)
	}
	input.Spec.Services.ToolProfiles[0], input.Spec.Services.ToolProfiles[1] = input.Spec.Services.ToolProfiles[1], input.Spec.Services.ToolProfiles[0]
	input.Spec.Services.ToolBackends[0], input.Spec.Services.ToolBackends[1] = input.Spec.Services.ToolBackends[1], input.Spec.Services.ToolBackends[0]
	second, _, err := platform.Resolve(input, registry)
	if err != nil || first.Revision != second.Revision {
		t.Fatal("list order changed tool revision")
	}
	profile.Config["parameter-name"] = "path"
	if _, _, err := platform.Resolve(input, registry); err == nil {
		t.Fatal("same operation advertised incompatible argument shapes")
	}
}
