// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
)

type lookupMap map[string]Descriptor

func (l lookupMap) Lookup(id, version string) (Descriptor, bool) {
	descriptor, ok := l[id+"@"+version]
	return descriptor, ok
}

func TestPlatformContractResolveProducesActionablePlanAndDigestOnlyLock(t *testing.T) {
	input := referencePlatform()
	resolved, lock, err := Resolve(input, referenceLookup())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Revision != "sha256:9e5a26c12cae8d966794f27daafff70d7a770f62935e50879a5892964bdc1090" {
		t.Fatalf("revision = %q", resolved.Revision)
	}
	if lock.Revision != resolved.Revision || !strings.HasPrefix(lock.Revision, "sha256:") || len(lock.Revision) != 71 {
		t.Fatalf("lock revision = %q", lock.Revision)
	}
	if len(resolved.ModelRoutes) != 1 || resolved.ModelRoutes[0].Gateway != CoreModelGateway || resolved.ModelRoutes[0].BackendRef != "local-ollama" {
		t.Fatalf("model routes = %#v", resolved.ModelRoutes)
	}
	if len(resolved.Instances) != 3 || resolved.Instances[2].Config["connection"] == nil {
		t.Fatalf("resolved instances = %#v", resolved.Instances)
	}
	encodedLock, marshalErr := CanonicalJSON(lock)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if strings.Contains(string(encodedLock), "agenova-workers") || strings.Contains(string(encodedLock), "ollama.agenova-models") {
		t.Fatalf("lock leaked actionable config: %s", encodedLock)
	}
	for _, item := range append(append([]LockedConfig{}, lock.Instances...), lock.Profiles...) {
		if !strings.HasPrefix(item.ConfigDigest, "sha256:") || len(item.ConfigDigest) != 71 {
			t.Fatalf("config digest = %q", item.ConfigDigest)
		}
	}
}

func TestVerifyResolvedLockRejectsTargetAndConfigDigestDrift(t *testing.T) {
	resolved, lock, err := Resolve(referencePlatform(), referenceLookup())
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyResolvedLock(resolved, lock); err != nil {
		t.Fatalf("resolved pair rejected: %v", err)
	}
	original := resolved.Instances[0].Config["context"]
	resolved.Instances[0].Config["context"] = "other-kind"
	if err := VerifyResolvedLock(resolved, lock); err == nil || !strings.Contains(err.Error(), "content revision mismatch") {
		t.Fatalf("changed deployment target accepted: %v", err)
	}
	resolved.Instances[0].Config["context"] = original
	lock.Instances[0].ConfigDigest = "sha256:incorrect"
	if err := VerifyResolvedLock(resolved, lock); err == nil || !strings.Contains(err.Error(), "lock content mismatch") {
		t.Fatalf("changed config digest accepted: %v", err)
	}
}

func TestPlatformContractResolutionIsStableAcrossInputOrdering(t *testing.T) {
	first := referencePlatform()
	second := referencePlatform()
	extra := v1alpha1.PlatformAdapterRequirement{Name: "multi-capability", ID: "agenova.io/test/multi", Version: "0.1.0"}
	first.Spec.Adapters = append(first.Spec.Adapters, extra)
	second.Spec.Adapters = append(second.Spec.Adapters, extra)
	second.Spec.Adapters[0], second.Spec.Adapters[2] = second.Spec.Adapters[2], second.Spec.Adapters[0]
	firstLookup := referenceLookup()
	secondLookup := referenceLookup()
	firstLookup["agenova.io/test/multi@0.1.0"] = Descriptor{ID: "agenova.io/test/multi", Version: "0.1.0", Capabilities: []Capability{CapabilityRuntime, CapabilityModel}}
	secondLookup["agenova.io/test/multi@0.1.0"] = Descriptor{ID: "agenova.io/test/multi", Version: "0.1.0", Capabilities: []Capability{CapabilityRuntime, CapabilityModel}}
	descriptor := secondLookup["agenova.io/test/multi@0.1.0"]
	descriptor.Capabilities[0], descriptor.Capabilities[1] = descriptor.Capabilities[1], descriptor.Capabilities[0]
	secondLookup["agenova.io/test/multi@0.1.0"] = descriptor
	firstResolved, firstLock, firstErr := Resolve(first, firstLookup)
	secondResolved, secondLock, secondErr := Resolve(second, secondLookup)
	if firstErr != nil || secondErr != nil {
		t.Fatalf("resolve errors = %v, %v", firstErr, secondErr)
	}
	if firstResolved.Revision != secondResolved.Revision || !reflect.DeepEqual(firstLock, secondLock) {
		t.Fatalf("ordering changed output:\nfirst=%#v\nsecond=%#v", firstLock, secondLock)
	}
}

