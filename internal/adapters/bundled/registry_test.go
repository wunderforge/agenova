// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"encoding/json"
	"strings"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/platform"
	"gopkg.in/yaml.v3"
)

func TestBundledRegistryConstructsCapabilityOwnedImplementations(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		id         string
		capability platform.Capability
		want       string
	}{
		{KubernetesDeploymentID, platform.CapabilityDeployment, KubernetesDeploymentID + "@" + ReferenceVersion},
		{AgentSandboxRuntimeID, platform.CapabilityRuntime, AgentSandboxRuntimeID + "@" + ReferenceVersion},
		{OpenAICompatibleModelID, platform.CapabilityModel, OpenAICompatibleModelID + "@" + ReferenceVersion},
	}
	for _, test := range tests {
		implementation, err := registry.Construct(test.id, ReferenceVersion, test.capability)
		if err != nil {
			t.Fatalf("Construct(%s): %v", test.id, err)
		}
		if got := DescribeImplementation(implementation); got != test.want {
			t.Fatalf("DescribeImplementation() = %q, want %q", got, test.want)
		}
	}
}

func TestBundledInitFragmentsComposeAndResolveOnePlatform(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	lifecycle, err := adapterregistry.NewLifecycle(registry, adapterregistry.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	deployment := mustInit(t, lifecycle, KubernetesDeploymentID, "kubernetes-deployment")
	runtime := mustInit(t, lifecycle, AgentSandboxRuntimeID, "agent-sandbox-runtime")
	model := mustInit(t, lifecycle, OpenAICompatibleModelID, "openai-compatible-backend")

	input := v1alpha1.Platform{
		APIVersion: v1alpha1.PlatformAPIVersion,
		Kind:       v1alpha1.PlatformKind,
		Metadata:   v1alpha1.ObjectMeta{Name: "reference-local"},
		Spec: v1alpha1.PlatformSpec{
			Adapters: append(append(deployment.Spec.Adapters, runtime.Spec.Adapters...), model.Spec.Adapters...),
			Infrastructure: v1alpha1.PlatformInfrastructure{
				Deployment:      deployment.Spec.Infrastructure.Deployment,
				RuntimeBackends: runtime.Spec.Infrastructure.RuntimeBackends,
				RuntimeProfiles: runtime.Spec.Infrastructure.RuntimeProfiles,
			},
			Services: v1alpha1.PlatformServices{
				ModelBackends: model.Spec.Services.ModelBackends,
				ModelProfiles: model.Spec.Services.ModelProfiles,
			},
			InitialPolicyRef: &v1alpha1.PlatformPolicyReference{ID: "reference-default-deny", Version: "1"},
		},
	}
	data, err := yaml.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	parsed, validationErr := v1alpha1.ParsePlatformYAML(data)
	if validationErr != nil {
		t.Fatalf("ParsePlatformYAML(): %v\n%s", validationErr, data)
	}
	resolved, lock, resolveErr := platform.Resolve(parsed, lifecycle)
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	if len(resolved.Adapters) != 3 || len(resolved.Instances) != 3 || len(resolved.Profiles) != 2 || len(resolved.ModelRoutes) != 1 {
		t.Fatalf("resolved = %#v", resolved)
	}
	if resolved.ModelRoutes[0].Gateway != platform.CoreModelGateway || lock.Revision != resolved.Revision {
		t.Fatalf("route/lock = %#v / %#v", resolved.ModelRoutes, lock)
	}
	encoded, _ := json.Marshal([]adapterregistry.PlatformFragment{deployment, runtime, model})
	for _, forbidden := range []string{"apiKey", "token", "password", "credentialRef"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("fragments contain %q: %s", forbidden, encoded)
		}
	}
}

func TestBundledCanonicalizersFailClosed(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		id         string
		capability platform.Capability
		config     map[string]any
		want       string
	}{
		{KubernetesDeploymentID, platform.CapabilityDeployment, map[string]any{"context": "kind-agenova", "namespace": "agenova-system", "extra": true}, "unknown-field"},
		{AgentSandboxRuntimeID, platform.CapabilityRuntime, map[string]any{"connection": map[string]any{"mode": "host-context", "namespace": "workers"}}, "unsupported-connection-mode"},
		{OpenAICompatibleModelID, platform.CapabilityModel, map[string]any{"endpoint": "https://user:secret@example.invalid/v1"}, "invalid-endpoint"},
		{OpenAICompatibleModelID, platform.CapabilityModel, map[string]any{"endpoint": "https://models.example/v1?api_key=hidden"}, "invalid-endpoint"},
	}
	for _, test := range tests {
		descriptor, ok := registry.Lookup(test.id, ReferenceVersion)
		if !ok {
			t.Fatal("missing descriptor")
		}
		_, err := descriptor.CanonicalizeInstance(test.capability, test.config)
		if err == nil || !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("canonicalize %s error = %v", test.id, err)
		}
	}
}

func mustInit(t *testing.T, lifecycle *adapterregistry.Lifecycle, id, name string) adapterregistry.PlatformFragment {
	t.Helper()
	fragment, err := lifecycle.Init(id+"@"+ReferenceVersion, name)
	if err != nil {
		t.Fatal(err)
	}
	return fragment
}