func TestPlatformContractProfileValidationConsumesCanonicalBackend(t *testing.T) {
	input := referencePlatform()
	input.Spec.Infrastructure.RuntimeBackends[0].Config = map[string]any{"connection": map[string]any{"mode": "host-context", "context": "kind-agenova"}}
	resolved, lock, err := Resolve(input, referenceLookup())
	if resolved != nil || lock != nil || err == nil || err.Category != ErrorInvalidConfig {
		t.Fatalf("Resolve() = %#v, %#v, %#v", resolved, lock, err)
	}
	if err.Detail != "unsupported-connection-mode at connection.mode" {
		t.Fatalf("error detail = %q", err.Detail)
	}
}

func TestPlatformContractFailuresReturnNoPartialPlan(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v1alpha1.Platform, lookupMap)
		want   ErrorCategory
	}{
		{"unknown descriptor", func(_ *v1alpha1.Platform, lookup lookupMap) {
			delete(lookup, "agenova.io/model/openai-compatible@0.1.0")
		}, ErrorUnknownAdapter},
		{"capability mismatch", func(_ *v1alpha1.Platform, lookup lookupMap) {
			descriptor := lookup["agenova.io/runtime/agent-sandbox@0.1.0"]
			descriptor.Capabilities = []Capability{CapabilityModel}
			lookup["agenova.io/runtime/agent-sandbox@0.1.0"] = descriptor
		}, ErrorCapability},
		{"adapter config failure", func(_ *v1alpha1.Platform, lookup lookupMap) {
			descriptor := lookup["agenova.io/model/openai-compatible@0.1.0"]
			descriptor.CanonicalizeInstance = func(Capability, map[string]any) (map[string]any, error) { return nil, errors.New("invalid endpoint") }
			lookup["agenova.io/model/openai-compatible@0.1.0"] = descriptor
		}, ErrorInvalidConfig},
		{"adapter emits credential ref", func(_ *v1alpha1.Platform, lookup lookupMap) {
			descriptor := lookup["agenova.io/model/openai-compatible@0.1.0"]
			descriptor.CanonicalizeInstance = func(_ Capability, _ map[string]any) (map[string]any, error) {
				return map[string]any{"credentialRef": "hidden"}, nil
			}
			lookup["agenova.io/model/openai-compatible@0.1.0"] = descriptor
		}, ErrorInvalidConfig},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := referencePlatform()
			lookup := referenceLookup()
			test.mutate(input, lookup)
			resolved, lock, err := Resolve(input, lookup)
			if resolved != nil || lock != nil || err == nil || err.Category != test.want {
				t.Fatalf("Resolve() = %#v, %#v, %#v; want %s", resolved, lock, err, test.want)
			}
		})
	}
}

func TestPlatformContractRedactsArbitraryAdapterErrors(t *testing.T) {
	input := referencePlatform()
	lookup := referenceLookup()
	descriptor := lookup["agenova.io/model/openai-compatible@0.1.0"]
	descriptor.CanonicalizeInstance = func(Capability, map[string]any) (map[string]any, error) {
		return nil, errors.New("invalid endpoint https://user:super-secret@example.invalid")
	}
	lookup["agenova.io/model/openai-compatible@0.1.0"] = descriptor
	_, _, err := Resolve(input, lookup)
	if err == nil || err.Detail != "adapter rejected configuration" || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("error = %#v", err)
	}
}

func TestPlatformContractSnapshotsTypedJSONContainers(t *testing.T) {
	input := referencePlatform()
	connection := map[string]string{"mode": "in-cluster", "namespace": "agenova-workers"}
	ports := []int{11434, 443}
	input.Spec.Infrastructure.RuntimeBackends[0].Config["connection"] = connection
	input.Spec.Services.ModelBackends[0].Config["ports"] = ports
	resolved, lock, err := Resolve(input, referenceLookup())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	connection["namespace"] = "mutated"
	ports[0] = 1
	runtimeConfig := resolved.Instances[2].Config["connection"].(map[string]any)
	modelPorts := resolved.Instances[1].Config["ports"].([]any)
	if runtimeConfig["namespace"] != "agenova-workers" || modelPorts[0] != 11434 {
		t.Fatalf("resolved snapshot mutated: runtime=%#v ports=%#v", runtimeConfig, modelPorts)
	}
	if lock.Revision != resolved.Revision {
		t.Fatalf("lock revision %q differs from resolved %q", lock.Revision, resolved.Revision)
	}
}

func referenceLookup() lookupMap {
	identityInstance := func(_ Capability, config map[string]any) (map[string]any, error) { return config, nil }
	identityProfile := func(_ Capability, _ map[string]any, config map[string]any) (map[string]any, error) {
		return config, nil
	}
	runtimeProfile := func(_ Capability, backend, profile map[string]any) (map[string]any, error) {
		connection, _ := backend["connection"].(map[string]any)
		if connection["mode"] != "in-cluster" {
			return nil, NewAdapterConfigError("unsupported-connection-mode", "connection.mode")
		}
		isolation, _ := profile["isolation"].(string)
		if isolation != "dedicated" {
			return nil, NewAdapterConfigError("unsupported-isolation", "isolation")
		}
		return map[string]any{"isolation": isolation}, nil
	}
	return lookupMap{
		"agenova.io/deployment/kubernetes@0.1.0":   {ID: "agenova.io/deployment/kubernetes", Version: "0.1.0", Capabilities: []Capability{CapabilityDeployment}, CanonicalizeInstance: identityInstance, CanonicalizeProfile: identityProfile},
		"agenova.io/runtime/agent-sandbox@0.1.0":   {ID: "agenova.io/runtime/agent-sandbox", Version: "0.1.0", Capabilities: []Capability{CapabilityRuntime}, CanonicalizeInstance: identityInstance, CanonicalizeProfile: runtimeProfile},
		"agenova.io/model/openai-compatible@0.1.0": {ID: "agenova.io/model/openai-compatible", Version: "0.1.0", Capabilities: []Capability{CapabilityModel}, CanonicalizeInstance: identityInstance, CanonicalizeProfile: identityProfile},
	}
}

func referencePlatform() *v1alpha1.Platform {
	return &v1alpha1.Platform{
		APIVersion: v1alpha1.PlatformAPIVersion,
		Kind:       v1alpha1.PlatformKind,
		Metadata:   v1alpha1.ObjectMeta{Name: "reference-local"},
		Spec: v1alpha1.PlatformSpec{
			Adapters: []v1alpha1.PlatformAdapterRequirement{
				{Name: "kubernetes-deployment", ID: "agenova.io/deployment/kubernetes", Version: "0.1.0"},
				{Name: "agent-sandbox-runtime", ID: "agenova.io/runtime/agent-sandbox", Version: "0.1.0"},
				{Name: "openai-compatible-backend", ID: "agenova.io/model/openai-compatible", Version: "0.1.0"},
			},
			Infrastructure: v1alpha1.PlatformInfrastructure{
				Deployment: &v1alpha1.PlatformInstance{Name: "control-plane", AdapterRef: "kubernetes-deployment", Config: map[string]any{"context": "kind-agenova", "namespace": "agenova-system"}},
				RuntimeBackends: []v1alpha1.PlatformInstance{{Name: "primary-runtime", AdapterRef: "agent-sandbox-runtime", Config: map[string]any{
					"connection": map[string]any{"mode": "in-cluster", "namespace": "agenova-workers"},
				}}},
				RuntimeProfiles: []v1alpha1.PlatformProfile{{Name: "standard-isolated", BackendRef: "primary-runtime", Config: map[string]any{"isolation": "dedicated"}}},
			},
			Services: v1alpha1.PlatformServices{
				ModelBackends: []v1alpha1.PlatformInstance{{Name: "local-ollama", AdapterRef: "openai-compatible-backend", Config: map[string]any{"endpoint": "https://ollama.agenova-models.svc.cluster.local/v1"}}},
				ModelProfiles: []v1alpha1.PlatformProfile{{Name: "approved-coding-model", BackendRef: "local-ollama", Config: map[string]any{"model": "llama3.1:latest"}}},
			},
			InitialPolicyRef: &v1alpha1.PlatformPolicyReference{ID: "reference-default-deny", Version: "1"},
		},
	}
}
